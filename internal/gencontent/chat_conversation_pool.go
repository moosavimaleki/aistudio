package gencontent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/redis/go-redis/v9"
)

const chatRootPrompt = "Initialize this lab conversation. Reply only READY."

type chatConversationRouter interface {
	Route(context.Context, string, string, []chatgptdirect.Message, directChatCompleter) (chatConversationRoute, error)
}

type chatConversationRoute struct {
	Input  chatgptdirect.Input
	finish func(context.Context, chatgptdirect.Result) error
	abort  func(context.Context) error
}

func (r chatConversationRoute) Finish(ctx context.Context, result chatgptdirect.Result) error {
	return r.finish(ctx, result)
}

func (r chatConversationRoute) Abort(ctx context.Context) error {
	return r.abort(ctx)
}

type chatConversationPool struct {
	redis *redis.Client
	max   int
	wait  time.Duration
	lease time.Duration
}

type chatConversationRoot struct {
	ConversationID  string `json:"conversation_id"`
	ParentMessageID string `json:"parent_message_id"`
	BrowserID       string `json:"browser_id"`
}

type chatConversationHistory struct {
	RootID          string `json:"root_id"`
	ConversationID  string `json:"conversation_id"`
	ParentMessageID string `json:"parent_message_id"`
	BrowserID       string `json:"browser_id"`
}

func newChatConversationPool(url string) (*chatConversationPool, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &chatConversationPool{
		redis: redis.NewClient(options),
		max:   positiveEnv("CHATGPT_CONVERSATION_POOL_MAX", 3),
		wait:  time.Duration(positiveEnv("CHATGPT_CONVERSATION_POOL_WAIT_SECONDS", 5)) * time.Second,
		lease: time.Duration(positiveEnv("CHATGPT_CONVERSATION_LEASE_SECONDS", 1800)) * time.Second,
	}, nil
}

func positiveEnv(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func (p *chatConversationPool) Route(
	ctx context.Context,
	model, browserID string,
	messages []chatgptdirect.Message,
	direct directChatCompleter,
) (chatConversationRoute, error) {
	if len(messages) == 0 {
		return chatConversationRoute{}, fmt.Errorf("messages is required")
	}
	lastUser := lastUserIndex(messages)
	if lastUser < 0 {
		return chatConversationRoute{}, fmt.Errorf("a non-empty user message is required")
	}
	key := p.poolKey(model, browserID)
	history, found, err := p.history(ctx, key, messages[:lastUser])
	if err != nil {
		return chatConversationRoute{}, err
	}
	root, rootID, token, err := p.acquire(ctx, key, history, found, direct, model, browserID)
	if err != nil {
		return chatConversationRoute{}, err
	}
	input := chatgptdirect.Input{Model: model, BrowserID: root.BrowserID, ConversationID: root.ConversationID, ParentMessageID: root.ParentMessageID}
	if found && history.RootID == rootID {
		input.ConversationID = history.ConversationID
		input.ParentMessageID = history.ParentMessageID
		input.Messages = []chatgptdirect.Message{messages[lastUser]}
	} else {
		input.Messages = messages
		input.IncludeHistory = true
	}
	return chatConversationRoute{
		Input: input,
		finish: func(finishCtx context.Context, result chatgptdirect.Result) error {
			defer p.release(finishCtx, key, rootID, token)
			if result.ConversationID == "" || result.ParentMessageID == "" {
				return fmt.Errorf("ChatGPT response did not include conversation metadata")
			}
			completed := append(append([]chatgptdirect.Message{}, messages...), chatgptdirect.Message{Role: "assistant", Content: result.Text})
			value := chatConversationHistory{RootID: rootID, ConversationID: result.ConversationID, ParentMessageID: result.ParentMessageID, BrowserID: result.BrowserID}
			encoded, _ := json.Marshal(value)
			return p.redis.Set(finishCtx, p.historyKey(key, completed), encoded, 0).Err()
		},
		abort: func(abortCtx context.Context) error { return p.release(abortCtx, key, rootID, token) },
	}, nil
}

func (p *chatConversationPool) acquire(
	ctx context.Context,
	key string,
	history chatConversationHistory,
	hasHistory bool,
	direct directChatCompleter,
	model, browserID string,
) (chatConversationRoot, string, string, error) {
	deadline := time.Now().Add(p.wait)
	for {
		roots, err := p.roots(ctx, key)
		if err != nil {
			return chatConversationRoot{}, "", "", err
		}
		ids := orderedRootIDs(roots, history.RootID, hasHistory)
		for _, id := range ids {
			token := fmt.Sprintf("lease-%d", time.Now().UnixNano())
			locked, lockErr := p.redis.SetNX(ctx, p.lockKey(key, id), token, p.lease).Result()
			if lockErr != nil {
				return chatConversationRoot{}, "", "", lockErr
			}
			if locked {
				return roots[id], id, token, nil
			}
		}
		if len(roots) < p.max {
			root, id, token, created, createErr := p.createRoot(ctx, key, direct, model, browserID)
			if createErr != nil {
				return chatConversationRoot{}, "", "", createErr
			}
			if created {
				return root, id, token, nil
			}
		}
		if time.Now().After(deadline) {
			return chatConversationRoot{}, "", "", fmt.Errorf("ChatGPT conversation pool is busy after waiting %s", p.wait)
		}
		select {
		case <-ctx.Done():
			return chatConversationRoot{}, "", "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (p *chatConversationPool) createRoot(ctx context.Context, key string, direct directChatCompleter, model, browserID string) (chatConversationRoot, string, string, bool, error) {
	bootstrap := p.bootstrapKey(key)
	token := fmt.Sprintf("bootstrap-%d", time.Now().UnixNano())
	locked, err := p.redis.SetNX(ctx, bootstrap, token, p.lease).Result()
	if err != nil || !locked {
		return chatConversationRoot{}, "", "", false, err
	}
	defer p.release(ctx, bootstrap, "", token)
	count, err := p.redis.HLen(ctx, p.rootsKey(key)).Result()
	if err != nil || count >= int64(p.max) {
		return chatConversationRoot{}, "", "", false, err
	}
	result, err := direct.Generate(ctx, chatgptdirect.Input{
		Model: model, BrowserID: browserID,
		Messages: []chatgptdirect.Message{{Role: "user", Content: chatRootPrompt}},
	})
	if err != nil {
		return chatConversationRoot{}, "", "", false, err
	}
	if result.ConversationID == "" || result.ParentMessageID == "" {
		return chatConversationRoot{}, "", "", false, fmt.Errorf("ChatGPT root conversation did not include metadata")
	}
	root := chatConversationRoot{ConversationID: result.ConversationID, ParentMessageID: result.ParentMessageID, BrowserID: result.BrowserID}
	encoded, _ := json.Marshal(root)
	if err := p.redis.HSet(ctx, p.rootsKey(key), result.ConversationID, encoded).Err(); err != nil {
		return chatConversationRoot{}, "", "", false, err
	}
	leaseToken := fmt.Sprintf("lease-%d", time.Now().UnixNano())
	locked, err = p.redis.SetNX(ctx, p.lockKey(key, result.ConversationID), leaseToken, p.lease).Result()
	if err != nil || !locked {
		return chatConversationRoot{}, "", "", false, err
	}
	return root, result.ConversationID, leaseToken, true, nil
}

func (p *chatConversationPool) history(ctx context.Context, key string, messages []chatgptdirect.Message) (chatConversationHistory, bool, error) {
	value, err := p.redis.Get(ctx, p.historyKey(key, messages)).Result()
	if err == redis.Nil {
		return chatConversationHistory{}, false, nil
	}
	if err != nil {
		return chatConversationHistory{}, false, err
	}
	var result chatConversationHistory
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return chatConversationHistory{}, false, err
	}
	return result, true, nil
}

func (p *chatConversationPool) roots(ctx context.Context, key string) (map[string]chatConversationRoot, error) {
	values, err := p.redis.HGetAll(ctx, p.rootsKey(key)).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]chatConversationRoot, len(values))
	for id, value := range values {
		var root chatConversationRoot
		if json.Unmarshal([]byte(value), &root) == nil && root.ConversationID != "" && root.ParentMessageID != "" {
			result[id] = root
		}
	}
	return result, nil
}

func orderedRootIDs(roots map[string]chatConversationRoot, preferred string, hasPreferred bool) []string {
	ids := make([]string, 0, len(roots))
	if hasPreferred {
		if _, ok := roots[preferred]; ok {
			ids = append(ids, preferred)
		}
	}
	firstOther := len(ids)
	for id := range roots {
		if id != preferred {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids[firstOther:])
	return ids
}

func lastUserIndex(messages []chatgptdirect.Message) int {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" && strings.TrimSpace(messages[index].Content) != "" {
			return index
		}
	}
	return -1
}

func (p *chatConversationPool) release(ctx context.Context, key, id, token string) error {
	lock := key
	if id != "" {
		lock = p.lockKey(key, id)
	}
	script := redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`)
	return script.Run(ctx, p.redis, []string{lock}, token).Err()
}

func (p *chatConversationPool) poolKey(model, browserID string) string {
	return hashKey(model + "\x00" + browserID)
}

func (p *chatConversationPool) rootsKey(key string) string {
	return "gencontent:chatgpt:" + key + ":roots"
}
func (p *chatConversationPool) bootstrapKey(key string) string {
	return "gencontent:chatgpt:" + key + ":bootstrap"
}
func (p *chatConversationPool) lockKey(key, id string) string {
	return "gencontent:chatgpt:" + key + ":lock:" + id
}
func (p *chatConversationPool) historyKey(key string, messages []chatgptdirect.Message) string {
	encoded, _ := json.Marshal(messages)
	return "gencontent:chatgpt:" + key + ":history:" + hashKey(string(encoded))
}

func hashKey(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
