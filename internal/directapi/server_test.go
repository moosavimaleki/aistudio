package directapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
)

type fakeGenerator struct {
	calls []chatgptdirect.Input
	err   error
}

func (f *fakeGenerator) Generate(_ context.Context, input chatgptdirect.Input) (chatgptdirect.Result, error) {
	f.calls = append(f.calls, input)
	return chatgptdirect.Result{Text: "answer", ConversationID: "thread-1", ParentMessageID: "answer-1"}, f.err
}

func TestChatContinuation(t *testing.T) {
	client := &fakeGenerator{calls: []chatgptdirect.Input{}}
	handler := NewHandler(client, "chatgpt/gpt-6")
	for _, body := range []string{`{"message":"hello"}`, `{"message":"again","conversation_id":"thread-1"}`} {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/chat", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["reply"] != "answer" || result["conversation_id"] != "thread-1" {
			t.Fatal(result)
		}
	}
	if len(client.calls) != 2 || client.calls[1].ConversationID != "thread-1" {
		t.Fatal(client.calls)
	}
}

func TestChatRejectsInvalidRequestsWithoutSending(t *testing.T) {
	for _, tc := range []struct {
		name, body, host, origin string
		status                   int
	}{
		{name: "invalid JSON", body: `{`, host: "localhost", status: 400},
		{name: "empty", body: `{"message":" "}`, host: "localhost", status: 400},
		{name: "multiple objects", body: `{"message":"a"}{}`, host: "localhost", status: 400},
		{name: "remote host", body: `{"message":"a"}`, host: "example.com", status: 403},
		{name: "origin", body: `{"message":"a"}`, host: "localhost", origin: "https://example.com", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeGenerator{}
			handler := NewHandler(client, "model")
			r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/chat", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status || len(client.calls) != 0 {
				t.Fatal(w.Code, client.calls)
			}
		})
	}
}

func TestUpstreamErrorIsNotRetried(t *testing.T) {
	client := &fakeGenerator{err: errors.New("uncertain send")}
	handler := NewHandler(client, "model")
	r := httptest.NewRequest(http.MethodPost, "http://localhost/chat", strings.NewReader(`{"message":"hello"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 502 || len(client.calls) != 1 {
		t.Fatal(w.Code, len(client.calls))
	}
}
