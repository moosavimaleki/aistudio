package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestHTTPPersistsMetricsAfterRequestCancellation(t *testing.T) {
	endpoint := os.Getenv("METRICS_TEST_REDIS_URL")
	if endpoint == "" {
		t.Skip("set METRICS_TEST_REDIS_URL for the isolated Redis integration test")
	}
	options, err := redis.ParseURL(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	store := &Store{Redis: client, Prefix: fmt.Sprintf("metrics-test-%s-%d", t.Name(), time.Now().UnixNano()), Retention: time.Hour, EventRetention: time.Hour, EventLimit: 10}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	handler := HTTP(store, func(string) bool { return false }, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		SetLabel(request.Context(), "model", "chatgpt/gpt-6")
		AddPhase(request.Context(), "black_box_evaluate", 1250)
		cancel()
		writer.WriteHeader(http.StatusGatewayTimeout)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	window := store.Read(context.Background(), 1)
	if Matching(window.Aggregate, "http.response", map[string]string{"model": "chatgpt/gpt-6", "status": "504"}) != 1 {
		t.Fatal("canceled request was not recorded")
	}
	if Matching(window.Aggregate, "http.phase.black_box_evaluate.sum_ms", nil) != 1250 || window.Inflight != 0 {
		t.Fatal("phase timing or inflight cleanup was lost after cancellation")
	}
	store.Timing(context.Background(), "fixture", 1500, nil)
	window = store.Read(context.Background(), 1)
	if Matching(window.Aggregate, BucketMetric("fixture.histogram", 1000), nil) != 0 || Matching(window.Aggregate, BucketMetric("fixture.histogram", 2500), nil) != 1 {
		t.Fatal("histogram boundaries were not persisted correctly")
	}
}
