package metrics

import (
	"fmt"
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
		labels := requestLabels(request.URL.Path, request.Method)
		store.Begin(request.Context(), id)
		started := time.Now()
		observer := &responseWriter{ResponseWriter: writer, status: http.StatusOK}
		defer func() {
			store.End(request.Context(), id)
			store.Increment(request.Context(), "http.response", 1, merge(labels, map[string]any{"status": observer.status}))
			store.Timing(request.Context(), "http.duration", float64(time.Since(started).Microseconds())/1000, labels)
			if observer.status >= 400 {
				store.Event(request.Context(), "http", "request-error", merge(labels, map[string]any{"status": observer.status}))
			}
		}()
		next.ServeHTTP(observer, request)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
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
