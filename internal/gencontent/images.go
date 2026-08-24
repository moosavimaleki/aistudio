package gencontent

import (
	"net/http"
	"strings"
	"time"
)

type imageGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	ResponseFormat string `json:"response_format"`
	BrowserID      string `json:"browser_id"`
}

func (s *Server) imageGenerations(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body imageGenerationRequest
	if !decodeJSON(writer, request, &body) {
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		openAIError(writer, http.StatusBadRequest, "prompt is required", "invalid_request_error")
		return
	}
	if body.N > 1 {
		openAIError(writer, http.StatusBadRequest, "only one image per request is supported", "invalid_request_error")
		return
	}
	if body.ResponseFormat != "" && body.ResponseFormat != "b64_json" {
		openAIError(writer, http.StatusBadRequest, "only response_format=b64_json is supported", "invalid_request_error")
		return
	}
	prompt := "Generate one image from this request. Return the image in the ChatGPT UI, not text only.\n\n" + body.Prompt
	result, err := s.chat.GenerateImage(request.Context(), prompt, body.BrowserID)
	if err != nil {
		openAIError(writer, chatBridgeStatus(err), err.Error(), "browser_bridge_error")
		return
	}
	data := make([]any, 0, len(result.Images))
	for _, image := range result.Images {
		if image.Data == "" {
			continue
		}
		data = append(data, map[string]any{"b64_json": image.Data})
	}
	if len(data) == 0 {
		openAIError(writer, http.StatusBadGateway, "ChatGPT returned no generated image", "browser_bridge_error")
		return
	}
	metadata := map[string]any{
		"browser_id":      result.BrowserID,
		"conversation_id": result.ConversationID,
		"requested_model": body.Model,
		"upstream_status": result.UpstreamStatus,
		"upstream_path":   result.UpstreamPath,
	}
	writeJSON(writer, http.StatusOK, map[string]any{"created": time.Now().Unix(), "data": data, "lab_metadata": metadata})
}
