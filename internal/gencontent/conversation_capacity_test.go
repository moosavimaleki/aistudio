package gencontent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
)

func TestFullPooledConversationIsReplaced(t *testing.T) {
	direct := &directChatStub{failures: []error{&chatgptdirect.UpstreamError{Status: 413, Body: `{"detail":{"code":"conversation_too_large"}}`}}}
	invalidated := false
	router := &conversationRouteStub{routes: []chatConversationRoute{
		{Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-extended", ConversationID: "full"}, invalidate: func(context.Context) error { invalidated = true; return nil }},
		{Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-extended", ConversationID: "fresh"}},
	}}
	server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-5.6-thinking-extended","messages":[{"role":"user","content":"hello"}]}`)))
	if response.Code != 200 || !invalidated || len(direct.inputs) != 2 || direct.inputs[1].ConversationID != "fresh" {
		t.Fatalf("status=%d invalidated=%t inputs=%+v", response.Code, invalidated, direct.inputs)
	}
	if conversationTooLarge(&chatgptdirect.UpstreamError{Status: 413, Body: "request too large"}) {
		t.Fatal("generic size errors must not trigger retries")
	}
}
