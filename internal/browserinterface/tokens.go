package browserinterface

import (
	"fmt"
	"log"
	"sync"
)

type TokenService struct {
	broker    *Broker
	fleet     *Fleet
	mu        sync.Mutex
	locks     map[string]*sync.Mutex
	activated map[string]providerActivation
}

type providerActivation struct {
	sessionID string
	index     int
}

func NewTokenService(broker *Broker, fleet *Fleet) *TokenService {
	return &TokenService{
		broker: broker, fleet: fleet,
		locks: map[string]*sync.Mutex{}, activated: map[string]providerActivation{},
	}
}
func (s *TokenService) Create(body map[string]any) (map[string]any, error) {
	if enabled, ok := body["attestationEnabled"].(bool); ok && !enabled {
		return map[string]any{"token": ""}, nil
	}
	_, authUser, err := ValidateTokenRequest(body)
	if err != nil {
		return nil, err
	}
	browserID, err := s.fleet.Resolve(fmt.Sprint(body["browserId"]))
	if err != nil {
		return nil, err
	}
	lock := s.browserLock(browserID)
	lock.Lock()
	defer lock.Unlock()
	spec, err := s.fleet.Spec(browserID)
	if err != nil {
		return nil, err
	}
	cookies, _ := body["cookies"].(string)
	if SessionFingerprint(cookies, authUser) != SessionFingerprint(spec.CookieHeader, spec.AuthUser) {
		return nil, fmt.Errorf("Cookies do not belong to selected browserId: %s", browserID)
	}
	session, err := s.fleet.Session(browserID)
	if err != nil {
		return nil, err
	}
	prepared, err := session.Prepare(cookies, authUser)
	if err != nil {
		return nil, err
	}
	sessionID := session.Fingerprint()
	providerIndex, providerSelected := s.selectedProvider(browserID, sessionID)
	payload := map[string]any{"digest": body["digest"], "authUser": authUser}
	if providerSelected {
		payload["providerIndex"] = providerIndex
	}
	request, err := s.broker.Request(payload, browserID)
	if err != nil {
		return nil, err
	}
	token, _ := request["token"].(string)
	if token == "" {
		return nil, fmt.Errorf("Container extension returned an empty token")
	}
	if !providerSelected {
		providerIndex, err := providerIndexFromSnapshot(request)
		if err != nil {
			return nil, err
		}
		s.activate(browserID, sessionID, providerIndex)
	}
	current, err := session.Snapshot()
	if err != nil {
		return nil, err
	}
	runtime, _ := prepared["runtimeConfig"].(map[string]any)
	if runtime == nil {
		runtime = map[string]any{}
	}
	if extensionRuntime, ok := request["runtimeConfig"].(map[string]any); ok {
		for name, value := range extensionRuntime {
			runtime[name] = value
		}
	}
	runtime["authUser"] = authUser
	return map[string]any{"token": token, "cookieRecords": current["cookieRecords"], "transportProfile": current["transportProfile"], "runtimeConfig": runtime, "browserId": browserID}, nil
}

func (s *TokenService) browserLock(browserID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.locks[browserID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[browserID] = lock
	}
	return lock
}

func (s *TokenService) selectedProvider(browserID, sessionID string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	activation, found := s.activated[browserID]
	return activation.index, found && activation.sessionID == sessionID
}

func (s *TokenService) activate(browserID, sessionID string, index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activated[browserID] = providerActivation{sessionID: sessionID, index: index}
	log.Printf("activated native provider %d for browser %s", index, browserID)
}

func (s *TokenService) deactivate(browserID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activated, browserID)
}

func providerIndexFromSnapshot(snapshot map[string]any) (int, error) {
	candidates, _ := snapshot["candidateTokens"].([]any)
	if len(candidates) > 0 {
		return len(candidates) - 1, nil
	}
	if token, _ := snapshot["token"].(string); token != "" {
		return 0, nil
	}
	return 0, fmt.Errorf("native provider returned no token")
}
