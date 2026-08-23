package gencontent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type chatCompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	BrowserID string        `json:"browser_id"`
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
	prompt, err := renderChatPrompt(body.Messages)
	if err != nil {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"message": err.Error(), "type": "invalid_request_error"}})
		return
	}
	result, err := s.chat.Generate(request.Context(), prompt, body.BrowserID)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, map[string]any{"error": map[string]any{"message": err.Error(), "type": "browser_bridge_error"}})
		return
	}
	model := "chatgpt-web"
	id := fmt.Sprintf("chatcmpl-web-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	if body.Stream {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("X-Lab-Streaming-Mode", "buffered")
		chunk := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": result.Text}, "finish_reason": "stop"}}}
		fmt.Fprintf(writer, "data: %s\n\ndata: [DONE]\n\n", mustJSON(chunk))
		return
	}
	response := map[string]any{
		"id": id, "object": "chat.completion", "created": created, "model": model,
		"choices":      []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": result.Text}, "finish_reason": "stop"}},
		"lab_metadata": map[string]any{"browser_id": result.BrowserID, "conversation_id": result.ConversationID, "requested_model": body.Model, "upstream_status": result.UpstreamStatus, "upstream_path": result.UpstreamPath},
	}
	writeJSON(writer, http.StatusOK, response)
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
		text, err := chatContentText(message.Content)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, strings.ToUpper(role[:1])+role[1:]+":\n"+text)
	}
	return strings.Join(blocks, "\n\n"), nil
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
