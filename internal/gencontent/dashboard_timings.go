package gencontent

import (
	"sort"
	"strings"

	"github.com/hamed/aistudio-api/internal/metrics"
)

type dashboardTiming struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Average float64 `json:"averageMs"`
	P50     int     `json:"p50Ms"`
	P95     int     `json:"p95Ms"`
}

type dashboardLatencyBucket struct {
	Label   string   `json:"label,omitempty"`
	UpperMS *float64 `json:"upperMs"`
	Count   int      `json:"count"`
}

// A negative percentile denotes a lower bound when an older histogram cannot
// locate the value more precisely, or a response exceeds the largest bucket.
func metricPercentile(values map[string]float64, name string, ratio float64, match func(map[string]string) bool) int {
	count := metricTotal(values, name+".count", match)
	if count == 0 {
		return 0
	}
	legacyCount := max(0, count-metricTotal(values, name+".histogram.count", match))
	legacyKnown := metricTotal(values, metrics.BucketMetric(name, 60000), match)
	for _, boundary := range metrics.LatencyBucketsMS() {
		cumulative := metricTotal(values, metrics.BucketMetric(name+".histogram", boundary), match)
		if boundary <= 60000 {
			cumulative += metricTotal(values, metrics.BucketMetric(name, boundary), match)
		} else {
			if legacyCount > legacyKnown {
				return -60000
			}
			cumulative += legacyCount
		}
		if cumulative >= count*ratio {
			return int(boundary)
		}
	}
	return -300000
}

func dashboardHistogram(values map[string]float64) []dashboardLatencyBucket {
	rows := []dashboardLatencyBucket{}
	modern := metricTotal(values, "http.duration.histogram.count", generationDuration)
	legacy := max(0, metricTotal(values, "http.duration.count", generationDuration)-modern)
	legacyKnown := metricTotal(values, metrics.BucketMetric("http.duration", 60000), generationDuration)
	previous := 0
	for _, boundary := range metrics.LatencyBucketsMS() {
		cumulative := int(metricTotal(values, metrics.BucketMetric("http.duration.histogram", boundary), generationDuration))
		if boundary <= 60000 {
			cumulative += int(metricTotal(values, metrics.BucketMetric("http.duration", boundary), generationDuration))
		} else {
			cumulative += int(legacyKnown)
		}
		cumulative = max(previous, cumulative)
		rows = append(rows, dashboardLatencyBucket{UpperMS: &boundary, Count: cumulative - previous})
		previous = cumulative
	}
	totalKnown := int(modern + legacyKnown)
	rows = append(rows, dashboardLatencyBucket{Label: "> 300s", Count: max(0, totalKnown-previous)})
	if legacy > legacyKnown {
		rows = append(rows, dashboardLatencyBucket{Label: "دادهٔ قدیمی: > 60s", Count: int(legacy - legacyKnown)})
	}
	return rows
}

func dashboardTimings(values map[string]float64) []dashboardTiming {
	names := map[string]bool{}
	for field := range values {
		name, labels := metrics.ParseField(field)
		if generationDuration(labels) && strings.HasPrefix(name, "http.phase.") && strings.HasSuffix(name, ".count") && !strings.Contains(name, ".histogram.") {
			names[strings.TrimSuffix(name, ".count")] = true
		}
	}
	rows := []dashboardTiming{}
	for name := range names {
		count := metricTotal(values, name+".count", generationDuration)
		if count <= 0 {
			continue
		}
		rows = append(rows, dashboardTiming{
			Name: strings.TrimPrefix(name, "http.phase."), Count: int(count),
			Average: round(metricTotal(values, name+".sum_ms", generationDuration) / count),
			P50:     metricPercentile(values, name, 0.50, generationDuration),
			P95:     metricPercentile(values, name, 0.95, generationDuration),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
