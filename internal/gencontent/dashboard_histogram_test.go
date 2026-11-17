package gencontent

import (
	"testing"

	"github.com/hamed/aistudio-api/internal/metrics"
)

func TestDashboardReadsLegacyAndCurrentHistograms(t *testing.T) {
	labels := map[string]any{"route": "v1/chat/completions", "model": "chatgpt/gpt-6"}
	values := map[string]float64{}
	add := func(name string, value float64) { values[metrics.Field(name, labels)] = value }
	add("http.duration.count", 3)
	add("http.duration.le_60000", 1)
	add("http.duration.histogram.count", 1)
	for _, boundary := range metrics.LatencyBucketsMS() {
		if boundary >= 30000 {
			add(metrics.BucketMetric("http.duration.histogram", boundary), 1)
		}
	}
	if got := latencyPercentile(values, .50, generationDuration); got != 60000 {
		t.Fatalf("median = %d, want known 60-second boundary", got)
	}
	if got := latencyPercentile(values, .95, generationDuration); got != -60000 {
		t.Fatalf("p95 = %d, want >60s lower bound for legacy samples", got)
	}
	total, legacy := 0, 0
	for _, row := range dashboardHistogram(values) {
		total += row.Count
		if row.Label == "دادهٔ قدیمی: > 60s" {
			legacy += row.Count
		}
	}
	if total != 3 || legacy != 1 {
		t.Fatalf("histogram lost or misclassified legacy samples: total=%d legacy=%d", total, legacy)
	}
}
