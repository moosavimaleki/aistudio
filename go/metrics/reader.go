package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Window struct {
	Minutes   []map[string]float64 `json:"minutes"`
	Aggregate map[string]float64   `json:"aggregate"`
	Gauges    map[string]string    `json:"gauges"`
	Events    []map[string]any     `json:"events"`
	Inflight  int64                `json:"inflight"`
}

func (s *Store) Read(ctx context.Context, minutes int) Window {
	if minutes < 1 {
		minutes = 1
	}
	maximum := int(s.Retention / time.Minute)
	if minutes > maximum {
		minutes = maximum
	}
	current := time.Now().Unix() / 60
	pipe := s.Redis.Pipeline()
	hashes := make([]*redis.MapStringStringCmd, 0, minutes)
	for minute := current - int64(minutes) + 1; minute <= current; minute++ {
		hashes = append(hashes, pipe.HGetAll(ctx, fmt.Sprintf("%s:minute:%d", s.Prefix, minute)))
	}
	gauges := pipe.HGetAll(ctx, s.Prefix+":gauges")
	events := pipe.LRange(ctx, s.Prefix+":events", 0, int64(s.EventLimit-1))
	_, _ = pipe.Exec(ctx)
	window := Window{Minutes: make([]map[string]float64, 0, minutes), Aggregate: map[string]float64{}, Gauges: gauges.Val(), Inflight: s.Inflight(ctx)}
	for _, hash := range hashes {
		decoded := map[string]float64{}
		values, _ := hash.Result()
		for name, value := range values {
			amount, _ := strconv.ParseFloat(value, 64)
			decoded[name] = amount
			window.Aggregate[name] += amount
		}
		window.Minutes = append(window.Minutes, decoded)
	}
	for _, raw := range events.Val() {
		var event map[string]any
		if json.Unmarshal([]byte(raw), &event) == nil {
			window.Events = append(window.Events, event)
		}
	}
	return window
}
func Matching(values map[string]float64, name string, labels map[string]string) float64 {
	total := 0.0
	for field, value := range values {
		metric, dimensions := ParseField(field)
		if metric != name {
			continue
		}
		matches := true
		for key, expected := range labels {
			if dimensions[key] != expected {
				matches = false
				break
			}
		}
		if matches {
			total += value
		}
	}
	return total
}
