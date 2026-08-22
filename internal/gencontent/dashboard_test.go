package gencontent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hamed/aistudio-api/internal/metrics"
	"github.com/redis/go-redis/v9"
)

func TestDashboardRoutesExposeSessionMetadata(t *testing.T) {
	browser := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"backend":"container-extension","connected":true,"browsers":[{"browserId":"default","authUser":"0","ready":true,"sessionState":"READY"}]}`))
	}))
	defer browser.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 10 * time.Millisecond, ReadTimeout: 10 * time.Millisecond, WriteTimeout: 10 * time.Millisecond})
	defer redisClient.Close()
	store := &metrics.Store{Redis: redisClient, Prefix: "test", Retention: time.Hour, EventRetention: time.Hour, EventLimit: 10}
	pool := &Pool{redis: redisClient, max: 3, namespace: "test-tabs"}
	server := NewServer(nil, pool, NewDashboard(store, pool, browser.URL)).Handler()

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("dashboard page status = %d", page.Code)
	}

	data := httptest.NewRecorder()
	server.ServeHTTP(data, httptest.NewRequest(http.MethodGet, "/dashboard/data?window=60", nil))
	if data.Code != http.StatusOK {
		t.Fatalf("dashboard data status = %d", data.Code)
	}
	if !strings.Contains(data.Body.String(), `"browserId":"default"`) {
		t.Fatalf("dashboard response does not include browser session: %s", data.Body.String())
	}
	if !strings.Contains(data.Body.String(), `"browserError":"browser health returned HTTP 503"`) {
		t.Fatalf("dashboard response does not include browser health error: %s", data.Body.String())
	}
}
