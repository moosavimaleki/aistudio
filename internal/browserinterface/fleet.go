package browserinterface

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Fleet struct {
	broker          *Broker
	config          Config
	sessions        map[string]*ChromeSession
	mu              sync.Mutex
	warmErrors      map[string]string
	chatGPTNext     int
	chatGPTActive   string
	chatGPTUses     int
	chatGPTGate     sync.Mutex
	chatGPTCurrent  string
	memoryAvailable func() uint64
}

func NewFleet(broker *Broker, config Config) *Fleet {
	sessions := map[string]*ChromeSession{}
	for index, spec := range config.Browsers {
		sessions[spec.ID] = NewChromeSession(spec, config.CDPBasePort+index, broker)
	}
	return &Fleet{broker: broker, config: config, sessions: sessions, warmErrors: map[string]string{}}
}
func (f *Fleet) Start() error {
	for _, spec := range f.config.Browsers {
		if !f.startsAtBoot(spec) {
			continue
		}
		session := f.sessions[spec.ID]
		if err := session.Start(); err != nil {
			f.Close()
			return err
		}
	}
	return nil
}

func (f *Fleet) Warm() {
	if f.config.ChatGPTRotateEvery > 0 {
		f.chatGPTGate.Lock()
		defer f.chatGPTGate.Unlock()
	}
	// Loading every provider page at once can exhaust a shared server's RAM
	// and CPU before any composer becomes usable. Keep all profiles, but
	// bound the expensive navigation phase.
	slots := make(chan struct{}, 2)
	var group sync.WaitGroup
	for _, spec := range f.config.Browsers {
		if !f.startsAtBoot(spec) {
			continue
		}
		spec := spec
		group.Add(1)
		go func() {
			defer group.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			session := f.sessions[spec.ID]
			var err error
			if spec.Provider == ProviderChatGPT {
				err = session.PrepareChatGPT()
			} else {
				_, err = session.Prepare(spec.CookieHeader, spec.AuthUser)
			}
			f.setWarmError(spec.ID, err)
		}()
	}
	group.Wait()
}
func (f *Fleet) Close() {
	for _, session := range f.sessions {
		session.Close()
	}
}

func (f *Fleet) Reset(id string) error {
	if f.config.ChatGPTRotateEvery > 0 {
		f.chatGPTGate.Lock()
		defer f.chatGPTGate.Unlock()
	}
	session, err := f.Session(id)
	if err != nil {
		return err
	}
	spec := session.Spec()
	if spec.Provider == ProviderChatGPT {
		f.parkOtherChatGPT(id)
	}
	if err := session.Reset(); err != nil {
		f.setWarmError(spec.ID, err)
		return err
	}
	if spec.Provider == ProviderChatGPT {
		err = session.PrepareChatGPT()
	} else {
		_, err = session.Prepare(spec.CookieHeader, spec.AuthUser)
	}
	if err != nil {
		f.setWarmError(spec.ID, err)
		return err
	}
	f.setWarmError(spec.ID, nil)
	return nil
}

func (f *Fleet) setWarmError(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		delete(f.warmErrors, id)
		return
	}
	if f.warmErrors == nil {
		f.warmErrors = map[string]string{}
	}
	f.warmErrors[id] = err.Error()
}
func (f *Fleet) Resolve(id string) (string, error) {
	if id == "" {
		id = f.config.DefaultID
	}
	if _, ok := f.sessions[id]; !ok {
		return "", fmt.Errorf("Unknown browserId: %s", id)
	}
	return id, nil
}

func (f *Fleet) ResolveChatGPT(id string) (string, error) {
	if id == "" {
		return f.resolveAutomaticChatGPT()
	}
	session := f.sessions[id]
	if session == nil || session.Spec().Provider != ProviderChatGPT {
		return "", fmt.Errorf("Unknown ChatGPT browserId: %s", id)
	}
	return id, nil
}
func (f *Fleet) Session(id string) (*ChromeSession, error) {
	resolved, err := f.Resolve(id)
	if err != nil {
		return nil, err
	}
	return f.sessions[resolved], nil
}
func (f *Fleet) Spec(id string) (BrowserSpec, error) {
	resolved, err := f.Resolve(id)
	if err != nil {
		return BrowserSpec{}, err
	}
	if session := f.sessions[resolved]; session != nil {
		return session.Spec(), nil
	}
	return BrowserSpec{}, fmt.Errorf("Unknown browserId: %s", id)
}
func (f *Fleet) Status() []map[string]any {
	items := make([]map[string]any, 0, len(f.config.Browsers))
	for _, spec := range f.config.Browsers {
		health := f.broker.Health(spec.ID)
		session := f.sessions[spec.ID]
		f.mu.Lock()
		warmError := f.warmErrors[spec.ID]
		f.mu.Unlock()
		ready := session.Ready()
		cookies := session.CookieDiagnostics()
		state := sessionState(ready, health, warmError)
		if f.config.LazyStart && !session.started.Load() && warmError == "" {
			state = "IDLE"
		}
		items = append(items, map[string]any{"browserId": spec.ID, "provider": spec.Provider, "authUser": spec.AuthUser, "connected": health["connected"], "pendingJobs": health["pendingJobs"], "heartbeatAgeSeconds": health["heartbeatAgeSeconds"], "ready": ready, "sessionState": state, "warmError": warmError, "cookieCount": cookies.Count, "authCookieCount": cookies.AuthCount, "cookieRevision": cookies.Revision, "cookieSourceCurrent": cookies.SourceCurrent})
	}
	return items
}

func (f *Fleet) Healthy() bool {
	return healthyStatuses(f.Status())
}

func healthyStatuses(statuses []map[string]any) bool {
	required := map[string]bool{}
	healthy := map[string]bool{}
	for _, status := range statuses {
		provider, _ := status["provider"].(string)
		if provider == "" {
			provider = ProviderAIStudio
		}
		required[provider] = true
		connected, _ := status["connected"].(bool)
		ready, _ := status["ready"].(bool)
		if connected && ready {
			healthy[provider] = true
		}
	}
	for provider := range required {
		if !healthy[provider] {
			return false
		}
	}
	return len(required) > 0
}

func (f *Fleet) MonitorChatGPT(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failures := map[string]int{}
	retryAfter := map[string]time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if f.config.ChatGPTRotateEvery > 0 {
				if !f.chatGPTGate.TryLock() {
					continue
				}
				f.parkOtherChatGPT(f.chatGPTCurrent)
			}
			for _, spec := range f.config.Browsers {
				if spec.Provider != ProviderChatGPT {
					continue
				}
				if time.Now().Before(retryAfter[spec.ID]) {
					continue
				}
				session := f.sessions[spec.ID]
				if (f.config.LazyStart || f.config.ChatGPTRotateEvery > 0) && !session.started.Load() {
					continue
				}
				health := f.broker.Health(spec.ID)
				pendingJobs, _ := health["pendingJobs"].(int)
				if pendingJobs > 0 {
					// Reloading chatgpt.com can briefly pause the extension heartbeat.
					// An in-flight job owns this browser and must not be interrupted.
					failures[spec.ID] = 0
					continue
				}
				connected, _ := health["connected"].(bool)
				if connected && session.refreshChatGPTReady() {
					f.setWarmError(spec.ID, nil)
					failures[spec.ID] = 0
					delete(retryAfter, spec.ID)
					continue
				}
				if !connected && session.Ready() {
					session.MarkProviderUnready()
				}
				if connected {
					failures[spec.ID] = 0
					continue
				}
				failures[spec.ID]++
				if reason := f.warmError(spec.ID); isChatGPTChallengeError(reason) || isChatGPTSessionError(reason) {
					// Reloading cannot resolve a sign-in or a human verification step.
					continue
				}
				if !shouldRestartChatGPTBrowser(connected, failures[spec.ID]) {
					continue
				}
				failures[spec.ID] = 0
				log.Printf("restarting unavailable ChatGPT browser %s", spec.ID)
				// A successful launch can still stall under load. Apply the cooldown
				// to every restart, not only failed launches, to avoid restart storms.
				retryAfter[spec.ID] = time.Now().Add(5 * time.Minute)
				if err := session.RestartChatGPT(); err != nil {
					f.setWarmError(spec.ID, err)
					retryAfter[spec.ID] = time.Now().Add(5 * time.Minute)
					log.Printf("ChatGPT browser %s restart failed: %v", spec.ID, err)
					continue
				}
				f.setWarmError(spec.ID, nil)
			}
			if f.config.ChatGPTRotateEvery > 0 {
				f.chatGPTGate.Unlock()
			}
		}
	}
}

func shouldRestartChatGPTBrowser(connected bool, failures int) bool {
	return !connected && failures >= 6
}

func (f *Fleet) warmError(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.warmErrors[id]
}

func sessionState(ready bool, health map[string]any, warmError string) string {
	connected, _ := health["connected"].(bool)
	if !connected {
		return "DISCONNECTED"
	}
	if ready {
		return "READY"
	}
	if warmError != "" {
		return "ERROR"
	}
	return "WARMING"
}
