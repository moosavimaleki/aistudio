package gencontent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

type chatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	BrowserID       string        `json:"browser_id"`
	ConversationID  string        `json:"conversation_id"`
	ParentMessageID string        `json:"parent_message_id"`
}

type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (s *Server) chatCompletions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body chatCompletionRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	if strings.TrimSpace(body.Model) == "" {
		openAIError(writer, http.StatusBadRequest, "model is required", "invalid_request_error")
		return
	}
	result, parentMessageID, transport, err := s.completeChat(request.Context(), body)
	if err != nil {
		openAIError(writer, chatCompletionStatus(err), err.Error(), "chatgpt_client_error")
		return
	}
	if strings.TrimSpace(result.Text) == "" {
		openAIError(writer, http.StatusBadGateway, "ChatGPT returned an empty final message", "browser_bridge_error")
		return
	}
	model := body.Model
	id := fmt.Sprintf("chatcmpl-web-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	if body.Stream {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("X-Lab-Streaming-Mode", "buffered")
		metadata := chatLabMetadata(body, result, parentMessageID, transport)
		content := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": result.Text}, "finish_reason": nil}}, "lab_metadata": metadata}
		finished := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}, "lab_metadata": metadata}
		fmt.Fprintf(writer, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", mustJSON(content), mustJSON(finished))
		return
	}
	response := map[string]any{
		"id": id, "object": "chat.completion", "created": created, "model": model,
		"choices":      []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": result.Text}, "finish_reason": "stop"}},
		"lab_metadata": chatLabMetadata(body, result, parentMessageID, transport),
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) completeChat(
	ctx context.Context,
	body chatCompletionRequest,
) (chatgptweb.Result, string, string, error) {
	if body.Model == "chatgpt-web" {
		prompt, err := renderChatPrompt(body.Messages)
		if err != nil {
			return chatgptweb.Result{}, "", "", err
		}
		result, err := s.chat.Generate(ctx, prompt, body.BrowserID)
		return result, "", "browser-ui", err
	}
	if _, err := chatgptdirect.ResolveModel(body.Model); err != nil {
		return chatgptweb.Result{}, "", "", err
	}
	if s.directErr != nil {
		return chatgptweb.Result{}, "", "", s.directErr
	}
	if s.direct == nil {
		return chatgptweb.Result{}, "", "", fmt.Errorf("ChatGPT direct client is unavailable")
	}
	messages, err := directMessages(body.Messages)
	if err != nil {
		return chatgptweb.Result{}, "", "", err
	}
	input := chatgptdirect.Input{
		Model:           body.Model,
		Messages:        messages,
		BrowserID:       body.BrowserID,
		ConversationID:  body.ConversationID,
		ParentMessageID: body.ParentMessageID,
	}
	var route chatConversationRoute
	if input.ConversationID == "" && input.ParentMessageID == "" && s.conversations != nil {
		if s.conversationErr != nil {
			return chatgptweb.Result{}, "", "", s.conversationErr
		}
		route, err = s.conversations.Route(ctx, body.Model, body.BrowserID, messages, s.direct)
		if err != nil {
			return chatgptweb.Result{}, "", "", err
		}
		input = route.Input
		defer route.Abort(context.Background())
	}
	result, err := s.direct.Generate(ctx, input)
	if err != nil {
		return chatgptweb.Result{}, "", "", err
	}
	if route.finish != nil {
		if err := route.Finish(context.Background(), result); err != nil {
			return chatgptweb.Result{}, "", "", err
		}
	}
	converted := chatgptweb.Result{
		Text:           result.Text,
		ConversationID: result.ConversationID,
		BrowserID:      result.BrowserID,
		UpstreamStatus: result.UpstreamStatus,
		UpstreamPath:   result.UpstreamPath,
	}
	return converted, result.ParentMessageID, "go-direct", nil
}

func directMessages(messages []chatMessage) ([]chatgptdirect.Message, error) {
	result := make([]chatgptdirect.Message, 0, len(messages))
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "system" && role != "developer" && role != "user" && role != "assistant" {
			return nil, fmt.Errorf("unsupported message role: %s", role)
		}
		text, err := chatContentText(message.Content)
		if err != nil {
			return nil, err
		}
		result = append(result, chatgptdirect.Message{Role: role, Content: text})
	}
	return result, nil
}

func renderChatPrompt(messages []chatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("messages is required")
	}
	blocks := make([]string, 0, len(messages))
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "" {
			return "", fmt.Errorf("message role is required")
		}
		if role != "system" && role != "developer" && role != "user" && role != "assistant" {
			return "", fmt.Errorf("unsupported message role: %s", role)
		}
		text, err := chatContentText(message.Content)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, strings.ToUpper(role[:1])+role[1:]+":\n"+text)
	}
	return strings.Join(blocks, "\n\n"), nil
}

func (s *Server) openAIModels(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	models := []any{openAIModel("chatgpt-web", "browser-ui")}
	for _, name := range chatgptdirect.ModelNames() {
		models = append(models, openAIModel(name, "go-direct"))
	}
	if request.URL.Path == "/v1/models" {
		writeJSON(writer, http.StatusOK, map[string]any{"object": "list", "data": models})
		return
	}
	name := strings.TrimPrefix(request.URL.Path, "/v1/models/")
	if name == "chatgpt-web" {
		writeJSON(writer, http.StatusOK, openAIModel(name, "browser-ui"))
		return
	}
	if _, err := chatgptdirect.ResolveModel(name); err == nil {
		writeJSON(writer, http.StatusOK, openAIModel(name, "go-direct"))
		return
	}
	openAIError(writer, http.StatusNotFound, "model not found", "invalid_request_error")
}

func openAIModel(name, transport string) map[string]any {
	return map[string]any{
		"id": name, "object": "model", "created": int64(0), "owned_by": "lab",
		"lab_transport": transport,
	}
}

func chatLabMetadata(
	body chatCompletionRequest,
	result chatgptweb.Result,
	parentMessageID string,
	transport string,
) map[string]any {
	return map[string]any{
		"browser_id":        result.BrowserID,
		"conversation_id":   result.ConversationID,
		"parent_message_id": parentMessageID,
		"requested_model":   body.Model,
		"upstream_status":   result.UpstreamStatus,
		"upstream_path":     result.UpstreamPath,
		"transport":         transport,
	}
}

func openAIError(writer http.ResponseWriter, status int, message, kind string) {
	writeJSON(writer, status, map[string]any{"error": map[string]any{"message": message, "type": kind}})
}

func chatBridgeStatus(err error) int {
	var bridge *chatgptweb.BridgeError
	if errors.As(err, &bridge) && bridge.Status >= 400 && bridge.Status < 600 {
		return bridge.Status
	}
	return http.StatusBadGateway
}

func chatCompletionStatus(err error) int {
	var upstream *chatgptdirect.UpstreamError
	if errors.As(err, &upstream) && upstream.Status >= 400 && upstream.Status < 600 {
		return upstream.Status
	}
	var bridge *chatgptdirect.BridgeError
	if errors.As(err, &bridge) && bridge.Status >= 400 && bridge.Status < 600 {
		return bridge.Status
	}
	if strings.Contains(err.Error(), "unsupported direct ChatGPT model") ||
		strings.Contains(err.Error(), "messages is required") ||
		strings.Contains(err.Error(), "user message is required") ||
		strings.Contains(err.Error(), "parent_message_id is required") ||
		strings.Contains(err.Error(), "unsupported message role") {
		return http.StatusBadRequest
	}
	if strings.Contains(err.Error(), "ChatGPT conversation pool is busy") {
		return http.StatusServiceUnavailable
	}
	return chatBridgeStatus(err)
}

func chatContentText(raw json.RawMessage) (string, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("message content must be text")
	}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" && part.Type != "input_text" {
			return "", fmt.Errorf("only text message content is supported")
		}
		values = append(values, part.Text)
	}
	return strings.Join(values, "\n"), nil
}
