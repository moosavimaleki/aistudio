package chatgptdirect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFullSnapshotStream(t *testing.T) {
	data := []byte("data: {\"conversation_id\":\"c1\",\"message\":{\"id\":\"m1\",\"author\":{\"role\":\"assistant\"},\"content\":{\"parts\":[\"answer\"]}}}\n\ndata: [DONE]\n\n")
	result := parseEventStream(data)
	if result.Text != "answer" || result.MessageID != "m1" || result.ConversationID != "c1" {
		t.Fatal(result)
	}
}

func TestSnapshotValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{name: "valid", body: `{"version":1,"url":"https://chatgpt.com/backend-api/conversations","headers":{"Cookie":"fixture=only","Authorization":"Bearer fixture"}}`, valid: true},
		{name: "remote origin", body: `{"version":1,"url":"https://example.com","headers":{"cookie":"fixture","authorization":"Bearer fixture"}}`},
		{name: "missing auth", body: `{"version":1,"url":"https://chatgpt.com","headers":{}}`},
		{name: "invalid JSON", body: `{"secret":"never-log-this"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			headers, err := loadSavedHeaders(path)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && headers["authorization"] != "Bearer fixture" {
				t.Fatal("headers were not normalized")
			}
		})
	}
}
