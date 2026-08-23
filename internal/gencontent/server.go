package gencontent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"net/http"
	"strings"
)

type Server struct {
	service   *Service
	pool      *Pool
	dashboard *Dashboard
}

func NewServer(service *Service, pool *Pool, dashboard *Dashboard) *Server {
	return &Server{service: service, pool: pool, dashboard: dashboard}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.dashboard.Register(mux)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/generate-content", s.legacy)
	mux.HandleFunc("/v1/projects/", s.vertex)
	return mux
}
func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	result, err := s.service.Health(request.Context())
	if err != nil {
		result["ok"] = false
		result["error"] = err.Error()
		writeJSON(writer, http.StatusServiceUnavailable, result)
		return
	}
	result["ok"] = true
	writeJSON(writer, http.StatusOK, result)
}
func (s *Server) legacy(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Model             string         `json:"model"`
		Prompt            string         `json:"prompt"`
		History           []any          `json:"history"`
		GenerationConfig  map[string]any `json:"generationConfig"`
		SafetySettings    any            `json:"safetySettings"`
		SystemInstruction any            `json:"systemInstruction"`
	}
	if !decodeJSON(writer, request, &body) {
		return
	}
	model := body.Model
	if model == "" {
		model = s.service.settings.Model
	}
	if model == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "model is required"})
		return
	}
	outcome, err := s.service.Generate(request.Context(), aistudio.GenerateInput{Model: normalizeModel(model), Prompt: body.Prompt, History: body.History, GenerationConfig: body.GenerationConfig, SafetySettings: body.SafetySettings, SystemInstruction: body.SystemInstruction})
	if err != nil {
		clientError(writer, err)
		return
	}
	result := outcome["result"].(aistudio.GenerateResult)
	writeJSON(writer, http.StatusOK, map[string]any{"state": "READY", "tabId": outcome["tabId"], "browserId": outcome["browserId"], "authUser": outcome["authUser"], "tabGenerateCount": outcome["generateCount"], "text": result.FinalText, "chunkCount": len(result.Chunks), "chunks": result.Chunks, "finishReason": result.FinishReason, "usage": result.Usage, "conversationMetadata": result.ConversationMetadata})
}
func (s *Server) vertex(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !strings.Contains(request.URL.Path, "publishers/google/models/") {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	prefix := strings.Split(request.URL.Path, "models/")
	if len(prefix) != 2 {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	target := prefix[1]
	stream := strings.HasSuffix(target, ":streamGenerateContent")
	model := strings.TrimSuffix(strings.TrimSuffix(target, ":streamGenerateContent"), ":generateContent")
	var body struct {
		Contents          []any          `json:"contents"`
		GenerationConfig  map[string]any `json:"generationConfig"`
		SafetySettings    any            `json:"safetySettings"`
		SystemInstruction any            `json:"systemInstruction"`
		Tools             []any          `json:"tools"`
		LabContext        struct {
			ContinuationToken any `json:"continuationToken"`
			ToolContext       any `json:"toolContext"`
		} `json:"labContext"`
	}
	if !decodeJSON(writer, request, &body) {
		return
	}
	if len(body.Contents) == 0 {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"error": "contents is required"})
		return
	}
	outcome, err := s.service.Generate(request.Context(), aistudio.GenerateInput{Model: normalizeModel(model), Contents: body.Contents, GenerationConfig: body.GenerationConfig, SafetySettings: body.SafetySettings, SystemInstruction: body.SystemInstruction, Tools: body.Tools, ContinuationToken: body.LabContext.ContinuationToken, ToolContext: body.LabContext.ToolContext})
	if err != nil {
		clientError(writer, err)
		return
	}
	response := vertexResponse(normalizeModel(model), outcome)
	if stream {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("X-Lab-Streaming-Mode", "buffered")
		fmt.Fprintf(writer, "data: %s\n\n", mustJSON(response))
		return
	}
	writeJSON(writer, http.StatusOK, response)
}
func vertexResponse(model string, outcome map[string]any) map[string]any {
	result := outcome["result"].(aistudio.GenerateResult)
	parts := result.ModelParts
	if len(parts) == 0 {
		parts = []any{map[string]any{"text": result.FinalText}}
	}
	candidate := map[string]any{"content": map[string]any{"role": "model", "parts": parts}}
	if result.FinishReason != nil {
		candidate["finishReason"] = result.FinishReason
	}
	metadata := map[string]any{"tabId": outcome["tabId"], "browserId": outcome["browserId"], "authUser": outcome["authUser"], "tabGenerateCount": outcome["generateCount"], "chunkCount": len(result.Chunks), "conversationMetadata": result.ConversationMetadata}
	response := map[string]any{"candidates": []any{candidate}, "modelVersion": model, "labMetadata": metadata}
	if result.Usage != nil {
		response["usageMetadata"] = result.Usage
	}
	return response
}
func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	defer request.Body.Close()
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return false
	}
	return true
}
func clientError(writer http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if client, ok := err.(*aistudio.ClientError); ok {
		if client.Status >= 400 && client.Status < 600 {
			status = client.Status
		}
		writeJSON(writer, status, client)
		return
	}
	if strings.Contains(err.Error(), "pool is full") {
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, map[string]any{"error": err.Error()})
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
func mustJSON(value any) string { data, _ := json.Marshal(value); return string(data) }

var _ = context.Background
