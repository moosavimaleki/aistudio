package browserinterface

import "testing"

func TestProviderIndexUsesNewestCandidate(t *testing.T) {
	index, err := providerIndexFromSnapshot(map[string]any{
		"candidateTokens": []any{"first", "second"},
	})
	if err != nil {
		t.Fatalf("provider index: %v", err)
	}
	if index != 1 {
		t.Fatalf("provider index = %d, want 1", index)
	}
}

func TestProviderIndexSupportsSingleToken(t *testing.T) {
	index, err := providerIndexFromSnapshot(map[string]any{"token": "only-candidate"})
	if err != nil {
		t.Fatalf("provider index: %v", err)
	}
	if index != 0 {
		t.Fatalf("provider index = %d, want 0", index)
	}
}

func TestSelectedProviderIsKeptForTheSameSession(t *testing.T) {
	service := &TokenService{activated: map[string]providerActivation{}}
	service.activate("default", "session-a", 2)

	index, found := service.selectedProvider("default", "session-a")
	if !found || index != 2 {
		t.Fatalf("selected provider = (%d, %v), want (2, true)", index, found)
	}
	if _, found := service.selectedProvider("default", "session-b"); found {
		t.Fatal("provider selection leaked into a different browser session")
	}

	service.deactivate("default")
	if _, found := service.selectedProvider("default", "session-a"); found {
		t.Fatal("provider remained active after browser recovery")
	}
}
