package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptweb"
	"github.com/hamed/aistudio-api/internal/metrics"
)

const chatGPTDispatchTimeout = 10 * time.Second

type ChatService struct {
	broker     *Broker
	fleet      *Fleet
	prepare    func(*ChromeSession) error
	ready      func(*ChromeSession) bool
	pressEnter func(*ChromeSession, string, <-chan struct{}) error
	mu         sync.Mutex
	locks      map[string]*sync.Mutex
}

type DirectChatRequest struct {
	BrowserID string
}

type chatJobRequest struct {
	BrowserID   string
	Prompt      string
	Kind        string
	Attachments []chatgptweb.Attachment
}

func NewChatService(broker *Broker, fleet *Fleet) *ChatService {
	return &ChatService{
		broker: broker,
		fleet:  fleet,
		prepare: func(session *ChromeSession) error {
			return session.PrepareChatGPT()
		},
		ready: func(session *ChromeSession) bool {
			return session.refreshChatGPTReady()
		},
		pressEnter: func(session *ChromeSession, nonce string, done <-chan struct{}) error {
			return session.PressChatGPTEnter(nonce, done)
		},
		locks: map[string]*sync.Mutex{},
	}
}

func (s *ChatService) Generate(ctx context.Context, browserID, prompt string) (map[string]any, error) {
	gateReady := metrics.Phase(ctx, "rotation_queue")
	release, err := s.fleet.acquireChatGPT(ctx)
	gateReady()
	if err != nil {
		return nil, err
	}
	defer release()
	result, _, err := s.run(ctx, chatJobRequest{
		BrowserID: browserID,
		Prompt:    prompt,
		Kind:      "chatgpt.generate",
	})
	return result, err
}

func (s *ChatService) GenerateImage(ctx context.Context, browserID, prompt string, attachments []chatgptweb.Attachment) (map[string]any, error) {
	gateReady := metrics.Phase(ctx, "rotation_queue")
	release, err := s.fleet.acquireChatGPT(ctx)
	gateReady()
	if err != nil {
		return nil, err
	}
	defer release()
	result, _, err := s.run(ctx, chatJobRequest{
		BrowserID:   browserID,
		Prompt:      prompt,
		Kind:        "chatgpt.generate_image",
		Attachments: attachments,
	})
	return result, err
}

func (s *ChatService) PrepareDirect(ctx context.Context, input DirectChatRequest) (map[string]any, error) {
	gateReady := metrics.Phase(ctx, "rotation_queue")
	release, err := s.fleet.acquireChatGPT(ctx)
	gateReady()
	if err != nil {
		return nil, err
	}
	defer release()
	result, browserID, err := s.run(ctx, chatJobRequest{
		BrowserID: input.BrowserID,
		Kind:      "chatgpt.prepare_direct",
	})
	if err != nil {
		return nil, err
	}
	session, err := s.fleet.Session(browserID)
	if err != nil {
		return nil, err
	}
	transportReady := metrics.Phase(ctx, "cookie_transport")
	transport, err := session.ChatGPTTransport()
	transportReady()
	if err != nil {
		return nil, err
	}
	for name, value := range transport {
		result[name] = value
	}
	return result, nil
}

func (s *ChatService) run(ctx context.Context, input chatJobRequest) (map[string]any, string, error) {
	if input.Kind != "chatgpt.prepare_direct" && input.Prompt == "" {
		return nil, "", fmt.Errorf("ChatGPT prompt is required")
	}
	resolved, err := s.fleet.ResolveChatGPT(input.BrowserID)
	if err != nil {
		return nil, "", err
	}
	lock := s.browserLock(resolved)
	lockReady := metrics.Phase(ctx, "profile_queue")
	lock.Lock()
	lockReady()
	defer lock.Unlock()
	session, err := s.fleet.Session(resolved)
	if err != nil {
		return nil, "", err
	}
	// A request pinned to a known blocked account must not evict a healthy
	// browser before ensureReady rejects that account.
	if reason := s.fleet.warmError(resolved); !session.Ready() &&
		(isChatGPTChallengeError(reason) || isChatGPTSessionError(reason)) {
		return nil, "", fmt.Errorf("ChatGPT browser %s is not ready: %s", resolved, reason)
	}
	s.fleet.parkOtherChatGPT(resolved)
	ready := metrics.Phase(ctx, "page_ready")
	err = s.ensureReady(ctx, resolved, session)
	ready()
	if err != nil {
		if input.BrowserID == "" {
			s.fleet.mu.Lock()
			s.fleet.chatGPTUses = s.fleet.config.ChatGPTRotateEvery
			s.fleet.mu.Unlock()
		}
		return nil, "", err
	}

	jobContext, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	payload := map[string]any{
		"kind":        input.Kind,
		"prompt":      input.Prompt,
		"submitNonce": jobID,
	}
	if len(input.Attachments) > 0 {
		payload["attachments"] = input.Attachments
	}
	type brokerResult struct {
		value map[string]any
		err   error
	}
	resultChannel := make(chan brokerResult, 1)
	resultDone := make(chan struct{})
	dispatched := make(chan struct{})
	go func() {
		result, requestErr := s.broker.requestContextWithDispatch(jobContext, jobID, payload, resolved, dispatched)
		resultChannel <- brokerResult{value: result, err: requestErr}
		close(resultDone)
	}()
	dispatchTimer := time.NewTimer(chatGPTDispatchTimeout)
	defer dispatchTimer.Stop()
	dispatchedDone := metrics.Phase(ctx, "extension_dispatch")
	select {
	case <-dispatched:
		dispatchedDone()
	case <-dispatchTimer.C:
		dispatchedDone()
		cancel()
		<-resultChannel
		session.MarkProviderUnready()
		return nil, "", fmt.Errorf("ChatGPT browser %s extension is not connected", resolved)
	case <-jobContext.Done():
		dispatchedDone()
		requestResult := <-resultChannel
		return nil, "", requestResult.err
	}
	submitted := metrics.Phase(ctx, "composer_submit")
	err = s.pressEnter(session, jobID, resultDone)
	submitted()
	if err != nil {
		cancel()
		if ctx.Err() == nil {
			session.MarkProviderUnready()
		}
		return nil, "", fmt.Errorf("submit ChatGPT prompt: %w", err)
	}
	extensionDone := metrics.Phase(ctx, "extension_result")
	requestResult := <-resultChannel
	extensionDone()
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
	if s.broker.connected(browserID) && s.ready(session) {
		return nil
	}
	if reason := s.fleet.warmError(browserID); isChatGPTChallengeError(reason) || isChatGPTSessionError(reason) {
		return fmt.Errorf("ChatGPT browser %s is not ready: %s", browserID, reason)
	}
	session.MarkProviderUnready()
	if err := s.prepare(session); err != nil {
		s.fleet.setWarmError(browserID, err)
		return fmt.Errorf("ChatGPT browser %s is not ready after recovery: %w", browserID, err)
	}
	s.fleet.setWarmError(browserID, nil)
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
