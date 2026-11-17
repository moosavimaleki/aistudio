package gencontent

import (
	"context"
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/redis/go-redis/v9"
)

// Intercept only the cleanup command; this test needs no Redis server.
type cleanupHook struct{ called bool }

func (h *cleanupHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *cleanupHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *cleanupHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		h.called = true
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := ctx.Deadline(); !ok {
			return context.DeadlineExceeded
		}
		cmd.(*redis.Cmd).SetVal(int64(1))
		return nil
	}
}

func TestConversationLeaseCleanupSurvivesRequestCancellation(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "unused"})
	t.Cleanup(func() { _ = client.Close() })
	hook := &cleanupHook{}
	client.AddHook(hook)
	pool := &chatConversationPool{redis: client}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pool.release(ctx, "bootstrap-key", "", "owned-token"); err != nil {
		t.Fatalf("canceled request left its bootstrap lease behind: %v", err)
	}
	if !hook.called {
		t.Fatal("lease cleanup never reached Redis")
	}
}

// forkHook short-circuits the Redis commands a fresh-topic Route issues: the
// history lookup misses, the root hash is empty, and every bootstrap/lease lock
// is acquired, so Route creates exactly one root and routes the topic onto it.
type forkHook struct{}

func (forkHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (forkHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (forkHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		switch command := cmd.(type) {
		case *redis.StringCmd:
			// history lookup miss: Process() keeps the hook's returned error, so
			// return redis.Nil rather than set it on the cmd.
			return redis.Nil
		case *redis.MapStringStringCmd:
			command.SetVal(map[string]string{}) // no pooled roots yet
		case *redis.BoolCmd:
			command.SetVal(true) // bootstrap + lease locks acquired
		case *redis.IntCmd:
			command.SetVal(1) // roots length + HSet
		}
		return nil
	}
}

type fakeChatCompleter struct{ result chatgptdirect.Result }

func (f fakeChatCompleter) Generate(_ context.Context, _ chatgptdirect.Input) (chatgptdirect.Result, error) {
	return f.result, nil
}

func TestConversationPoolForksNewTopicUnderReusedConversation(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "unused"})
	t.Cleanup(func() { _ = client.Close() })
	client.AddHook(forkHook{})
	pool := &chatConversationPool{redis: client, max: 3}
	completer := fakeChatCompleter{result: chatgptdirect.Result{
		ConversationID: "conv-fork", ParentMessageID: "parent-fork", BrowserID: "browser-x",
	}}
	route, err := pool.Route(context.Background(), "chatgpt/gpt-5.6-thinking-extended", "browser-x",
		[]chatgptdirect.Message{{Role: "user", Content: "brand new topic"}}, completer)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.Input.ConversationID != "conv-fork" || route.Input.ParentMessageID != "parent-fork" {
		t.Fatalf("fork must reuse the pooled conversation id: %#v", route.Input)
	}
	if route.Input.IncludeHistory {
		t.Fatal("new topic must fork as an isolated branch (IncludeHistory=false), not append")
	}
}
