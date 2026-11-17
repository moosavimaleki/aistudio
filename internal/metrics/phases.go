package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type phaseKey struct{}
type phaseTrace struct {
	mu      sync.Mutex
	id      string
	started time.Time
	values  map[string]float64
}

func WithPhases(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, phaseKey{}, &phaseTrace{id: id, started: time.Now(), values: map[string]float64{}})
}

// Phase records cumulative milliseconds, including failed attempts. Parent and
// child phases may overlap; they must not be added together as a request total.
func Phase(ctx context.Context, name string) func() {
	started := time.Now()
	return func() { AddPhase(ctx, name, float64(time.Since(started).Microseconds())/1000) }
}

func AddPhase(ctx context.Context, name string, milliseconds float64) {
	trace, _ := ctx.Value(phaseKey{}).(*phaseTrace)
	if trace == nil || !validPhaseName(name) || milliseconds < 0 || math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) {
		return
	}
	trace.mu.Lock()
	trace.values[name] += milliseconds
	trace.mu.Unlock()
}

func Phases(ctx context.Context) map[string]float64 {
	trace, _ := ctx.Value(phaseKey{}).(*phaseTrace)
	if trace == nil {
		return nil
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	values := make(map[string]float64, len(trace.values)+1)
	for name, value := range trace.values {
		values[name] = value
	}
	values["total"] = float64(time.Since(trace.started).Microseconds()) / 1000
	return values
}

func RequestID(ctx context.Context) string {
	if trace, _ := ctx.Value(phaseKey{}).(*phaseTrace); trace != nil {
		return trace.id
	}
	return ""
}

// Internal service timings are propagated on failures as well as successes.
func ImportPhases(ctx context.Context, prefix, encoded string) {
	var values map[string]float64
	if len(encoded) > 16384 || json.Unmarshal([]byte(encoded), &values) != nil {
		return
	}
	for name, value := range values {
		AddPhase(ctx, prefix+name, value)
	}
}

func serverTiming(values map[string]float64) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s;dur=%.3f", name, values[name]))
	}
	return strings.Join(parts, ", ")
}

func validPhaseName(name string) bool {
	if name == "" || len(name) > 80 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
