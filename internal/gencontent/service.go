package gencontent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BrowserProfile struct {
	Slot                   int
	ID, Provider, AuthUser string
	Connected, Ready       bool
}
type Service struct {
	settings        aistudio.Settings
	pool            *Pool
	healthURL       string
	http            *http.Client
	recoveryURL     string
	recoveryHTTP    *http.Client
	profileFailures *profileFailures
}

func NewService(settings aistudio.Settings, pool *Pool) *Service {
	endpoint := settings.TokenFactoryURL
	parsed, _ := url.Parse(endpoint)
	parsed.Path = "/health"
	parsed.RawQuery = ""
	recoveryURL := *parsed
	recoveryURL.Path = "/internal/browsers"
	return &Service{
		settings: settings, pool: pool, healthURL: parsed.String(),
		http:        &http.Client{Timeout: 5 * time.Second},
		recoveryURL: recoveryURL.String(), recoveryHTTP: &http.Client{Timeout: 90 * time.Second},
		profileFailures: newProfileFailures(),
	}
}
func (s *Service) profiles(ctx context.Context) ([]BrowserProfile, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", s.healthURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Cannot read browser profiles: %w", err)
	}
	defer response.Body.Close()
	var body struct {
		Browsers []struct {
			ID        string `json:"browserId"`
			Provider  string `json:"provider"`
			AuthUser  string `json:"authUser"`
			Connected bool   `json:"connected"`
			Ready     bool   `json:"ready"`
		} `json:"browsers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK && len(body.Browsers) == 0 {
		return nil, fmt.Errorf("Cannot read browser profiles: HTTP %d", response.StatusCode)
	}
	items := make([]BrowserProfile, 0, len(body.Browsers))
	slot := 0
	for _, item := range body.Browsers {
		if item.Provider != "" && item.Provider != "aistudio" {
			continue
		}
		slot++
		items = append(items, BrowserProfile{Slot: slot, ID: item.ID, Provider: item.Provider, AuthUser: item.AuthUser, Connected: item.Connected, Ready: item.Ready})
	}
	return items, nil
}
func (s *Service) chooseProfile(ctx context.Context) (BrowserProfile, error) {
	profiles, err := s.profiles(ctx)
	if err != nil {
		return BrowserProfile{}, err
	}
	if profile, found := s.selectProfile(profiles); found {
		return profile, nil
	}
	waited, err := s.waitForProfileCooldown(ctx, profiles)
	if err != nil {
		return BrowserProfile{}, err
	}
	if waited {
		profiles, err = s.profiles(ctx)
		if err != nil {
			return BrowserProfile{}, err
		}
		if profile, found := s.selectProfile(profiles); found {
			return profile, nil
		}
	}
	candidates := profiles
	if preferred := os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID"); preferred != "" {
		candidates = nil
		for _, profile := range profiles {
			if profile.ID == preferred {
				candidates = append(candidates, profile)
			}
		}
	}
	s.recoverProfiles(ctx, candidates)
	profiles, err = s.profiles(ctx)
	if err != nil {
		return BrowserProfile{}, err
	}
	if profile, found := s.selectProfile(profiles); found {
		return profile, nil
	}
	return BrowserProfile{}, s.noUsableProfileError(profiles)
}

func (s *Service) selectProfile(profiles []BrowserProfile) (BrowserProfile, bool) {
	ready := []BrowserProfile{}
	for _, profile := range profiles {
		if profile.Connected && profile.Ready && !s.profileFailures.Has(profile.ID) {
			ready = append(ready, profile)
		}
	}
	if preferred := os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID"); preferred != "" {
		for _, profile := range ready {
			if profile.ID == preferred {
				return profile, true
			}
		}
		return BrowserProfile{}, false
	}
	if len(ready) == 0 {
		return BrowserProfile{}, false
	}
	return ready[rand.IntN(len(ready))], true
}

func (s *Service) noUsableProfileError(profiles []BrowserProfile) error {
	message := "No usable Chrome profile is available"
	if preferred := os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID"); preferred != "" {
		message = "Preferred Chrome profile is not usable: " + preferred
	}
	return &aistudio.ClientError{
		Message: message, Phase: "HEALTH", Status: http.StatusServiceUnavailable,
		Diagnostics: s.profileFailureDiagnostics(profiles),
	}
}
func (s *Service) generateOnce(ctx context.Context, input aistudio.GenerateInput) (map[string]any, error) {
	profile, err := s.chooseProfile(ctx)
	if err != nil {
		return nil, err
	}
	return s.generateWithProfile(ctx, input, profile)
}

func (s *Service) generateForBrowser(ctx context.Context, input aistudio.GenerateInput, browserID string) (map[string]any, error) {
	profiles, err := s.profiles(ctx)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile.ID == browserID && profile.Connected && profile.Ready {
			return s.generateWithProfile(ctx, input, profile)
		}
	}
	return nil, s.noUsableProfileError(profiles)
}

func (s *Service) generateWithProfile(ctx context.Context, input aistudio.GenerateInput, profile BrowserProfile) (map[string]any, error) {
	lease, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	released := false
	defer func() {
		if !released {
			_ = s.pool.Discard(context.Background(), lease)
		}
	}()
	settings, err := s.profileSettings(profile)
	if err != nil {
		return nil, err
	}
	tab, err := aistudio.NewTab(settings, nil, lease.TabID)
	if err != nil {
		return nil, err
	}
	if _, err = tab.Initialize(ctx); err != nil {
		return nil, s.profileError(profile, err)
	}
	input, err = resolveInlineData(ctx, input, tab)
	if err != nil {
		return nil, s.profileError(profile, err)
	}
	result, err := tab.Generate(ctx, input, nil)
	if err != nil {
		return nil, s.profileError(profile, err)
	}
	state := map[string]any{"browserId": profile.ID, "authUser": profile.AuthUser, "generateCount": tab.GenerateCount}
	if err := s.pool.Release(ctx, lease, state); err != nil {
		return nil, err
	}
	s.profileFailures.Clear(profile.ID)
	released = true
	return map[string]any{"tabId": tab.ID, "browserId": profile.ID, "authUser": profile.AuthUser, "generateCount": tab.GenerateCount, "result": result}, nil
}

func (s *Service) Health(ctx context.Context) (map[string]any, error) {
	profiles, err := s.profiles(ctx)
	if err != nil {
		return map[string]any{"service": "gencontent", "pool": s.pool.Stats(ctx), "usableProfiles": 0}, &aistudio.ClientError{
			Message: "Cannot read browser profile health", Phase: "HEALTH", Status: http.StatusServiceUnavailable,
			Diagnostics: map[string]any{"cause": err.Error()},
		}
	}
	usable := 0
	for _, profile := range profiles {
		if profile.Connected && profile.Ready && !s.profileFailures.Has(profile.ID) {
			usable++
		}
	}
	result := map[string]any{
		"service": "gencontent", "pool": s.pool.Stats(ctx), "usableProfiles": usable,
		"profileFailures": s.profileFailureDiagnostics(profiles),
	}
	if usable == 0 {
		return result, &aistudio.ClientError{
			Message: "No usable Chrome profile is available", Phase: "HEALTH", Status: http.StatusServiceUnavailable,
			Diagnostics: s.profileFailureDiagnostics(profiles),
		}
	}
	return result, nil
}

func (s *Service) profileError(profile BrowserProfile, err error) error {
	var client *aistudio.ClientError
	if !errors.As(err, &client) {
		return err
	}
	if client.Diagnostics == nil {
		client.Diagnostics = map[string]any{}
	}
	client.Diagnostics["browserId"] = profile.ID
	client.Diagnostics["authUser"] = profile.AuthUser
	if aistudio.InvalidatesTab(client) || client.Status == http.StatusTooManyRequests {
		s.profileFailures.Mark(profile, client)
		client.Diagnostics["profileQuarantined"] = true
	}
	return err
}

func (s *Service) profileFailureDiagnostics(profiles []BrowserProfile) map[string]any {
	items := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		failure, found := s.profileFailures.Status(profile.ID)
		item := map[string]any{
			"browserId": profile.ID, "ready": profile.Ready, "connected": profile.Connected,
			"quarantined": s.profileFailures.Has(profile.ID),
		}
		if found {
			item["failure"] = failure
		}
		items = append(items, item)
	}
	return map[string]any{"profiles": items}
}

func resolveInlineData(ctx context.Context, input aistudio.GenerateInput, tab *aistudio.Tab) (aistudio.GenerateInput, error) {
	if len(input.Contents) == 0 {
		return input, nil
	}
	encoded, _ := json.Marshal(input.Contents)
	var contents []any
	if err := json.Unmarshal(encoded, &contents); err != nil {
		return input, err
	}
	for _, rawContent := range contents {
		content, ok := rawContent.(map[string]any)
		if !ok {
			continue
		}
		parts, _ := content["parts"].([]any)
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			inline, found, err := readInlinePart(part)
			if err != nil {
				return input, err
			}
			if !found {
				continue
			}
			decoded, err := aistudio.DecodeInlineData(inline.data)
			if err != nil {
				return input, fmt.Errorf("inlineData.data must be valid base64")
			}
			fileID, err := tab.UploadBytes(ctx, decoded, inline.mimeType, inline.displayName)
			if err != nil {
				return input, err
			}
			delete(part, "inlineData")
			delete(part, "inline_data")
			part["fileData"] = map[string]any{"fileId": fileID, "mimeType": inline.mimeType}
		}
	}
	input.Contents = contents
	return input, nil
}
func (s *Service) profileSettings(profile BrowserProfile) (aistudio.Settings, error) {
	files, err := aistudio.DiscoverCookieFiles(s.settings.Values["AISTUDIO_COOKIE_DIR"])
	if err != nil {
		return aistudio.Settings{}, err
	}
	if profile.Slot < 1 || profile.Slot > len(files) {
		return aistudio.Settings{}, fmt.Errorf("Cannot load cookie profile %d", profile.Slot)
	}
	text, err := os.ReadFile(filepath.Clean(files[profile.Slot-1]))
	if err != nil {
		return aistudio.Settings{}, err
	}
	header, err := aistudio.ParseNetscapeCookieHeader(string(text), "aistudio.google.com")
	if err != nil {
		return aistudio.Settings{}, err
	}
	settings := s.settings
	settings.BrowserID, settings.AuthUser, settings.CookieHeader = profile.ID, profile.AuthUser, header
	return settings, nil
}
func normalizeModel(model string) string {
	if strings.HasPrefix(model, "models/") {
		return model
	}
	return "models/" + model
}
