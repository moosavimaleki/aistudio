package gencontent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/hamed/aistudio-api/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type dashboardChatFixture struct{}

func (dashboardChatFixture) Generate(ctx context.Context, input chatgptdirect.Input) (chatgptdirect.Result, error) {
	metrics.AddPhase(ctx, "black_box_evaluate", 1500)
	return chatgptdirect.Result{Text: "fixture reply", Model: input.Model, ConversationID: "fixture-conversation", ParentMessageID: "fixture-parent"}, nil
}

func TestDirectDashboardRecordsChatWithoutBrowserRequests(t *testing.T) {
	endpoint := os.Getenv("METRICS_TEST_REDIS_URL")
	if endpoint == "" {
		t.Skip("set METRICS_TEST_REDIS_URL to run the isolated Redis integration test")
	}
	options, err := redis.ParseURL(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	store := &metrics.Store{Redis: client, Prefix: fmt.Sprintf("dashboard-test-%s-%d", t.Name(), time.Now().UnixNano()), Retention: time.Hour, EventRetention: time.Hour, EventLimit: 20}
	t.Setenv("CHATGPT_MODE", "direct")
	t.Setenv("CHATGPT_SESSION_FILE", "")
	cookies := t.TempDir()
	t.Setenv("CHATGPT_COOKIE_DIR", cookies)
	if err := os.WriteFile(filepath.Join(cookies, "fixture.txt"), []byte("fixture-private-cookie"), 0600); err != nil {
		t.Fatal(err)
	}
	var browserCalls atomic.Int64
	browser := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { browserCalls.Add(1) }))
	defer browser.Close()
	dashboard := NewDashboard(store, nil, browser.URL)
	server := &Server{dashboard: dashboard, direct: dashboardChatFixture{}, chatMode: "direct"}
	handler := metrics.HTTP(store, func(path string) bool { return strings.HasPrefix(path, "/dashboard") || path == "/" }, server.Handler())
	for range 3 {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"chatgpt/gpt-6","messages":[{"role":"user","content":"fixture"}]}`))
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		if writer.Code != 200 || writer.Header().Get("X-Lab-Timings") == "" || !strings.Contains(writer.Body.String(), "timings_ms") {
			t.Fatalf("chat timing missing: status=%d body=%s", writer.Code, writer.Body.String())
		}
	}
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/dashboard/data?window=15", nil))
	var data dashboardResponse
	if err := json.Unmarshal(writer.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Summary.Requests != 3 || data.Summary.Success != 3 || len(data.Models) != 1 || data.Models[0].Model != "chatgpt/gpt-6" {
		t.Fatalf("chat counters missing: %+v models=%+v", data.Summary, data.Models)
	}
	if len(data.Timings) != 1 || data.Timings[0].Average != 1500 || data.Timings[0].Count != 3 {
		t.Fatalf("phase timing not persisted: %+v", data.Timings)
	}
	requests := 0
	for _, point := range data.Series {
		requests += point.Requests
	}
	if requests != 3 || browserCalls.Load() != 0 || len(data.Profiles) != 0 || data.Direct.State != "CONFIGURED" {
		t.Fatalf("direct dashboard state invalid: requests=%d browserCalls=%d", requests, browserCalls.Load())
	}
	if *data.LatencyBuckets[0].UpperMS != 1000 || data.LatencyBuckets[0].Count != 3 {
		t.Fatalf("first bucket is not one second: %+v", data.LatencyBuckets)
	}
	if strings.Contains(writer.Body.String(), "fixture-private-cookie") {
		t.Fatal("credential leaked into dashboard")
	}
}
