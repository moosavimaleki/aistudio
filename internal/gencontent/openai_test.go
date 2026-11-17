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
	attachments []chatgptweb.Attachment
	answer      string
	err         error
}

type directChatStub struct {
	input    chatgptdirect.Input
	inputs   []chatgptdirect.Input
	results  []chatgptdirect.Result
	failures []error
	err      error
}

type conversationRouteStub struct {
	model, browserID string
	messages         []chatgptdirect.Message
	route            chatConversationRoute
	routes           []chatConversationRoute
	calls            int
	err              error
}

func (s *conversationRouteStub) Route(
	_ context.Context,
	model, browserID string,
	messages []chatgptdirect.Message,
	_ directChatCompleter,
) (chatConversationRoute, error) {
	s.model, s.browserID, s.messages = model, browserID, messages
	s.calls++
	if index := s.calls - 1; index < len(s.routes) {
		return s.routes[index], s.err
	}
	return s.route, s.err
}

func (s *directChatStub) Generate(_ context.Context, input chatgptdirect.Input) (chatgptdirect.Result, error) {
	s.input = input
	s.inputs = append(s.inputs, input)
	if index := len(s.inputs) - 1; index < len(s.failures) && s.failures[index] != nil {
		return chatgptdirect.Result{}, s.failures[index]
	}
	if s.err != nil {
		return chatgptdirect.Result{}, s.err
	}
	if index := len(s.inputs) - 1; index < len(s.results) {
		return s.results[index], nil
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

func TestOpenAIChatRecreatesMissingPooledConversation(t *testing.T) {
	invalidated := false
	direct := &directChatStub{failures: []error{&chatgptdirect.UpstreamError{
		Status: http.StatusNotFound,
		Body:   `{"detail":{"code":"conversation_not_found"}}`,
	}}}
	router := &conversationRouteStub{routes: []chatConversationRoute{
		{
			Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-off", ConversationID: "stale", ParentMessageID: "stale-parent"},
			abort: func(context.Context) error { return nil },
			invalidate: func(context.Context) error {
				invalidated = true
				return nil
			},
		},
		{
			Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-off", ConversationID: "fresh", ParentMessageID: "fresh-parent"},
			abort: func(context.Context) error { return nil },
		},
	}}
	server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-5.6-thinking-off","messages":[{"role":"user","content":"hello"}]}`)))

	if response.Code != http.StatusOK || router.calls != 2 || !invalidated || len(direct.inputs) != 2 ||
		direct.inputs[1].ConversationID != "fresh" {
		t.Fatalf("stale conversation was not recreated: status=%d calls=%d invalidated=%t inputs=%#v", response.Code, router.calls, invalidated, direct.inputs)
	}
}

func TestConversationNotFoundIncludesDeleted(t *testing.T) {
	deleted := &chatgptdirect.UpstreamError{
		Status: http.StatusNotFound,
		Body:   `{"detail":{"code":"conversation_deleted","can_retry":false}}`,
	}
	if !conversationNotFound(deleted) {
		t.Fatal("a deleted pooled conversation must be invalidated and replaced")
	}
}

func TestOpenAIChatSkipsUnavailableAutomaticPoolBrowser(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			invalidated := false
			direct := &directChatStub{failures: []error{&chatgptdirect.BridgeError{
				Status:  status,
				Message: "ChatGPT browser chatgpt3 is not ready",
			}}}
			router := &conversationRouteStub{routes: []chatConversationRoute{
				{
					Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-off", BrowserID: "chatgpt3", ConversationID: "stale", ParentMessageID: "stale-parent"},
					abort: func(context.Context) error { return nil },
					invalidate: func(context.Context) error {
						invalidated = true
						return nil
					},
				},
				{
					Input: chatgptdirect.Input{Model: "chatgpt/gpt-5.6-thinking-off", BrowserID: "chatgpt", ConversationID: "fresh", ParentMessageID: "fresh-parent"},
					abort: func(context.Context) error { return nil },
				},
			}}
			server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-5.6-thinking-off","messages":[{"role":"user","content":"hello"}]}`)))

			if response.Code != http.StatusOK || router.calls != 2 || !invalidated || len(direct.inputs) != 2 ||
				direct.inputs[1].BrowserID != "chatgpt" {
				t.Fatalf("unavailable automatic browser was reused: status=%d calls=%d invalidated=%t inputs=%#v", response.Code, router.calls, invalidated, direct.inputs)
			}
		})
	}
}

func TestPoolDoesNotRetryAnInFlightTimeout(t *testing.T) {
	err := &chatgptdirect.BridgeError{Status: http.StatusGatewayTimeout, Message: "ChatGPT generation deadline exceeded"}
	if pooledBrowserUnavailable(err) {
		t.Fatal("only browser preparation failures can safely select another profile")
	}
}

func TestOpenAIChatRecreatesMissingExplicitConversation(t *testing.T) {
	direct := &directChatStub{failures: []error{&chatgptdirect.UpstreamError{
		Status: http.StatusNotFound,
		Body:   `{"detail":{"code":"conversation_not_found"}}`,
	}}}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{
		"model":"chatgpt/gpt-5.6-thinking-off",
		"conversation_id":"missing",
		"parent_message_id":"missing-parent",
		"messages":[
			{"role":"user","content":"First question"},
			{"role":"assistant","content":"First answer"},
			{"role":"user","content":"Continue"}
		]
	}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || len(direct.inputs) != 2 || direct.inputs[0].ConversationID != "missing" ||
		direct.inputs[1].ConversationID != "" || direct.inputs[1].ParentMessageID != "" || !direct.inputs[1].IncludeHistory ||
		len(direct.inputs[1].Messages) != 3 {
		t.Fatalf("explicit stale conversation was not recreated: status=%d inputs=%#v", response.Code, direct.inputs)
	}
}

func TestOpenAIDirectChatRepairsUnavailableToolReply(t *testing.T) {
	direct := &directChatStub{results: []chatgptdirect.Result{
		{Text: "ابزار ترمینال خطای موقت می‌دهد", ConversationID: "conversation-1", ParentMessageID: "message-1", BrowserID: "chatgpt", UpstreamStatus: http.StatusOK},
		{Text: `<tool>{"name":"bash","arguments":{"command":"ls"}}</tool>`, ConversationID: "conversation-1", ParentMessageID: "message-2", BrowserID: "chatgpt", UpstreamStatus: http.StatusOK},
	}}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{"model":"chatgpt/gpt-5.6-thinking-off","tools":[{"type":"function","function":{"name":"bash"}}],"messages":[{"role":"user","content":"چه فایل‌هایی هست؟"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"finish_reason":"tool_calls"`) {
		t.Fatalf("tool repair failed: status=%d body=%s", response.Code, response.Body.String())
	}
	if len(direct.inputs) != 2 || direct.inputs[1].ConversationID != "conversation-1" || direct.inputs[1].ParentMessageID != "message-1" ||
		!strings.Contains(direct.inputs[1].Messages[len(direct.inputs[1].Messages)-1].Content, "Correct the previous reply") {
		t.Fatalf("tool repair did not continue the first conversation: %#v", direct.inputs)
	}
}

func TestOpenAIDirectChatPreservesBridgeStatus(t *testing.T) {
	direct := &directChatStub{err: &chatgptdirect.BridgeError{
		Status:  http.StatusServiceUnavailable,
		Message: "browser is not ready",
	}}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{"model":"chatgpt/gpt-5.6-thinking-extended","messages":[{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func (s *chatStub) Generate(_ context.Context, prompt, _ string) (chatgptweb.Result, error) {
	s.prompt = prompt
	answer := s.answer
	if answer == "" {
		answer = "answer"
	}
	return chatgptweb.Result{Text: answer, BrowserID: "chatgpt", ConversationID: "c1", UpstreamStatus: 200}, s.err
}

func (s *chatStub) GenerateImage(_ context.Context, prompt, _ string, attachments []chatgptweb.Attachment) (chatgptweb.Result, error) {
	s.imagePrompt = prompt
	s.attachments = attachments
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

func TestOpenAIChatCompletionsRequiresToolCallID(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"chatgpt-web","messages":[{"role":"tool","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "tool_call_id is required") {
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

func TestOpenAIImageGenerationRejectsUnsupportedModel(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"gpt-image-1","prompt":"draw"}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body)))

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "only model=chatgpt-web") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIImageGenerationRejectsNegativeCount(t *testing.T) {
	server := &Server{chat: &chatStub{}}
	body := `{"model":"chatgpt-web","prompt":"draw","n":-1}`
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

func TestOpenAIChatCompletionsReturnsNestedToolCall(t *testing.T) {
	chat := &chatStub{answer: `<tool>{"name":"weather","arguments":{"city":"Tehran","options":{"units":"c"}}}</tool>`}
	server := &Server{chat: chat}
	body := `{
		"model":"chatgpt-web",
		"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}],
		"messages":[{"role":"user","content":"Weather in Tehran?"}]
	}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var value struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(response.Body.Bytes(), &value) != nil || len(value.Choices) != 1 ||
		value.Choices[0].FinishReason != "tool_calls" || len(value.Choices[0].Message.ToolCalls) != 1 ||
		value.Choices[0].Message.ToolCalls[0].Function.Name != "weather" ||
		value.Choices[0].Message.ToolCalls[0].Function.Arguments != `{"city":"Tehran","options":{"units":"c"}}` {
		t.Fatalf("tool call was not OpenAI-compatible: %s", response.Body.String())
	}
	if !strings.Contains(chat.prompt, "API tool adapter") {
		t.Fatalf("browser UI did not receive the tool instruction: %q", chat.prompt)
	}
}

func TestOpenAIChatPromotesFencedReadOnlyBash(t *testing.T) {
	chat := &chatStub{answer: "```bash\ndocker ps -a\n```"}
	server := &Server{chat: chat}
	body := `{"model":"chatgpt-web","tools":[{"type":"function","function":{"name":"bash"}}],"messages":[{"role":"user","content":"Show containers"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"finish_reason":"tool_calls"`) ||
		!strings.Contains(response.Body.String(), `docker ps -a`) {
		t.Fatalf("read-only command was not promoted: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIChatDoesNotPromoteWritableBash(t *testing.T) {
	chat := &chatStub{answer: "```bash\ndocker ps && rm -rf /tmp/example\n```"}
	server := &Server{chat: chat}
	body := `{"model":"chatgpt-web","tools":[{"type":"function","function":{"name":"bash"}}],"messages":[{"role":"user","content":"Show containers"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"finish_reason":"tool_calls"`) ||
		!strings.Contains(response.Body.String(), "rm -rf") {
		t.Fatalf("writable command was promoted or lost: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIDirectMessagesContinueAfterToolResult(t *testing.T) {
	tools := []openAITool{{Type: "function"}}
	tools[0].Function.Name = "weather"
	plan, err := parseToolPlan(tools, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := directMessages([]chatMessage{
		{Role: "user", Content: json.RawMessage(`"Weather in Tehran?"`)},
		{Role: "assistant", Content: json.RawMessage(`null`), ToolCalls: []openAIToolCall{{ID: "call_1", Type: "function", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "weather", Arguments: `{"city":"Tehran"}`}}}},
		{Role: "tool", ToolCallID: "call_1", Content: json.RawMessage(`"22 C and sunny"`)},
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 4 || !strings.Contains(messages[2].Content, `<tool>{"arguments":{"city":"Tehran"},"name":"weather"}</tool>`) ||
		!strings.Contains(messages[3].Content, "Tool result for call_1 (weather):") {
		t.Fatalf("tool continuation was not preserved: %#v", messages)
	}
}

func TestOpenAIChatStreamToolCallHasIndex(t *testing.T) {
	chat := &chatStub{answer: `<tool>{"name":"weather","arguments":{}}</tool>`}
	server := &Server{chat: chat}
	body := `{"model":"chatgpt-web","stream":true,"tools":[{"type":"function","function":{"name":"weather"}}],"messages":[{"role":"user","content":"weather"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"index":0`) || !strings.Contains(response.Body.String(), `"finish_reason":"tool_calls"`) {
		t.Fatalf("invalid stream: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIChatRequiredToolChoiceRejectsPlainText(t *testing.T) {
	server := &Server{chat: &chatStub{answer: "The weather is sunny."}}
	body := `{"model":"chatgpt-web","tool_choice":"required","tools":[{"type":"function","function":{"name":"weather"}}],"messages":[{"role":"user","content":"weather"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "did not honor the required tool_choice") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAIDirectChatContinuesConversation(t *testing.T) {
	direct := &directChatStub{}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{
		"model":"chatgpt/gpt-5.6-thinking-extended",
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

func TestOpenAIDirectChatUsesThinkingModelName(t *testing.T) {
	direct := &directChatStub{}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{"model":"chatgpt/gpt-5.6-thinking-extended","messages":[{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if direct.input.Model != "chatgpt/gpt-5.6-thinking-extended" {
		t.Fatalf("model was not forwarded: %#v", direct.input)
	}
}

func TestOpenAIDirectChatAddsToolReminderToLatestUser(t *testing.T) {
	direct := &directChatStub{}
	server := &Server{chat: &chatStub{}, direct: direct}
	body := `{
		"model":"chatgpt/gpt-5.6-thinking-off",
		"tools":[{"type":"function","function":{"name":"bash"}}],
		"messages":[
			{"role":"system","content":"Caller policy"},
			{"role":"user","content":"List files"}
		]
	}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK || len(direct.input.Messages) != 3 ||
		!strings.Contains(direct.input.Messages[2].Content, "Client tool protocol") ||
		!strings.Contains(direct.input.Messages[2].Content, "bash") {
		t.Fatalf("tool reminder was not attached to latest user message: status=%d input=%#v", response.Code, direct.input)
	}
}

func TestOpenAIDirectChatRoutesRequestsWithoutConversationIDs(t *testing.T) {
	direct := &directChatStub{}
	router := &conversationRouteStub{route: chatConversationRoute{
		Input: chatgptdirect.Input{
			Model: "chatgpt/gpt-5.6-thinking-extended", BrowserID: "chatgpt2",
			ConversationID: "conversation-root", ParentMessageID: "message-root",
			IncludeHistory: true,
		},
		finish: func(context.Context, chatgptdirect.Result) error { return nil },
		abort:  func(context.Context) error { return nil },
	}}
	server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
	body := `{"model":"chatgpt/gpt-5.6-thinking-extended","messages":[{"role":"system","content":"brief"},{"role":"user","content":"hello"}]}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if router.model != "chatgpt/gpt-5.6-thinking-extended" || len(router.messages) != 2 {
		t.Fatalf("router did not receive the complete OpenAI history: %#v", router)
	}
	if direct.input.ConversationID != "conversation-root" || direct.input.ParentMessageID != "message-root" || !direct.input.IncludeHistory {
		t.Fatalf("direct input did not use the automatic route: %#v", direct.input)
	}
}

func TestOpenAIDirectToolTranscriptBypassesConversationPool(t *testing.T) {
	direct := &directChatStub{}
	router := &conversationRouteStub{err: fmt.Errorf("conversation pool must not receive a tool transcript")}
	server := &Server{chat: &chatStub{}, direct: direct, conversations: router}
	body := `{
		"model":"chatgpt/gpt-5.6-thinking-off",
		"messages":[
			{"role":"user","content":"Show containers"},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"docker ps -a\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"container list"}
		]
	}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(router.messages) != 0 || direct.input.ConversationID != "" || len(direct.input.Messages) != 3 {
		t.Fatalf("tool transcript unexpectedly used the pool: router=%#v input=%#v", router, direct.input)
	}
}

func TestOpenAIDirectChatReturns503WhenConversationPoolIsBusy(t *testing.T) {
	server := &Server{
		chat:          &chatStub{},
		direct:        &directChatStub{},
		conversations: &conversationRouteStub{err: fmt.Errorf("ChatGPT conversation pool is busy after waiting 5s")},
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-5.6-thinking-off","messages":[{"role":"user","content":"hello"}]}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
