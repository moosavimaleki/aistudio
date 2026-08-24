package browserinterface

import "testing"

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
