package aistudio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestRetryRetriesTransientStatus(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.RequestRetry(
		context.Background(), http.MethodPost, server.URL, nil, []byte("{}"), 4,
	)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
