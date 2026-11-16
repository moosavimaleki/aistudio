package chatgptdirect

import (
	"encoding/json"
	"testing"
)

func TestGPT6PayloadsMatchCapturedModels(t *testing.T) {
	for _, fixture := range []struct{ name, slug, effort string }{
		{"chatgpt/gpt-6", "gpt-6", ""},
		{"chatgpt/gpt-6-thinking-standard", "gpt-6-thinking", "standard"},
		{"chatgpt/gpt-6-thinking-extended", "gpt-6-thinking", "extended"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			model, err := ResolveModel(fixture.name)
			if err != nil {
				t.Fatal(err)
			}
			payload, _, err := buildPayload(Input{Messages: []Message{{Role: "user", Content: "hello"}}}, model, "parent", BrowserContext{})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire["model"] != fixture.slug || wire["parent_message_id"] != "parent" {
				t.Fatalf("wrong model/parent: %s", encoded)
			}
			effort, present := wire["thinking_effort"]
			if fixture.effort == "" && present || fixture.effort != "" && effort != fixture.effort {
				t.Fatalf("wrong effort: %s", encoded)
			}
			encodings := wire["supported_encodings"].([]any)
			if len(encodings) != 1 || encodings[0] != "v1" {
				t.Fatalf("wrong encoding: %v", encodings)
			}
		})
	}
}
