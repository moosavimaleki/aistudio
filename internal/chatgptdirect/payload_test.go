package chatgptdirect

import "testing"

func TestBuildPayloadContinuesConversation(t *testing.T) {
	model, err := ResolveModel("chatgpt/gpt-5.6-pro")
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		Model:           model.Name,
		ConversationID:  "conversation-1",
		ParentMessageID: "message-1",
		Messages: []Message{
			{Role: "user", Content: "old question"},
			{Role: "assistant", Content: "old answer"},
			{Role: "user", Content: "new question"},
		},
	}
	payload, prompt, err := buildPayload(input, model, input.ParentMessageID, BrowserContext{Timezone: "Asia/Tehran"})
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "new question" || payload.ConversationID == nil || *payload.ConversationID != "conversation-1" {
		t.Fatalf("unexpected continuation: %#v", payload)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Author["role"] != "user" {
		t.Fatalf("continuation must send only the new turn: %#v", payload.Messages)
	}
}

func TestBuildPayloadAddsHistoryForNewBranch(t *testing.T) {
	model, err := ResolveModel("chatgpt/gpt-5.6-pro")
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		Model: model.Name, ConversationID: "conversation-root", ParentMessageID: "message-root", IncludeHistory: true,
		Messages: []Message{
			{Role: "system", Content: "answer briefly"},
			{Role: "user", Content: "new question"},
		},
	}
	payload, _, err := buildPayload(input, model, input.ParentMessageID, BrowserContext{})
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Author["role"] != "system" || payload.Messages[1].Author["role"] != "user" {
		t.Fatalf("new branch must contain its supplied context and user turn: %#v", payload.Messages)
	}
}

func TestParseEventStreamReturnsContinuationIDs(t *testing.T) {
	data := []byte("event: delta\ndata: {\"p\":\"\",\"o\":\"add\",\"v\":{\"conversation_id\":\"c1\",\"message\":{\"id\":\"m1\",\"author\":{\"role\":\"assistant\"},\"content\":{\"parts\":[\"hel\"]}}}}\n\n" +
		"event: delta\ndata: {\"p\":\"/message/content/parts/0\",\"o\":\"append\",\"v\":\"lo\"}\n\n")
	result := parseEventStream(data)
	if result.Text != "hello" || result.ConversationID != "c1" || result.MessageID != "m1" {
		t.Fatalf("unexpected stream result: %#v", result)
	}
}

func TestParseEventStreamCompressedPatch(t *testing.T) {
	data := []byte(
		"data: {\"p\":\"/message/content/parts/0\",\"o\":\"append\",\"v\":\"hel\"}\n\n" +
			"data: {\"v\":\"lo\"}\n\n",
	)
	result := parseEventStream(data)
	if result.Text != "hello" {
		t.Fatalf("text = %q, want hello", result.Text)
	}
}
