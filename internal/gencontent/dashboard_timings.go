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
	UpperMS *float64 `json:"upperMs"`
	Count   int      `json:"count"`
}

func metricPercentile(values map[string]float64, name string, ratio float64, match func(map[string]string) bool) int {
	count := metricTotal(values, name+".count", match)
	if count == 0 {
		return 0
	}
	boundaries := metrics.LatencyBucketsMS()
	for _, boundary := range boundaries {
		if metricTotal(values, metrics.BucketMetric(name, boundary), match) >= count*ratio {
			return int(boundary)
		}
	}
	return int(boundaries[len(boundaries)-1])
}

func dashboardHistogram(values map[string]float64) []dashboardLatencyBucket {
	rows := []dashboardLatencyBucket{}
	previous := 0
	for _, boundary := range metrics.LatencyBucketsMS() {
		cumulative := int(metricTotal(values, metrics.BucketMetric("http.duration", boundary), generationDuration))
		cumulative = max(previous, cumulative)
		rows = append(rows, dashboardLatencyBucket{UpperMS: &boundary, Count: cumulative - previous})
		previous = cumulative
	}
	total := int(metricTotal(values, "http.duration.count", generationDuration))
	return append(rows, dashboardLatencyBucket{Count: max(0, total-previous)})
}

func dashboardTimings(values map[string]float64) []dashboardTiming {
	names := map[string]bool{}
	for field := range values {
		name, labels := metrics.ParseField(field)
		if generationDuration(labels) && strings.HasPrefix(name, "http.phase.") && strings.HasSuffix(name, ".count") {
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
