package browserinterface

import (
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

func TestTokenFactoryValidatesVideoPromptDigest(t *testing.T) {
	t.Setenv("AISTUDIO_UPSTREAM_CONFIG", "../../config/upstream.yaml")
	payload, err := aistudio.BuildVideoPayload(aistudio.VideoInput{
		Model: aistudio.VideoModel, Prompt: "synthetic video test",
	})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := aistudio.RPCURL("CreateInteractionStream")
	digest, _ := aistudio.VideoContentDigest(payload)
	upstream, _ := aistudio.LoadUpstream()
	body := map[string]any{
		"digest": digest, "cookies": "SAPISID=test", "authorization": "test-auth",
		"authUser": "0", "waaApiKey": upstream.Opaque["waa_api_key"],
		"generateRequest": map[string]any{
			"url": endpoint, "method": "POST", "payload": payload,
			"headers": map[string]string{
				"authorization": "test-auth", "cookie": "SAPISID=test",
				"origin": upstream.AIStudio["origin"], "user-agent": "synthetic-browser",
				"x-client-data": "synthetic-client", "x-goog-api-key": "test-key", "x-goog-authuser": "0",
			},
		},
	}
	if _, _, err := ValidateTokenRequest(body); err != nil {
		t.Fatalf("valid video context rejected: %v", err)
	}
	body["digest"] = strings.Repeat("0", 64)
	if _, _, err := ValidateTokenRequest(body); err == nil {
		t.Fatal("video prompt and digest mismatch was accepted")
	}
}
