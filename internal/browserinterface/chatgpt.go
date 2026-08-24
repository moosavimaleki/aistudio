package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const chatGPTDispatchTimeout = 10 * time.Second

type ChatService struct {
	broker     *Broker
	fleet      *Fleet
	prepare    func(*ChromeSession) error
	pressEnter func(*ChromeSession, string) error
	mu         sync.Mutex
	locks      map[string]*sync.Mutex
}

type DirectChatRequest struct {
	BrowserID       string
	Prompt          string
	Model           string
	ConversationID  string
	ParentMessageID string
	ThinkingEffort  string
}

type chatJobRequest struct {
	BrowserID       string
	Prompt          string
	Kind            string
	Model           string
	ConversationID  string
	ParentMessageID string
	ThinkingEffort  string
}

func NewChatService(broker *Broker, fleet *Fleet) *ChatService {
	return &ChatService{
		broker: broker,
		fleet:  fleet,
		prepare: func(session *ChromeSession) error {
			return session.PrepareChatGPT()
		},
		pressEnter: func(session *ChromeSession, nonce string) error {
			return session.PressChatGPTEnter(nonce)
		},
		locks: map[string]*sync.Mutex{},
	}
}

func (s *ChatService) Generate(ctx context.Context, browserID, prompt string) (map[string]any, error) {
	result, _, err := s.run(ctx, chatJobRequest{
		BrowserID: browserID,
		Prompt:    prompt,
		Kind:      "chatgpt.generate",
	})
	return result, err
}

func (s *ChatService) GenerateImage(ctx context.Context, browserID, prompt string) (map[string]any, error) {
	result, _, err := s.run(ctx, chatJobRequest{
		BrowserID: browserID,
		Prompt:    prompt,
		Kind:      "chatgpt.generate_image",
	})
	return result, err
}

func (s *ChatService) PrepareDirect(ctx context.Context, input DirectChatRequest) (map[string]any, error) {
	result, browserID, err := s.run(ctx, chatJobRequest{
		BrowserID:       input.BrowserID,
		Prompt:          input.Prompt,
		Kind:            "chatgpt.prepare_direct",
		Model:           input.Model,
		ConversationID:  input.ConversationID,
		ParentMessageID: input.ParentMessageID,
		ThinkingEffort:  input.ThinkingEffort,
	})
	if err != nil {
		return nil, err
	}
	session, err := s.fleet.Session(browserID)
	if err != nil {
		return nil, err
	}
	transport, err := session.ChatGPTTransport()
	if err != nil {
		return nil, err
	}
	for name, value := range transport {
		result[name] = value
	}
	return result, nil
}

func (s *ChatService) run(ctx context.Context, input chatJobRequest) (map[string]any, string, error) {
	if input.Prompt == "" {
		return nil, "", fmt.Errorf("ChatGPT prompt is required")
	}
	resolved, err := s.fleet.ResolveChatGPT(input.BrowserID)
	if err != nil {
		return nil, "", err
	}
	lock := s.browserLock(resolved)
	lock.Lock()
	defer lock.Unlock()
	session, err := s.fleet.Session(resolved)
	if err != nil {
		return nil, "", err
	}
	if err := s.ensureReady(ctx, resolved, session); err != nil {
		return nil, "", err
	}

	jobContext, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	payload := map[string]any{
		"kind":            input.Kind,
		"prompt":          input.Prompt,
		"submitNonce":     jobID,
		"model":           input.Model,
		"conversationId":  input.ConversationID,
		"parentMessageId": input.ParentMessageID,
		"thinkingEffort":  input.ThinkingEffort,
	}
	type brokerResult struct {
		value map[string]any
		err   error
	}
	resultChannel := make(chan brokerResult, 1)
	dispatched := make(chan struct{})
	go func() {
		result, requestErr := s.broker.requestContextWithDispatch(jobContext, jobID, payload, resolved, dispatched)
		resultChannel <- brokerResult{value: result, err: requestErr}
	}()
	dispatchTimer := time.NewTimer(chatGPTDispatchTimeout)
	defer dispatchTimer.Stop()
	select {
	case <-dispatched:
	case <-dispatchTimer.C:
		cancel()
		<-resultChannel
		session.MarkProviderUnready()
		return nil, "", fmt.Errorf("ChatGPT browser %s extension is not connected", resolved)
	case <-jobContext.Done():
		requestResult := <-resultChannel
		return nil, "", requestResult.err
	}
	if err := s.pressEnter(session, jobID); err != nil {
		cancel()
		if ctx.Err() == nil {
			session.MarkProviderUnready()
		}
		return nil, "", fmt.Errorf("submit ChatGPT prompt: %w", err)
	}
	requestResult := <-resultChannel
	if requestResult.err != nil {
		if ctx.Err() == nil && shouldRecoverChatGPT(requestResult.err) {
			session.MarkProviderUnready()
		}
		return nil, "", requestResult.err
	}
	result := requestResult.value
	result["browserId"] = resolved
	return result, resolved, nil
}

func (s *ChatService) ensureReady(ctx context.Context, browserID string, session *ChromeSession) error {
	if session.Ready() && s.broker.connected(browserID) {
		return nil
	}
	if err := s.prepare(session); err != nil {
		return fmt.Errorf("ChatGPT browser %s is not ready after recovery: %w", browserID, err)
	}
	deadline := time.NewTimer(chatGPTDispatchTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if session.Ready() && s.broker.connected(browserID) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("ChatGPT browser %s recovery: %w", browserID, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("ChatGPT browser %s extension is not connected after recovery", browserID)
		case <-ticker.C:
		}
	}
}

func shouldRecoverChatGPT(err error) bool {
	message := strings.ToLower(err.Error())
	return !strings.Contains(message, "http 429") && !strings.Contains(message, "too many requests")
}

func (s *ChatService) browserLock(browserID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[browserID] == nil {
		s.locks[browserID] = &sync.Mutex{}
	}
	return s.locks[browserID]
}
