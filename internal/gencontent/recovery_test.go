package gencontent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

func TestResetProfileCallsBrowserRecoveryEndpoint(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/internal/browsers/browser2/reset" {
			t.Errorf("path = %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	service := &Service{
		recoveryURL:  server.URL + "/internal/browsers",
		recoveryHTTP: server.Client(),
	}
	if err := service.resetProfile(context.Background(), "browser2"); err != nil {
		t.Fatalf("reset profile: %v", err)
	}
	if !called {
		t.Fatal("browser recovery endpoint was not called")
	}
}

func TestResetProfileRejectsFailedRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "warm failed", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	service := &Service{
		recoveryURL:  server.URL + "/internal/browsers",
		recoveryHTTP: server.Client(),
	}
	if err := service.resetProfile(context.Background(), "default"); err == nil {
		t.Fatal("expected failed browser recovery to be returned")
	}
}

func TestRateLimitedBrowserIsEligibleForFailover(t *testing.T) {
	err := aistudio.ResponseError("RPC", http.StatusTooManyRequests, "quota exceeded")
	err.Diagnostics = map[string]any{"browserId": "default"}

	browserID, found := rateLimitedBrowser(err)
	if !found || browserID != "default" {
		t.Fatalf("rate-limited browser = (%q, %v)", browserID, found)
	}
}
