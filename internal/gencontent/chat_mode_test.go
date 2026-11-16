package gencontent

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatModes(t *testing.T) {
	for _, mode := range []string{"", "browser", "headless", "direct", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("CHATGPT_MODE", mode)
			t.Setenv("CHATGPT_DIRECT_NATIVE", "")
			got, err := chatMode()
			if (err != nil) != (mode == "invalid") {
				t.Fatal(got, err)
			}
			if err == nil && got != mode {
				t.Fatal(got)
			}
		})
	}
}

func TestDirectHealthDoesNotContactBrowser(t *testing.T) {
	s := &Server{chatMode: "direct", direct: &directChatStub{}}
	w := httptest.NewRecorder()
	s.health(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestDefaultModelUsesConfiguredTransport(t *testing.T) {
	for _, mode := range []string{"browser", "headless", "direct"} {
		t.Run(mode, func(t *testing.T) {
			direct := &directChatStub{}
			server := &Server{chatMode: mode, chat: &chatStub{}, direct: direct}
			response := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt-web","messages":[{"role":"user","content":"hello"}]}`))
			server.chatCompletions(response, request)
			if response.Code != 200 {
				t.Fatal(response.Code, response.Body.String())
			}
			want := "browser-ui"
			if mode == "direct" {
				want = "go-direct"
			}
			if !strings.Contains(response.Body.String(), `"transport":"`+want+`"`) {
				t.Fatal(response.Body.String(), want)
			}
			if mode == "direct" && direct.input.Model != "chatgpt/gpt-5.6-thinking-extended" {
				t.Fatal(direct.input)
			}
		})
	}
}
