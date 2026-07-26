package browserinterface

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type TokenService struct {
	broker    *Broker
	fleet     *Fleet
	mu        sync.Mutex
	locks     map[string]*sync.Mutex
	activated map[string]string
}

func NewTokenService(broker *Broker, fleet *Fleet) *TokenService {
	return &TokenService{
		broker: broker, fleet: fleet,
		locks: map[string]*sync.Mutex{}, activated: map[string]string{},
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
	request, err := s.broker.Request(map[string]any{"digest": body["digest"], "authUser": authUser}, browserID)
	if err != nil {
		return nil, err
	}
	token, _ := request["token"].(string)
	if token == "" {
		return nil, fmt.Errorf("Container extension returned an empty token")
	}
	sessionID := session.Fingerprint()
	if os.Getenv("TOKEN_FACTORY_SAME_BROWSER_PROBE") == "1" && s.activatedSession(browserID) != sessionID {
		providerIndex, err := s.selectProvider(body, request, session)
		if err != nil {
			return nil, err
		}
		request, err = s.broker.Request(map[string]any{
			"digest": body["digest"], "authUser": authUser, "providerIndex": providerIndex,
		}, browserID)
		if err != nil {
			return nil, err
		}
		token, _ = request["token"].(string)
		if token == "" {
			return nil, fmt.Errorf("Container extension returned an empty provider token")
		}
		s.activate(browserID, sessionID)
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

func (s *TokenService) activatedSession(browserID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activated[browserID]
}

func (s *TokenService) activate(browserID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activated[browserID] = sessionID
}

func (s *TokenService) selectProvider(body, snapshot map[string]any, session *ChromeSession) (int, error) {
	tokens := []string{}
	if candidates, ok := snapshot["candidateTokens"].([]any); ok {
		for _, candidate := range candidates {
			if token, ok := candidate.(string); ok && token != "" {
				tokens = append(tokens, token)
			}
		}
	}
	if len(tokens) == 0 {
		if token, _ := snapshot["token"].(string); token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return 0, fmt.Errorf("native provider returned no candidate token")
	}
	generateRequest, _ := body["generateRequest"].(map[string]any)
	diagnostics := []string{}
	for index, token := range tokens {
		probe, err := session.Probe(generateRequest, token)
		if err == nil && probe.Status != 0 && probe.Status != 401 && probe.Status != 403 {
			return index, nil
		}
		if err != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%d:error=%v", index, err))
			continue
		}
		body := probe.Body
		if len(body) > 160 {
			body = body[:160]
		}
		diagnostics = append(diagnostics, fmt.Sprintf(
			"%d:status=%d network=%q location=%q blocked=%q directive=%q body=%q",
			index, probe.Status, probe.NetworkError, probe.Location,
			probe.BlockedURI, probe.ViolatedDirective, body,
		))
	}
	fallback := len(tokens) - 1
	log.Printf(
		"same-browser probe was inconclusive; using native provider %d (%s)",
		fallback,
		strings.Join(diagnostics, "; "),
	)
	return fallback, nil
}
