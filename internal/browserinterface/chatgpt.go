package browserinterface

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ChatService struct {
	broker     *Broker
	fleet      *Fleet
	pressEnter func(*ChromeSession, string) error
	mu         sync.Mutex
	locks      map[string]*sync.Mutex
}

func NewChatService(broker *Broker, fleet *Fleet) *ChatService {
	return &ChatService{
		broker: broker,
		fleet:  fleet,
		pressEnter: func(session *ChromeSession, nonce string) error {
			return session.PressChatGPTEnter(nonce)
		},
		locks: map[string]*sync.Mutex{},
	}
}

func (s *ChatService) Generate(ctx context.Context, browserID, prompt string) (map[string]any, error) {
	if prompt == "" {
		return nil, fmt.Errorf("ChatGPT prompt is required")
	}
	resolved, err := s.fleet.ResolveChatGPT(browserID)
	if err != nil {
		return nil, err
	}
	lock := s.browserLock(resolved)
	lock.Lock()
	defer lock.Unlock()
	session, err := s.fleet.Session(resolved)
	if err != nil {
		return nil, err
	}
	if err := session.RefreshChatGPTCookies(); err != nil {
		return nil, err
	}

	jobContext, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	type brokerResult struct {
		value map[string]any
		err   error
	}
	resultChannel := make(chan brokerResult, 1)
	go func() {
		result, requestErr := s.broker.RequestContextWithID(jobContext, jobID, map[string]any{
			"kind":        "chatgpt.generate",
			"prompt":      prompt,
			"submitNonce": jobID,
		}, resolved)
		resultChannel <- brokerResult{value: result, err: requestErr}
	}()
	if err := s.pressEnter(session, jobID); err != nil {
		cancel()
		return nil, fmt.Errorf("submit ChatGPT prompt: %w", err)
	}
	requestResult := <-resultChannel
	if requestResult.err != nil {
		return nil, requestResult.err
	}
	result := requestResult.value
	if err := session.PersistChatGPTCookies(); err != nil {
		return nil, fmt.Errorf("persist ChatGPT cookies: %w", err)
	}
	result["browserId"] = resolved
	return result, nil
}

func (s *ChatService) browserLock(browserID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[browserID] == nil {
		s.locks[browserID] = &sync.Mutex{}
	}
	return s.locks[browserID]
}
