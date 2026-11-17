package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func HTTP(store *Store, excluded func(string) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if excluded(request.URL.Path) {
			next.ServeHTTP(writer, request)
			return
		}
		id := fmt.Sprintf("request-%d", time.Now().UnixNano())
		if parent := request.Header.Get("X-Lab-Request-ID"); validPhaseName(parent) {
			id = parent
		}
		request = request.WithContext(WithPhases(request.Context(), id))
		labels := requestLabels(request.URL.Path, request.Method)
		request = request.WithContext(context.WithValue(request.Context(), requestLabelsKey{}, labels))
		store.Begin(request.Context(), id)
		started := time.Now()
		observer := &responseWriter{ResponseWriter: writer, status: http.StatusOK, ctx: request.Context()}
		defer func() {
			elapsed := float64(time.Since(started).Microseconds()) / 1000
			completion, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 2*time.Second)
			defer cancel()
			phases := Phases(request.Context())
			fields := merge(labels, map[string]any{"request_id": id, "status": observer.status, "timings_ms": phases})
			encoded, _ := json.Marshal(fields)
			log.Printf("request_timing %s", encoded)
			store.Event(completion, "http", "request-timing", fields)
			store.End(completion, id)
			store.Increment(completion, "http.response", 1, merge(labels, map[string]any{"status": observer.status}))
			store.Timing(completion, "http.duration", elapsed, labels)
			for name, elapsed := range phases {
				if name != "total" {
					store.Timing(completion, "http.phase."+name, elapsed, labels)
				}
			}
			if observer.status >= 400 {
				store.Event(completion, "http", "request-error", merge(labels, map[string]any{"status": observer.status}))
			}
		}()
		next.ServeHTTP(observer, request)
	})
}

type requestLabelsKey struct{}

func SetLabel(ctx context.Context, name string, value any) {
	if labels, ok := ctx.Value(requestLabelsKey{}).(map[string]any); ok {
		labels[name] = value
	}
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	ctx         context.Context
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if w.ctx != nil {
		values := Phases(w.ctx)
		encoded, _ := json.Marshal(values)
		w.Header().Set("X-Lab-Request-ID", RequestID(w.ctx))
		w.Header().Set("X-Lab-Timings", string(encoded))
		w.Header().Set("Server-Timing", serverTiming(values))
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func requestLabels(path, method string) map[string]any {
	labels := map[string]any{"route": routeName(path), "method": method}
	if _, suffix, found := strings.Cut(path, "/models/"); found {
		model := strings.SplitN(suffix, ":", 2)[0]
		if decoded, err := url.PathUnescape(model); err == nil {
			labels["model"] = decoded
		}
	}
	return labels
}
func routeName(path string) string {
	switch {
	case strings.HasSuffix(path, ":streamGenerateContent"):
		return "streamGenerateContent"
	case strings.HasSuffix(path, ":generateContent"):
		return "generateContent"
	case path == "/generate-content":
		return "legacyGenerateContent"
	case path == "/get-token":
		return "getToken"
	case path == "/bootstrap":
		return "bootstrap"
	}
	return strings.Trim(path, "/")
}
func merge(left, right map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range left {
		result[key] = value
	}
	for key, value := range right {
		result[key] = value
	}
	return result
}
