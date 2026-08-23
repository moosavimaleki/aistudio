package browserinterface

import "testing"

func TestObservedTransportProfileKeepsAIStudioTier(t *testing.T) {
	session := &ChromeSession{headers: map[string]string{
		"x-aistudio-g1-tier": "TIER1",
		"x-client-data":      "client-data",
		"cookie":             "must-not-leak",
	}}

	profile := session.observedTransportProfile()
	if profile["x-aistudio-g1-tier"] != "TIER1" {
		t.Fatalf("AI Studio tier = %q", profile["x-aistudio-g1-tier"])
	}
	if _, found := profile["cookie"]; found {
		t.Fatal("cookie leaked into transport profile")
	}
}
