package browserinterface

import (
	"testing"
	"time"
)

func TestResolveChatGPTRotatesAcrossReadyBrowsersWhenUnspecified(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt"] = time.Now()
	broker.heartbeats["chatgpt2"] = time.Now()
	fleet := &Fleet{
		broker: broker,
		config: Config{
			ChatGPTDefaultID: "chatgpt",
			Browsers: []BrowserSpec{
				{ID: "chatgpt", Provider: ProviderChatGPT},
				{ID: "chatgpt2", Provider: ProviderChatGPT},
			},
		},
		sessions: map[string]*ChromeSession{
			"chatgpt":  {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
			"chatgpt2": {spec: BrowserSpec{ID: "chatgpt2", Provider: ProviderChatGPT}, providerReady: true},
		},
	}

	first, err := fleet.ResolveChatGPT("")
	if err != nil {
		t.Fatal(err)
	}
	second, err := fleet.ResolveChatGPT("")
	if err != nil {
		t.Fatal(err)
	}
	if first != "chatgpt" || second != "chatgpt2" {
		t.Fatalf("expected ready browsers to rotate, got %q then %q", first, second)
	}
}

func TestResolveChatGPTSkipsDisconnectedBrowser(t *testing.T) {
	broker := NewBroker()
	broker.heartbeats["chatgpt2"] = time.Now()
	fleet := &Fleet{
		broker: broker,
		config: Config{
			ChatGPTDefaultID: "chatgpt",
			Browsers: []BrowserSpec{
				{ID: "chatgpt", Provider: ProviderChatGPT},
				{ID: "chatgpt2", Provider: ProviderChatGPT},
			},
		},
		sessions: map[string]*ChromeSession{
			"chatgpt":  {spec: BrowserSpec{ID: "chatgpt", Provider: ProviderChatGPT}, providerReady: true},
			"chatgpt2": {spec: BrowserSpec{ID: "chatgpt2", Provider: ProviderChatGPT}, providerReady: true},
		},
	}

	resolved, err := fleet.ResolveChatGPT("")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "chatgpt2" {
		t.Fatalf("expected connected browser chatgpt2, got %q", resolved)
	}
}

func TestFleetHealthAcceptsAnyReadyBrowser(t *testing.T) {
	statuses := []map[string]any{
		{"browserId": "default", "provider": ProviderAIStudio, "connected": true, "ready": false},
		{"browserId": "browser2", "provider": ProviderAIStudio, "connected": true, "ready": true},
	}
	if !healthyStatuses(statuses) {
		t.Fatal("expected fleet to stay healthy while browser2 is usable")
	}
}

func TestFleetHealthRequiresOneBrowserPerProvider(t *testing.T) {
	statuses := []map[string]any{
		{"browserId": "default", "provider": ProviderAIStudio, "connected": true, "ready": true},
		{"browserId": "chatgpt", "provider": ProviderChatGPT, "connected": false, "ready": true},
	}
	if healthyStatuses(statuses) {
		t.Fatal("expected unhealthy ChatGPT provider to fail fleet health")
	}
	statuses = append(statuses, map[string]any{"browserId": "chatgpt2", "provider": ProviderChatGPT, "connected": true, "ready": true})
	if !healthyStatuses(statuses) {
		t.Fatal("expected one healthy ChatGPT browser to satisfy provider health")
	}
}

func TestFleetHealthRejectsFleetWithoutReadyBrowser(t *testing.T) {
	statuses := []map[string]any{
		{"browserId": "default", "connected": true, "ready": false},
		{"browserId": "browser2", "connected": false, "ready": true},
	}
	if healthyStatuses(statuses) {
		t.Fatal("expected fleet without a connected ready browser to be unhealthy")
	}
}

func TestChatGPTPendingJobIsReportedAsActive(t *testing.T) {
	broker := NewBroker()
	job := &Job{ID: "job-1", BrowserID: "chatgpt", result: make(chan jobResult, 1)}
	broker.jobs[job.ID] = job
	health := broker.Health("chatgpt")
	pendingJobs, ok := health["pendingJobs"].(int)
	if !ok || pendingJobs != 1 {
		t.Fatalf("expected one active ChatGPT job, got %#v", health["pendingJobs"])
	}
}

func TestSessionStateDoesNotTrustStaleReadiness(t *testing.T) {
	if state := sessionState(true, map[string]any{"connected": false}); state != "DISCONNECTED" {
		t.Fatalf("expected disconnected state, got %s", state)
	}
	if state := sessionState(true, map[string]any{"connected": true}); state != "READY" {
		t.Fatalf("expected ready state, got %s", state)
	}
}
