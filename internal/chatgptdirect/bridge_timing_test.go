package chatgptdirect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hamed/aistudio-api/internal/metrics"
)

func TestBridgeImportsTimingsOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Lab-Request-ID") != "request-fixture" {
			t.Error("request identity was not propagated")
		}
		w.Header().Set("X-Lab-Timings", `{"page_ready":42,"total":45}`)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"fixture page unavailable"}`))
	}))
	defer server.Close()
	ctx := metrics.WithPhases(context.Background(), "request-fixture")
	_, err := newBridgeClient(server.URL).prepare(ctx, prepareRequest{})
	if err == nil || metrics.Phases(ctx)["browser_page_ready"] != 42 {
		t.Fatalf("failure timing lost: %v %v", err, metrics.Phases(ctx))
	}
}
