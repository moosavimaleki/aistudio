package gencontent

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
	"github.com/hamed/aistudio-api/internal/metrics"
)

type videoGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	BrowserID      string `json:"browser_id"`
	ResponseFormat string `json:"response_format"`
	N              int    `json:"n"`
}

func videoAPIModel() map[string]any {
	model := openAIModel(strings.TrimPrefix(aistudio.VideoModel, "models/"), "go-direct")
	model["supported_endpoints"] = []string{"/v1/videos/generations"}
	return model
}

func (s *Server) videoGenerations(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body videoGenerationRequest
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if !decodeJSON(writer, request, &body) {
		return
	}
	if body.Model == "" {
		body.Model = aistudio.VideoModel
	}
	input := aistudio.VideoInput{Model: normalizeModel(body.Model), Prompt: body.Prompt}
	if _, err := aistudio.BuildVideoPayload(input); err != nil {
		openAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	if body.N < 0 || body.N > 1 {
		openAIError(writer, http.StatusBadRequest, "only one video per request is supported", "invalid_request_error")
		return
	}
	if body.ResponseFormat == "" {
		body.ResponseFormat = "url"
	}
	if body.ResponseFormat != "url" && body.ResponseFormat != "b64_json" {
		openAIError(writer, http.StatusBadRequest, "response_format must be url or b64_json", "invalid_request_error")
		return
	}
	metrics.SetLabel(request.Context(), "model", input.Model)
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Minute)
	defer cancel()
	outcome, err := s.video.GenerateVideo(ctx, input, body.BrowserID)
	if err != nil {
		clientError(writer, err)
		return
	}
	if !aistudio.IsMP4(outcome.Result.Data) {
		openAIError(writer, http.StatusBadGateway, "AI Studio returned no valid MP4", "upstream_error")
		return
	}
	item := map[string]any{"mime_type": "video/mp4", "size_bytes": len(outcome.Result.Data)}
	if body.ResponseFormat == "b64_json" {
		item["b64_json"] = base64.StdEncoding.EncodeToString(outcome.Result.Data)
	} else {
		id, err := s.videoFiles.save(outcome.Result.Data)
		if err != nil {
			openAIError(writer, http.StatusInsufficientStorage, err.Error(), "storage_error")
			return
		}
		item["id"] = id
		item["url"] = "/v1/videos/" + id + "/content"
		item["expires_at"] = time.Now().Add(videoRetention).Unix()
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"created": time.Now().Unix(), "model": strings.TrimPrefix(input.Model, "models/"),
		"data": []any{item}, "usage": outcome.Result.Usage,
		"lab_metadata": map[string]any{
			"browser_id": outcome.BrowserID, "interaction_id": outcome.Result.InteractionID,
			"transport": "go-direct", "upstream_method": "CreateInteractionStream",
		},
	})
}

func (s *Server) videoContent(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/v1/videos/")
	if !strings.HasSuffix(id, "/content") {
		http.NotFound(writer, request)
		return
	}
	id = strings.TrimSuffix(id, "/content")
	file, err := s.videoFiles.open(id)
	if err != nil {
		openAIError(writer, http.StatusNotFound, "video not found or expired", "not_found_error")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		openAIError(writer, http.StatusInternalServerError, "cannot read video", "storage_error")
		return
	}
	writer.Header().Set("Content-Type", "video/mp4")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.mp4"`, id))
	writer.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(writer, request, id+".mp4", info.ModTime(), file)
}
