package aistudio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestGeneratePayloadUsesStablePositions(t *testing.T) {
	payload, err := BuildGeneratePayload(GenerateInput{Model: "models/test", Prompt: "سلام", GenerationConfig: map[string]any{"thinkingConfig": map[string]any{"levelEnum": 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := payload[1]; !equalJSON(got, []any{[]any{[]any{[]any{nil, "سلام"}}, "user"}}) {
		t.Fatalf("contents = %#v", got)
	}
	config := payload[3].([]any)
	if got := config[16]; !equalJSON(got, []any{false, nil, nil, 4}) {
		t.Fatalf("thinking config = %#v", got)
	}
	digest, err := ContentBindingDigest(payload)
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte("سلام"))
	if digest != hex.EncodeToString(expected[:]) {
		t.Fatalf("digest = %s", digest)
	}
}

func TestPayloadRejectsMissingThinkingMode(t *testing.T) {
	_, err := BuildGeneratePayload(GenerateInput{Model: "models/test", Prompt: "hello"})
	if err == nil {
		t.Fatal("expected missing thinkingConfig to fail")
	}
}

func TestAuthorizationUsesCookieHeader(t *testing.T) {
	auth, err := NewAuthContext("https://aistudio.google.com/path", "SAPISID=value; __Secure-1PAPISID=one")
	if err != nil {
		t.Fatal(err)
	}
	if got := auth.Authorization(); got == "" {
		t.Fatal("expected SAPISID authorization")
	}
}

func equalJSON(left, right any) bool { return formatJSON(left) == formatJSON(right) }
func formatJSON(value any) string    { data, _ := json.Marshal(value); return string(data) }
