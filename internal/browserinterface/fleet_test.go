package browserinterface

import "testing"

func TestFleetHealthAcceptsAnyReadyBrowser(t *testing.T) {
	statuses := []map[string]any{
		{"browserId": "default", "connected": true, "ready": false},
		{"browserId": "browser2", "connected": true, "ready": true},
	}
	if !healthyStatuses(statuses) {
		t.Fatal("expected fleet to stay healthy while browser2 is usable")
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
