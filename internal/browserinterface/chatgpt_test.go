package browserinterface

import (
	"context"
	"testing"
	"time"
)

func TestChatServiceDispatchesBrowserJob(t *testing.T) {
	broker := NewBroker()
	fleet := &Fleet{config: Config{ChatGPTDefaultID: "chatgpt"}, sessions: map[string]*ChromeSession{"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}}}}
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
