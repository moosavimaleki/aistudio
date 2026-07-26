package browserinterface

import (
	"fmt"
	"sync"
)

type Fleet struct {
	broker     *Broker
	config     Config
	sessions   map[string]*ChromeSession
	mu         sync.Mutex
	warmErrors map[string]string
}

func NewFleet(broker *Broker, config Config) *Fleet {
	sessions := map[string]*ChromeSession{}
	for index, spec := range config.Browsers {
		sessions[spec.ID] = NewChromeSession(spec, config.CDPBasePort+index, broker)
	}
	return &Fleet{broker: broker, config: config, sessions: sessions, warmErrors: map[string]string{}}
}
func (f *Fleet) Start() error {
	for _, session := range f.sessions {
		if err := session.Start(); err != nil {
			f.Close()
			return err
		}
	}
	return nil
}

func (f *Fleet) Warm() {
	for _, spec := range f.config.Browsers {
		session := f.sessions[spec.ID]
		if _, err := session.Prepare(spec.CookieHeader, spec.AuthUser); err != nil {
			f.mu.Lock()
			f.warmErrors[spec.ID] = err.Error()
			f.mu.Unlock()
		}
	}
}
func (f *Fleet) Close() {
	for _, session := range f.sessions {
		session.Close()
	}
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
		items = append(items, map[string]any{"browserId": spec.ID, "authUser": spec.AuthUser, "connected": health["connected"], "pendingJobs": health["pendingJobs"], "heartbeatAgeSeconds": health["heartbeatAgeSeconds"], "ready": ready, "sessionState": sessionState(ready, health), "warmError": warmError})
	}
	return items
}

func (f *Fleet) Healthy() bool {
	for _, status := range f.Status() {
		if status["browserId"] == f.config.DefaultID {
			connected, _ := status["connected"].(bool)
			ready, _ := status["ready"].(bool)
			return connected && ready
		}
	}
	return false
}

func sessionState(ready bool, health map[string]any) string {
	if ready {
		return "READY"
	}
	if connected, _ := health["connected"].(bool); connected {
		return "WARMING"
	}
	return "DISCONNECTED"
}
