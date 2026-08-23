package gencontent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

type chatStub struct {
	prompt string
}

func (s *chatStub) Generate(_ context.Context, prompt, _ string) (chatgptweb.Result, error) {
	s.prompt = prompt
	return chatgptweb.Result{Text: "answer", BrowserID: "default", ConversationID: "c1"}, nil
}

func TestOpenAIChatCompletions(t *testing.T) {
	chat := &chatStub{}
	server := &Server{chat: chat}
	body := `{"model":"anything","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if chat.prompt != "System:\nbe brief\n\nUser:\nhello" {
		t.Fatalf("unexpected prompt: %q", chat.prompt)
	}
	var value map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["object"] != "chat.completion" || value["model"] != "chatgpt-web" {
		t.Fatalf("unexpected response: %#v", value)
	}
}
