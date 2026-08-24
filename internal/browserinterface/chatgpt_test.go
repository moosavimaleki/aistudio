package browserinterface

import (
	"context"
	"testing"
	"time"
)

func TestChatServiceDispatchesBrowserJob(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{config: Config{ChatGPTDefaultID: "chatgpt"}, sessions: map[string]*ChromeSession{"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true}}}
	service := NewChatService(broker, fleet)
	service.pressEnter = func(_ *ChromeSession, _ string) error { return nil }

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
	service.pressEnter = func(_ *ChromeSession, _ string) error { return nil }

	done := make(chan error, 1)
	go func() {
		_, _, err := service.run(context.Background(), chatJobRequest{
			BrowserID:       "chatgpt",
			Prompt:          "hello",
			Kind:            "chatgpt.prepare_direct",
			Model:           "gpt-5-6-pro",
			ConversationID:  "conversation-1",
			ParentMessageID: "message-1",
			ThinkingEffort:  "standard",
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
	if job.Payload["kind"] != "chatgpt.prepare_direct" ||
		job.Payload["model"] != "gpt-5-6-pro" ||
		job.Payload["conversationId"] != "conversation-1" ||
		job.Payload["parentMessageId"] != "message-1" {
		t.Fatalf("unexpected payload: %#v", job.Payload)
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"headers": map[string]string{"x-conduit-token": "test"}})
	if err := <-done; err != nil {
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
	service.prepare = func(session *ChromeSession) error {
		session.providerReady = true
		broker.mu.Lock()
		broker.heartbeats["chatgpt"] = time.Now()
		broker.mu.Unlock()
		return nil
	}
	pressed := false
	service.pressEnter = func(_ *ChromeSession, _ string) error {
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
