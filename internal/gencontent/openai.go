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
	"github.com/hamed/aistudio-api/internal/metrics"
)

type chatCompletionRequest struct {
	Model             string          `json:"model"`
	Messages          []chatMessage   `json:"messages"`
	Stream            bool            `json:"stream"`
	BrowserID         string          `json:"browser_id"`
	ConversationID    string          `json:"conversation_id"`
	ParentMessageID   string          `json:"parent_message_id"`
	Tools             []openAITool    `json:"tools"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls"`
}

type chatMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCallID string           `json:"tool_call_id"`
	ToolCalls  []openAIToolCall `json:"tool_calls"`
	Name       string           `json:"name"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
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
	metrics.SetLabel(request.Context(), "model", body.Model)
	plan, err := parseToolPlan(body.Tools, body.ToolChoice, body.ParallelToolCalls)
	if err != nil {
		openAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	result, parentMessageID, transport, err := s.completeChat(request.Context(), body, plan)
	if err != nil {
		openAIError(writer, chatCompletionStatus(err), err.Error(), "chatgpt_client_error")
		return
	}
	toolCalls, content := plan.extract(result.Text)
	if shouldRepairToolCall(body, plan, toolCalls, content) {
		retryBody, retryPlan, retryErr := repairToolCallRequest(body, plan, result, parentMessageID)
		if retryErr != nil {
			openAIError(writer, http.StatusBadGateway, retryErr.Error(), "chatgpt_client_error")
			return
		}
		result, parentMessageID, transport, err = s.completeChat(request.Context(), retryBody, retryPlan)
		if err != nil {
			openAIError(writer, chatCompletionStatus(err), err.Error(), "chatgpt_client_error")
			return
		}
		plan = retryPlan
		toolCalls, content = plan.extract(result.Text)
	}
	if plan.requiresCall() && len(toolCalls) == 0 {
		openAIError(writer, http.StatusBadGateway, "ChatGPT did not honor the required tool_choice", "upstream_error")
		return
	}
	if strings.TrimSpace(content) == "" && len(toolCalls) == 0 {
		openAIError(writer, http.StatusBadGateway, "ChatGPT returned an empty final message", "browser_bridge_error")
		return
	}
	usage := estimateChatUsage(body.Messages, result.Text)
	model := body.Model
	id := fmt.Sprintf("chatcmpl-web-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	if body.Stream {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("X-Lab-Streaming-Mode", "buffered")
		metadata := chatLabMetadata(body, result, parentMessageID, transport)
		metadata["timings_ms"] = metrics.Phases(request.Context())
		metadata["request_id"] = metrics.RequestID(request.Context())
		delta := map[string]any{"role": "assistant"}
		finishReason := "stop"
		if len(toolCalls) > 0 {
			if content != "" {
				delta["content"] = content
			}
			delta["tool_calls"] = streamToolCalls(toolCalls)
			finishReason = "tool_calls"
		} else {
			delta["content"] = content
		}
		content := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": nil}}, "lab_metadata": metadata}
		finished := map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finishReason}}, "usage": usage, "lab_metadata": metadata}
		fmt.Fprintf(writer, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", mustJSON(content), mustJSON(finished))
		return
	}
	message := map[string]any{"role": "assistant"}
	finishReason := "stop"
	if len(toolCalls) > 0 {
		if content == "" {
			message["content"] = nil
		} else {
			message["content"] = content
		}
		message["tool_calls"] = toolCalls
		finishReason = "tool_calls"
	} else {
		message["content"] = content
	}
	response := map[string]any{
		"id": id, "object": "chat.completion", "created": created, "model": model,
		"choices":      []any{map[string]any{"index": 0, "message": message, "finish_reason": finishReason}},
		"usage":        usage,
		"lab_metadata": chatLabMetadata(body, result, parentMessageID, transport),
	}
	metadata := response["lab_metadata"].(map[string]any)
	metadata["timings_ms"] = metrics.Phases(request.Context())
	metadata["request_id"] = metrics.RequestID(request.Context())
	writeJSON(writer, http.StatusOK, response)
}

func shouldRepairToolCall(body chatCompletionRequest, plan toolPlan, calls []openAIToolCall, content string) bool {
	return body.Model != "chatgpt-web" && plan.enabled() && len(calls) == 0 &&
		(plan.requiresCall() || claimsToolUnavailable(content))
}

func repairToolCallRequest(
	body chatCompletionRequest,
	plan toolPlan,
	result chatgptweb.Result,
	parentMessageID string,
) (chatCompletionRequest, toolPlan, error) {
	original, err := latestUserMessage(body.Messages)
	if err != nil {
		return chatCompletionRequest{}, toolPlan{}, err
	}
	repair := plan.repairPrompt(original)
	repairContent, err := json.Marshal(repair)
	if err != nil {
		return chatCompletionRequest{}, toolPlan{}, fmt.Errorf("encode ChatGPT tool repair prompt: %w", err)
	}
	retry := body
	retry.Messages = append(append([]chatMessage(nil), body.Messages...), chatMessage{Role: "user", Content: repairContent})
	retry.ConversationID = result.ConversationID
	retry.ParentMessageID = parentMessageID
	strict := plan
	strict.choice = "required"
	return retry, strict, nil
}

func (s *Server) completeChat(
	ctx context.Context,
	body chatCompletionRequest,
	plan toolPlan,
) (chatgptweb.Result, string, string, error) {
	if body.Model == "chatgpt-web" && s.chatMode != "direct" {
		prompt, err := renderChatPrompt(body.Messages, plan)
		if err != nil {
			return chatgptweb.Result{}, "", "", err
		}
		result, err := s.chat.Generate(ctx, prompt, body.BrowserID)
		return result, "", "browser-ui", err
	}
	if body.Model == "chatgpt-web" && s.chatMode == "direct" {
		body.Model = "chatgpt/gpt-5.6-thinking-extended"
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
	messages, err := directMessages(body.Messages, plan)
	if err != nil {
		return chatgptweb.Result{}, "", "", err
	}
	messages = addToolReminder(messages, body.Messages, plan)
	input := chatgptdirect.Input{
		Model:           body.Model,
		Messages:        messages,
		BrowserID:       body.BrowserID,
		ConversationID:  body.ConversationID,
		ParentMessageID: body.ParentMessageID,
	}
	var route chatConversationRoute
	// A tool-capable request must retain its adapter contract and the full tool
	// transcript. The shared pool optimizes ordinary chat by forwarding only the
	// newest user message after a history hit, which drops that contract.
	if !plan.enabled() && !hasToolTranscript(body.Messages) && input.ConversationID == "" && input.ParentMessageID == "" && s.conversations != nil {
		if s.conversationErr != nil {
			return chatgptweb.Result{}, "", "", s.conversationErr
		}
		routed := metrics.Phase(ctx, "conversation_route")
		route, err = s.conversations.Route(ctx, body.Model, body.BrowserID, messages, s.direct)
		routed()
		if err != nil {
			return chatgptweb.Result{}, "", "", err
		}
		input = route.Input
		defer route.Abort(context.Background())
	}
	result, err := s.direct.Generate(ctx, input)
	for retry := 0; err != nil && retry < 10; retry++ {
		missing := conversationNotFound(err)
		full := route.invalidate != nil && conversationTooLarge(err)
		unavailable := body.BrowserID == "" && pooledBrowserUnavailable(err)
		if !missing && !unavailable && !full {
			break
		}
		if route.invalidate != nil {
			if invalidateErr := route.Invalidate(context.Background()); invalidateErr != nil {
				return chatgptweb.Result{}, "", "", invalidateErr
			}
			routed := metrics.Phase(ctx, "conversation_route")
			route, err = s.conversations.Route(ctx, body.Model, body.BrowserID, messages, s.direct)
			routed()
			if err != nil {
				return chatgptweb.Result{}, "", "", err
			}
			input = route.Input
			defer route.Abort(context.Background())
		} else if missing && input.ConversationID != "" {
			input.ConversationID = ""
			input.ParentMessageID = ""
			input.IncludeHistory = true
		} else {
			break
		}
		result, err = s.direct.Generate(ctx, input)
	}
	if err != nil {
		return chatgptweb.Result{}, "", "", err
	}
	if route.finish != nil {
		finished := metrics.Phase(ctx, "conversation_save")
		err := route.Finish(context.Background(), result)
		finished()
		if err != nil {
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

func conversationNotFound(err error) bool {
	var upstream *chatgptdirect.UpstreamError
	if !errors.As(err, &upstream) || upstream.Status != http.StatusNotFound {
		return false
	}
	body := strings.ToLower(upstream.Body)
	return strings.Contains(body, "conversation_not_found") || strings.Contains(body, "conversation_deleted")
}

func conversationTooLarge(err error) bool {
	var upstream *chatgptdirect.UpstreamError
	return errors.As(err, &upstream) && upstream.Status == http.StatusRequestEntityTooLarge &&
		strings.Contains(upstream.Body, "conversation_too_large")
}

func pooledBrowserUnavailable(err error) bool {
	var bridge *chatgptdirect.BridgeError
	if !errors.As(err, &bridge) || (bridge.Status != http.StatusServiceUnavailable && bridge.Status != http.StatusGatewayTimeout) {
		return false
	}
	message := strings.ToLower(bridge.Message)
	return strings.Contains(message, "browser") &&
		(strings.Contains(message, "not ready") || strings.Contains(message, "not connected"))
}

func addToolReminder(messages []chatgptdirect.Message, original []chatMessage, plan toolPlan) []chatgptdirect.Message {
	if !plan.enabled() || len(original) == 0 || !strings.EqualFold(original[len(original)-1].Role, "user") {
		return messages
	}
	reminder := plan.reminder()
	if reminder == "" {
		return messages
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			messages[index].Content += reminder
			return messages
		}
	}
	return messages
}

func hasToolTranscript(messages []chatMessage) bool {
	for _, message := range messages {
		if strings.EqualFold(message.Role, "tool") || len(message.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

func latestUserMessage(messages []chatMessage) (string, error) {
	for index := len(messages) - 1; index >= 0; index-- {
		if !strings.EqualFold(messages[index].Role, "user") {
			continue
		}
		content, err := messageText(messages[index])
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(content) != "" {
			return content, nil
		}
	}
	return "", fmt.Errorf("a non-empty user message is required for ChatGPT tool repair")
}

func directMessages(messages []chatMessage, plan toolPlan) ([]chatgptdirect.Message, error) {
	result := make([]chatgptdirect.Message, 0, len(messages)+1)
	instruction := plan.instruction()
	for index, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "system" && role != "developer" && role != "user" && role != "assistant" && role != "tool" {
			return nil, fmt.Errorf("unsupported message role: %s", role)
		}
		if instruction != "" && role != "system" && role != "developer" {
			result = append(result, chatgptdirect.Message{Role: "system", Content: instruction})
			instruction = ""
		}
		text, err := messageText(message)
		if err != nil {
			return nil, err
		}
		if role == "assistant" && len(message.ToolCalls) > 0 {
			calls := toolCallsPrompt(message.ToolCalls)
			if calls == "" {
				return nil, fmt.Errorf("assistant tool_calls must use JSON object arguments")
			}
			text = strings.TrimSpace(text + "\n" + calls)
		}
		if role == "tool" {
			if message.ToolCallID == "" {
				return nil, fmt.Errorf("tool_call_id is required for tool messages")
			}
			role = "user"
			name := toolResultName(messages, index)
			if name == "" {
				name = message.Name
			}
			text = "Tool result" + optionalToolCallID(message.ToolCallID) + optionalToolName(name) + ":\n" + text
		}
		if role == "user" && len(result) > 0 && result[len(result)-1].Role == "user" && strings.HasPrefix(text, "Tool result") {
			result[len(result)-1].Content += "\n\n" + text
			continue
		}
		result = append(result, chatgptdirect.Message{Role: role, Content: text})
	}
	if instruction != "" {
		result = append(result, chatgptdirect.Message{Role: "system", Content: instruction})
	}
	return result, nil
}

func optionalToolCallID(id string) string {
	if id == "" {
		return ""
	}
	return " for " + id
}

func optionalToolName(name string) string {
	if name == "" {
		return ""
	}
	return " (" + name + ")"
}

func messageText(message chatMessage) (string, error) {
	if len(message.Content) == 0 || string(message.Content) == "null" {
		return "", nil
	}
	return chatContentText(message.Content)
}

func renderChatPrompt(messages []chatMessage, plan toolPlan) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("messages is required")
	}
	translated, err := directMessages(messages, plan)
	if err != nil {
		return "", err
	}
	blocks := make([]string, 0, len(translated))
	for _, message := range translated {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		blocks = append(blocks, strings.ToUpper(role[:1])+role[1:]+":\n"+message.Content)
	}
	return strings.Join(blocks, "\n\n"), nil
}

func (s *Server) openAIModels(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	video := videoAPIModel()
	models := []any{openAIModel("chatgpt-web", "browser-ui"), video}
	for _, name := range chatgptdirect.ModelNames() {
		models = append(models, openAIModel(name, "go-direct"))
	}
	if request.URL.Path == "/v1/models" {
		writeJSON(writer, http.StatusOK, map[string]any{"object": "list", "data": models})
		return
	}
	name := strings.TrimPrefix(request.URL.Path, "/v1/models/")
	if name == video["id"] {
		writeJSON(writer, http.StatusOK, video)
		return
	}
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
		"usage_estimated":   true,
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
		strings.Contains(err.Error(), "unsupported message role") ||
		strings.Contains(err.Error(), "tool_call_id is required") ||
		strings.Contains(err.Error(), "tool_calls must use JSON object arguments") {
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
