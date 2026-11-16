package chatgptdirect

import (
	"context"
	"testing"
)

func TestCompletedStreamDoesNotReadHistory(t *testing.T) {
	client := &Client{parents: map[string]string{}}
	stream := parseEventStream([]byte("data: {\"conversation_id\":\"thread\",\"message\":{\"id\":\"answer\",\"author\":{\"role\":\"assistant\"},\"channel\":\"final\",\"status\":\"finished_successfully\",\"content\":{\"parts\":[\"hello\"]}}}\n\ndata: [DONE]\n\n"))
	result, err := client.confirmTurn(context.Background(), stream, conversationPayload{}, "model", "profile")
	if err != nil || result.Text != "hello" || result.ParentMessageID != "answer" {
		t.Fatal(result, err)
	}
	if client.parents["profile\x00thread"] != "answer" {
		t.Fatal("continuation parent was not saved")
	}
}

func TestIncompleteAndAnalysisSnapshotsCannotComplete(t *testing.T) {
	for _, status := range []string{"in_progress", "finished_successfully"} {
		for _, channel := range []string{"analysis", "final"} {
			data := "data: {\"message\":{\"id\":\"m\",\"author\":{\"role\":\"assistant\"},\"channel\":\"" + channel + "\",\"status\":\"" + status + "\",\"content\":{\"parts\":[\"text\"]}}}\n\n"
			result := parseEventStream([]byte(data))
			if result.Done {
				t.Fatal("missing terminator accepted")
			}
			if result.Final && (channel != "final" || status != "finished_successfully") {
				t.Fatal(result)
			}
		}
	}
}
