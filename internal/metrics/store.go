package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var latencyBuckets = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

type Store struct {
	Redis                     *redis.Client
	Prefix                    string
	Retention, EventRetention time.Duration
	EventLimit                int
}

func FromEnv() (*Store, error) {
	endpoint := os.Getenv("REDIS_URL")
	if endpoint == "" {
		endpoint = "redis://localhost:6379/0"
	}
	options, err := redis.ParseURL(endpoint)
	if err != nil {
		return nil, err
	}
	return &Store{Redis: redis.NewClient(options), Prefix: valueOr(os.Getenv("METRICS_NAMESPACE"), "lab:metrics:v1"), Retention: time.Duration(intEnv("METRICS_RETENTION_SECONDS", 172800)) * time.Second, EventRetention: time.Duration(intEnv("METRICS_EVENT_RETENTION_SECONDS", 604800)) * time.Second, EventLimit: intEnv("METRICS_EVENT_LIMIT", 200)}, nil
}
func (s *Store) Close() error { return s.Redis.Close() }
func (s *Store) Increment(ctx context.Context, name string, amount float64, labels map[string]any) {
	s.write(ctx, func(pipe redis.Pipeliner, key string) { pipe.HIncrByFloat(ctx, key, Field(name, labels), amount) })
}
func (s *Store) Timing(ctx context.Context, name string, elapsed float64, labels map[string]any) {
	s.write(ctx, func(pipe redis.Pipeliner, key string) {
		pipe.HIncrByFloat(ctx, key, Field(name+".count", labels), 1)
		pipe.HIncrByFloat(ctx, key, Field(name+".sum_ms", labels), elapsed)
		for _, boundary := range latencyBuckets {
			if elapsed <= boundary {
				pipe.HIncrByFloat(ctx, key, Field(fmt.Sprintf("%s.le_%g", name, boundary), labels), 1)
			}
		}
		pipe.HIncrByFloat(ctx, key, Field(name+".le_inf", labels), 1)
	})
}
func (s *Store) Gauge(ctx context.Context, name string, value any, labels map[string]any) {
	key := s.Prefix + ":gauges"
	pipe := s.Redis.Pipeline()
	pipe.HSet(ctx, key, Field(name, labels), fmt.Sprint(value))
	pipe.Expire(ctx, key, s.Retention)
	_, _ = pipe.Exec(ctx)
}
func (s *Store) Event(ctx context.Context, category, event string, fields map[string]any) {
	payload := map[string]any{"timestamp": time.Now().UnixMilli(), "category": category, "event": event}
	for key, value := range fields {
		if value != nil {
			payload[key] = value
		}
	}
	encoded, _ := json.Marshal(payload)
	key := s.Prefix + ":events"
	pipe := s.Redis.Pipeline()
	pipe.LPush(ctx, key, string(encoded))
	pipe.LTrim(ctx, key, 0, int64(s.EventLimit-1))
	pipe.Expire(ctx, key, s.EventRetention)
	_, _ = pipe.Exec(ctx)
}
func (s *Store) Begin(ctx context.Context, id string) {
	key := s.Prefix + ":inflight"
	pipe := s.Redis.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(time.Now().Add(5 * time.Minute).Unix()), Member: id})
	pipe.Expire(ctx, key, 10*time.Minute)
	_, _ = pipe.Exec(ctx)
}
func (s *Store) End(ctx context.Context, id string) {
	_ = s.Redis.ZRem(ctx, s.Prefix+":inflight", id).Err()
}
func (s *Store) Inflight(ctx context.Context) int64 {
	key := s.Prefix + ":inflight"
	pipe := s.Redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprint(time.Now().Unix()))
	count := pipe.ZCard(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0
	}
	return count.Val()
}
func (s *Store) write(ctx context.Context, action func(redis.Pipeliner, string)) {
	key := s.minuteKey(time.Now())
	pipe := s.Redis.Pipeline()
	action(pipe, key)
	pipe.Expire(ctx, key, s.Retention)
	_, _ = pipe.Exec(ctx)
}
func (s *Store) minuteKey(now time.Time) string {
	return fmt.Sprintf("%s:minute:%d", s.Prefix, now.Unix()/60)
}
func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func intEnv(name string, fallback int) int {
	var value int
	if _, err := fmt.Sscanf(os.Getenv(name), "%d", &value); err == nil && value > 0 {
		return value
	}
	return fallback
}
