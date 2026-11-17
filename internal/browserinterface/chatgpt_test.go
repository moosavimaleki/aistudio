package browserinterface

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChatServiceDoesNotSubmitBehindLoginDialog(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{config: Config{ChatGPTDefaultID: "chatgpt"}, sessions: map[string]*ChromeSession{
		"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
	}}
	service := NewChatService(broker, fleet)
	service.ready = func(*ChromeSession) bool { return false }
	service.prepare = func(*ChromeSession) error { return errors.New("ChatGPT session requires sign-in in the browser") }

	_, err := service.GenerateImage(context.Background(), "chatgpt", "draw a circle", nil)
	if err == nil || !isChatGPTSessionError(err.Error()) {
		t.Fatalf("expected sign-in error, got %v", err)
	}
	if job := broker.Next("chatgpt"); job != nil {
		t.Fatalf("unexpected browser job: %#v", job)
	}
	if !isChatGPTSessionError(fleet.warmError("chatgpt")) {
		t.Fatal("failed on-demand startup must retain its reason for the monitor")
	}
	service.prepare = func(*ChromeSession) error {
		t.Fatal("a known sign-in failure must not reload the account repeatedly")
		return nil
	}
	_, err = service.GenerateImage(context.Background(), "chatgpt", "draw a circle", nil)
	if err == nil || !isChatGPTSessionError(err.Error()) {
		t.Fatalf("expected retained sign-in error, got %v", err)
	}
}

func TestChatServiceReturnsKnownChallengeWithoutRetrying(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{
		config: Config{ChatGPTDefaultID: "chatgpt"},
		sessions: map[string]*ChromeSession{
			"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}},
		},
		warmErrors: map[string]string{"chatgpt": "ChatGPT browser challenge is waiting for native completion: Just a moment..."},
	}
	service := NewChatService(broker, fleet)
	service.ready = func(*ChromeSession) bool { return false }
	service.prepare = func(*ChromeSession) error {
		t.Fatal("known browser challenge must not trigger another preparation")
		return nil
	}
	_, err := service.GenerateImage(context.Background(), "chatgpt", "draw a circle", nil)
	if err == nil || !isChatGPTChallengeError(err.Error()) {
		t.Fatalf("expected browser challenge, got %v", err)
	}
}

func TestBlockedAccountDoesNotParkOtherBrowsers(t *testing.T) {
	fleet := &Fleet{
		config: Config{ChatGPTRotateEvery: 10},
		sessions: map[string]*ChromeSession{
			"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}},
		},
		warmErrors: map[string]string{"chatgpt": "ChatGPT browser challenge is waiting for native completion"},
		memoryAvailable: func() uint64 {
			t.Fatal("a blocked account attempted to park other browsers")
			return 0
		},
	}
	service := NewChatService(NewBroker(), fleet)
	_, _, err := service.run(context.Background(), chatJobRequest{BrowserID: "chatgpt", Kind: "chatgpt.prepare_direct"})
	if err == nil || !isChatGPTChallengeError(err.Error()) {
		t.Fatalf("expected retained challenge error, got %v", err)
	}
}

func TestChatServiceDispatchesBrowserJob(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{config: Config{ChatGPTDefaultID: "chatgpt"}, sessions: map[string]*ChromeSession{"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true}}}
	service := NewChatService(broker, fleet)
	service.ready = func(session *ChromeSession) bool { return session.Ready() }
	service.pressEnter = func(_ *ChromeSession, _ string, _ <-chan struct{}) error { return nil }

	done := make(chan map[string]any, 1)
	go func() {
		result, err := service.Generate(context.Background(), "", "hello")
		if err != nil {
			done <- map[string]any{"error": err.Error()}
			return
		}
		done <- result
	}()

	deadline := time.Now().Add(time.Second)
	var job *Job
	for job == nil && time.Now().Before(deadline) {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("chat job was not dispatched")
	}
	if job.Payload["kind"] != "chatgpt.generate" || job.Payload["prompt"] != "hello" || job.Payload["submitNonce"] == "" {
		t.Fatalf("unexpected payload: %#v", job.Payload)
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"text": "answer"})
	result := <-done
	if result["text"] != "answer" || result["browserId"] != "chatgpt" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestChatServicePreparesDirectJob(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{
		config: Config{ChatGPTDefaultID: "chatgpt"},
		sessions: map[string]*ChromeSession{
			"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
		},
	}
	service := NewChatService(broker, fleet)
	service.ready = func(session *ChromeSession) bool { return session.Ready() }
	pressed := make(chan struct{}, 1)
	service.pressEnter = func(_ *ChromeSession, _ string, _ <-chan struct{}) error {
		pressed <- struct{}{}
		return nil
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := service.run(context.Background(), chatJobRequest{
			BrowserID: "chatgpt",
			Kind:      "chatgpt.prepare_direct",
		})
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	var job *Job
	for job == nil && time.Now().Before(deadline) {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("direct chat job was not dispatched")
	}
	if job.Payload["kind"] != "chatgpt.prepare_direct" || job.Payload["prompt"] != "" {
		t.Fatalf("unexpected payload: %#v", job.Payload)
	}
	select {
	case <-pressed:
	case <-time.After(time.Second):
		t.Fatal("direct probe was not submitted through Chrome")
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"headers": map[string]string{"x-conduit-token": "test"}})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestChatServiceAcceptsCompletedProbeBeforeChromeClick(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{config: Config{ChatGPTDefaultID: "chatgpt"}, sessions: map[string]*ChromeSession{
		"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
	}}
	service := NewChatService(broker, fleet)
	service.ready = func(session *ChromeSession) bool { return session.Ready() }
	service.pressEnter = func(_ *ChromeSession, _ string, done <-chan struct{}) error {
		select {
		case <-done:
			return nil
		case <-time.After(time.Second):
			return errors.New("completed probe was not observed")
		}
	}

	result := make(chan error, 1)
	go func() {
		_, _, err := service.run(context.Background(), chatJobRequest{BrowserID: "chatgpt", Kind: "chatgpt.prepare_direct"})
		result <- err
	}()
	var job *Job
	for deadline := time.Now().Add(time.Second); job == nil && time.Now().Before(deadline); {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("direct chat job was not dispatched")
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"headers": map[string]string{"x-conduit-token": "test"}})
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestChatServiceRecoversDisconnectedExtensionBeforeSubmitting(t *testing.T) {
	broker := NewBroker()
	fleet := &Fleet{
		config: Config{ChatGPTDefaultID: "chatgpt"},
		sessions: map[string]*ChromeSession{
			"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
		},
	}
	service := NewChatService(broker, fleet)
	service.ready = func(session *ChromeSession) bool { return session.Ready() }
	service.prepare = func(session *ChromeSession) error {
		session.providerReady = true
		broker.mu.Lock()
		broker.heartbeats["chatgpt"] = time.Now()
		broker.mu.Unlock()
		return nil
	}
	pressed := false
	service.pressEnter = func(_ *ChromeSession, _ string, _ <-chan struct{}) error {
		pressed = true
		return nil
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.Generate(context.Background(), "chatgpt", "hello")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	var job *Job
	for job == nil && time.Now().Before(deadline) {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("recovered browser did not receive the job")
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"text": "answer"})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !pressed {
		t.Fatal("recovered composer was not submitted")
	}
}
