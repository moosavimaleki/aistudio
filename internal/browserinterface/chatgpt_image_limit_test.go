package browserinterface

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatGPTImageLimitReturns429(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	fleet := &Fleet{
		broker: broker,
		config: Config{ChatGPTDefaultID: "chatgpt"},
		sessions: map[string]*ChromeSession{
			"chatgpt": {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
		},
	}
	server := NewServer(fleet, broker)
	server.chat.ready = func(*ChromeSession) bool { return true }
	server.chat.pressEnter = func(*ChromeSession, string, <-chan struct{}) error { return nil }

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/chatgpt/generate",
		strings.NewReader(`{"browserId":"chatgpt","prompt":"draw a circle","image":true}`))
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(response, request)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	var job *Job
	for job == nil && time.Now().Before(deadline) {
		job = broker.Next("chatgpt")
		time.Sleep(time.Millisecond)
	}
	if job == nil {
		t.Fatal("image job was not dispatched")
	}
	broker.Complete(job.ID, "chatgpt", map[string]any{"error": "ChatGPT image generation limit reached for this account"})
	<-done
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected HTTP 429, got %d: %s", response.Code, response.Body.String())
	}
}
