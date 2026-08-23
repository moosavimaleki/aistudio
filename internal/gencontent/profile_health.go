package gencontent

import (
	"sync"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

const profileFailureCooldown = 30 * time.Second

// profileFailures gives a failed session a short recovery cooldown. Successful
// browser recovery clears the failure immediately.
type profileFailures struct {
	mu    sync.RWMutex
	items map[string]profileFailure
}

type profileFailure struct {
	At     time.Time `json:"at"`
	Phase  string    `json:"phase"`
	Status int       `json:"status"`
}

func newProfileFailures() *profileFailures {
	return &profileFailures{items: map[string]profileFailure{}}
}

func (f *profileFailures) Mark(profile BrowserProfile, err error) {
	client, ok := err.(*aistudio.ClientError)
	if !ok {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[profile.ID] = profileFailure{At: time.Now().UTC(), Phase: client.Phase, Status: client.Status}
}

func (f *profileFailures) Has(browserID string) bool {
	return f.RetryAfter(browserID) > 0
}

func (f *profileFailures) RetryAfter(browserID string) time.Duration {
	f.mu.RLock()
	defer f.mu.RUnlock()
	failure, found := f.items[browserID]
	if !found {
		return 0
	}
	remaining := time.Until(failure.At.Add(profileFailureCooldown))
	if remaining <= 0 {
		return 0
	}
	return remaining
}

func (f *profileFailures) Status(browserID string) (profileFailure, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	value, found := f.items[browserID]
	return value, found
}

func (f *profileFailures) Clear(browserID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, browserID)
}
