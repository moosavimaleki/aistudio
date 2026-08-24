package chatgptdirect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBridgeReportsMissingArtifactNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"headers":{"oai-device-id":"test"}}`))
	}))
	defer server.Close()
	client := &bridgeClient{endpoint: server.URL, http: server.Client()}

	_, err := client.prepare(context.Background(), prepareRequest{Prompt: "test"})

	if err == nil || !strings.Contains(err.Error(), "prepare headers, cookies") {
		t.Fatalf("unexpected error: %v", err)
	}
}
