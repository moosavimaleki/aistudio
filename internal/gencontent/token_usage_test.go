package gencontent

import (
	"encoding/json"
	"testing"
)

func TestEstimateChatUsage(t *testing.T) {
	usage := estimateChatUsage([]chatMessage{
		{Role: "system", Content: json.RawMessage(`"Answer briefly"`)},
		{Role: "user", Content: json.RawMessage(`"Hello"`)},
	}, "Hi")
	if usage.PromptTokens <= 0 || usage.CompletionTokens <= 0 || usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
		t.Fatalf("invalid usage: %#v", usage)
	}
}
