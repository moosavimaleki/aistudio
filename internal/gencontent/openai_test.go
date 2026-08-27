package gencontent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

type chatStub struct {
	prompt      string
	imagePrompt string
	err         error
}

type directChatStub struct {
	input chatgptdirect.Input
	err   error
}

type conversationRouteStub struct {
	model, browserID string
	messages         []chatgptdirect.Message
	route            chatConversationRoute
	err              error
}

func (s *conversationRouteStub) Route(
	_ context.Context,
	model, browserID string,
	messages []chatgptdirect.Message,
	_ directChatCompleter,
) (chatConversationRoute, error) {
	s.model, s.browserID, s.messages = model, browserID, messages
	return s.route, s.err
}

func (s *directChatStub) Generate(_ context.Context, input chatgptdirect.Input) (chatgptdirect.Result, error) {
	s.input = input
	if s.err != nil {
		return chatgptdirect.Result{}, s.err
	}
	return chatgptdirect.Result{
		Text:            "direct answer",
		ConversationID:  "conversation-2",
		ParentMessageID: "message-2",
		BrowserID:       "chatgpt",
		Model:           input.Model,
		UpstreamStatus:  http.StatusOK,
		UpstreamPath:    "/backend-api/f/conversation",
	}, nil
}

func TestOpenAIDirectChatPreservesBridgeStatus(t *testing.T) {
	direct := &directChatStub{err: &chatgptdirect.BridgeError{
		Status:  http.StatusServiceUnavailable,
		Message: "browser is not ready",
	}}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{"model":"chatgpt/gpt-5.6-pro","messages":[{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func (s *chatStub) Generate(_ context.Context, prompt, _ string) (chatgptweb.Result, error) {
	s.prompt = prompt
	return chatgptweb.Result{Text: "answer", BrowserID: "chatgpt", ConversationID: "c1", UpstreamStatus: 200}, s.err
}

func (s *chatStub) GenerateImage(_ context.Context, prompt, _ string) (chatgptweb.Result, error) {
	s.imagePrompt = prompt
	return chatgptweb.Result{
		Images:         []chatgptweb.Image{{MIMEType: "image/png", Data: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}},
		BrowserID:      "chatgpt",
		ConversationID: "c1",
		UpstreamStatus: 200,
	}, s.err
}

func TestOpenAIModels(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"chatgpt-web"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIChatCompletionsBufferedStream(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"chatgpt-web","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || response.Header().Get("X-Lab-Streaming-Mode") != "buffered" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	if strings.Count(response.Body.String(), "data: ") != 3 || !strings.Contains(response.Body.String(), `"finish_reason":"stop"`) {
		t.Fatalf("unexpected stream: %s", response.Body.String())
	}
}

func TestOpenAIChatCompletionsRejectsUnsupportedRole(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"chatgpt-web","messages":[{"role":"tool","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "unsupported message role") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIChatCompletionsPreservesBridgeStatus(t *testing.T) {
	server := &Server{chat: &chatStub{err: &chatgptweb.BridgeError{Status: http.StatusGatewayTimeout, Message: "timeout"}}}
	body := `{"model":"chatgpt-web","messages":[{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIImageGeneration(t *testing.T) {
	chat := &chatStub{}
	server := &Server{chat: chat}
	body := `{"model":"chatgpt-web","prompt":"draw a circle","response_format":"b64_json"}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body)))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"mime_types":["image/png"]`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(chat.imagePrompt, "Generate one image") {
		t.Fatalf("unexpected image prompt: %q", chat.imagePrompt)
	}
}

func TestOpenAIImageGenerationRejectsMultipleImages(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"chatgpt-web","prompt":"draw","n":2}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body)))

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "only one image") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIChatCompletions(t *testing.T) {
	chat := &chatStub{}
	server := &Server{chat: chat}
	body := `{"model":"chatgpt-web","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hello"}]}`
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

func TestOpenAIDirectChatContinuesConversation(t *testing.T) {
	direct := &directChatStub{}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{
		"model":"chatgpt/gpt-5.6-pro",
		"browser_id":"chatgpt",
		"conversation_id":"conversation-1",
		"parent_message_id":"message-1",
		"messages":[{"role":"user","content":"continue"}]
	}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if direct.input.ConversationID != "conversation-1" || direct.input.ParentMessageID != "message-1" {
		t.Fatalf("continuation was not forwarded: %#v", direct.input)
	}
	if !strings.Contains(response.Body.String(), `"parent_message_id":"message-2"`) ||
		!strings.Contains(response.Body.String(), `"transport":"go-direct"`) {
		t.Fatalf("missing continuation metadata: %s", response.Body.String())
	}
}

func TestOpenAIDirectChatRoutesRequestsWithoutConversationIDs(t *testing.T) {
	direct := &directChatStub{}
	router := &conversationRouteStub{route: chatConversationRoute{
		Input: chatgptdirect.Input{
			Model: "chatgpt/gpt-5.6-pro", BrowserID: "chatgpt2",
			ConversationID: "conversation-root", ParentMessageID: "message-root",
			IncludeHistory: true,
		},
		finish: func(context.Context, chatgptdirect.Result) error { return nil },
		abort:  func(context.Context) error { return nil },
	}}
	server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
	body := `{"model":"chatgpt/gpt-5.6-pro","messages":[{"role":"system","content":"brief"},{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if router.model != "chatgpt/gpt-5.6-pro" || len(router.messages) != 2 {
		t.Fatalf("router did not receive the complete OpenAI history: %#v", router)
	}
	if direct.input.ConversationID != "conversation-root" || direct.input.ParentMessageID != "message-root" || !direct.input.IncludeHistory {
		t.Fatalf("direct input did not use the automatic route: %#v", direct.input)
	}
}

func TestOpenAIDirectChatReturns503WhenConversationPoolIsBusy(t *testing.T) {
	server := &Server{
		chat:          &chatStub{},
		direct:        &directChatStub{},
		conversations: &conversationRouteStub{err: fmt.Errorf("ChatGPT conversation pool is busy after waiting 5s")},
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
