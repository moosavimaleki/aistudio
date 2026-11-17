package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestPhasesAggregateConcurrentAttemptsAndImportedFailures(t *testing.T) {
	ctx := WithPhases(context.Background(), "request-test")
	var group sync.WaitGroup
	for range 20 {
		group.Add(1)
		go func() { defer group.Done(); AddPhase(ctx, "queue", 2) }()
	}
	group.Wait()
	ImportPhases(ctx, "browser_", `{"page_ready":12,"total":15,"bad name":1,"negative":-1}`)
	values := Phases(ctx)
	if values["queue"] != 40 || values["browser_page_ready"] != 12 || values["browser_total"] != 15 {
		t.Fatalf("lost timing across retry/service boundary: %v", values)
	}
	if _, ok := values["browser_bad name"]; ok {
		t.Fatal("unsafe timing name accepted")
	}
	values["queue"] = 0
	if Phases(ctx)["queue"] != 40 {
		t.Fatal("snapshot must not expose mutable trace state")
	}
}

func TestTimingHeadersOnFailureAndImplicitWrite(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		ctx := WithPhases(context.Background(), "request-fixture")
		AddPhase(ctx, "page_ready", 25)
		recorder := httptest.NewRecorder()
		writer := &responseWriter{ResponseWriter: recorder, status: http.StatusOK, ctx: ctx}
		if status != http.StatusOK {
			writer.WriteHeader(status)
		}
		writer.Write([]byte(`{"fixture":true}`))
		writer.Flush()
		writer.WriteHeader(http.StatusTeapot)
		if recorder.Code != status || recorder.Header().Get("X-Lab-Request-ID") != "request-fixture" {
			t.Fatalf("lost status/request identity: %d %v", recorder.Code, recorder.Header())
		}
		var values map[string]float64
		if json.Unmarshal([]byte(recorder.Header().Get("X-Lab-Timings")), &values) != nil || values["page_ready"] != 25 {
			t.Fatal("missing error/success diagnostic timings")
		}
		if !strings.Contains(recorder.Header().Get("Server-Timing"), "page_ready;dur=25.000") || !recorder.Flushed {
			t.Fatal("Server-Timing or streaming flush lost")
		}
	}
}

func TestPhasesAreOptional(t *testing.T) {
	ctx := context.Background()
	Phase(ctx, "unused")()
	ImportPhases(ctx, "ignored_", `{"fixture":10}`)
	if Phases(ctx) != nil || RequestID(ctx) != "" {
		t.Fatal("untraced requests must remain supported")
	}
}
