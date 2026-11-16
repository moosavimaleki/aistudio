package chatgptdirect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestHistoryRetryHonorsServerDelayWithoutResending(t *testing.T) {
	var gets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("history retry must never submit a message")
		}
		if gets.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"current_node": "completed", "messages": []any{}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"url":"https://chatgpt.com","headers":{"cookie":"fixture=1","authorization":"Bearer fixture"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	httpClient := newTestHttpClient()
	client := &Client{http: httpClient, origin: server.URL, sentinel: &sentinelClient{http: httpClient, sessionFile: path}}
	state, err := client.conversation(context.Background(), "thread", "")
	if err != nil || state.CurrentNode != "completed" || gets.Load() != 2 {
		t.Fatal(state, err, gets.Load())
	}
}

func TestHistoryDelayBounds(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		header string
		want   time.Duration
	}{
		{header: "2", want: 3 * time.Second}, {header: "", want: 0}, {header: "61", want: 0},
		{header: "-1", want: 0}, {header: now.Add(4 * time.Second).Format(http.TimeFormat), want: 5 * time.Second},
	} {
		if got := historyRetryDelay(tc.header, now); got != tc.want {
			t.Errorf("%q got %v want %v", tc.header, got, tc.want)
		}
	}
}
