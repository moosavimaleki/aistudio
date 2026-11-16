// Package directapi serves sequential conversations through a native Go client.
package directapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
)

// Generator sends exactly one upstream turn without automatic resubmission.
type Generator interface {
	Generate(context.Context, chatgptdirect.Input) (chatgptdirect.Result, error)
}

// Handler exposes POST /chat and GET /health on a loopback HTTP server.
type Handler struct {
	client Generator
	model  string
	gate   chan struct{}
}

// NewHandler creates a serialized HTTP endpoint for one authenticated account.
func NewHandler(client Generator, model string) http.Handler {
	return &Handler{client: client, model: model, gate: make(chan struct{}, 1)}
}

type chatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	if (host != "localhost" && host != "127.0.0.1") || r.Header.Get("Origin") != "" {
		respond(w, http.StatusForbidden, map[string]string{"error": "local requests only"})
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/health" {
		respond(w, http.StatusOK, map[string]any{"ok": true, "transport": "direct"})
		return
	}
	if r.URL.Path != "/chat" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respond(w, 405, map[string]string{"error": "use POST"})
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		respond(w, 415, map[string]string{"error": "Content-Type must be application/json"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	var input chatRequest
	if err := decoder.Decode(&input); err != nil {
		respond(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		respond(w, 400, map[string]string{"error": "only one JSON object is accepted"})
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		respond(w, 400, map[string]string{"error": "message must not be empty"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	select {
	case h.gate <- struct{}{}:
		defer func() { <-h.gate }()
	case <-ctx.Done():
		respond(w, 504, map[string]string{"error": "request canceled before sending"})
		return
	}
	result, err := h.client.Generate(ctx, chatgptdirect.Input{
		Model: h.model, ConversationID: input.ConversationID,
		Messages: []chatgptdirect.Message{{Role: "user", Content: input.Message}},
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		respond(w, status, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, map[string]string{
		"conversation_id": result.ConversationID, "reply": result.Text,
		"parent_message_id": result.ParentMessageID,
	})
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// A disconnected caller cannot consume the response. Never retry its send.
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}
