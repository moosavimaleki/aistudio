package browserinterface

import (
	"context"
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"sync"
	"time"
)

type Job struct {
	ID           string `json:"id"`
	BrowserID    string
	Payload      map[string]any
	result       chan jobResult
	dispatchedAt time.Time
}
type jobResult struct {
	value map[string]any
	err   error
}
type Broker struct {
	mu                  sync.Mutex
	jobs                map[string]*Job
	heartbeats          map[string]time.Time
	timeout, redispatch time.Duration
}

func NewBroker() *Broker {
	return &Broker{jobs: map[string]*Job{}, heartbeats: map[string]time.Time{}, timeout: 60 * time.Second, redispatch: 10 * time.Second}
}
func (b *Broker) Request(payload map[string]any, browserID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
	defer cancel()
	return b.RequestContext(ctx, payload, browserID)
}

func (b *Broker) RequestContext(ctx context.Context, payload map[string]any, browserID string) (map[string]any, error) {
	return b.RequestContextWithID(ctx, fmt.Sprintf("job-%d", time.Now().UnixNano()), payload, browserID)
}

func (b *Broker) RequestContextWithID(ctx context.Context, jobID string, payload map[string]any, browserID string) (map[string]any, error) {
	job := &Job{ID: jobID, BrowserID: browserID, Payload: payload, result: make(chan jobResult, 1)}
	b.mu.Lock()
	b.jobs[job.ID] = job
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.jobs, job.ID); b.mu.Unlock() }()
	select {
	case result := <-job.result:
		return result.value, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("Container extension job did not finish: %w", ctx.Err())
	}
}
func (b *Broker) Next(browserID string) *Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.heartbeats[browserID] = time.Now()
	for _, job := range b.jobs {
		if job.BrowserID == browserID && (job.dispatchedAt.IsZero() || time.Since(job.dispatchedAt) >= b.redispatch) {
			job.dispatchedAt = time.Now()
			return &Job{ID: job.ID, Payload: job.Payload}
		}
	}
	return nil
}
func (b *Broker) Complete(id, browserID string, result map[string]any) bool {
	b.mu.Lock()
	job := b.jobs[id]
	if job == nil || job.BrowserID != browserID {
		b.mu.Unlock()
		return false
	}
	delete(b.jobs, id)
	b.mu.Unlock()
	if message, _ := result["error"].(string); message != "" {
		job.result <- jobResult{err: fmt.Errorf("%s", message)}
	} else {
		job.result <- jobResult{value: result}
	}
	return true
}
func (b *Broker) Health(browserID string) map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	last := b.heartbeats[browserID]
	pending := 0
	for _, job := range b.jobs {
		if job.BrowserID == browserID {
			pending++
		}
	}
	var age any
	if !last.IsZero() {
		age = float64(time.Since(last).Milliseconds()) / 1000
	}
	return map[string]any{"connected": !last.IsZero() && time.Since(last) < 5*time.Second, "pendingJobs": pending, "heartbeatAgeSeconds": age}
}

var _ = aistudio.RuntimeConfig{}
