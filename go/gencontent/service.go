package gencontent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hamed/aistudio-api/go/aistudio"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type BrowserProfile struct {
	Slot             int
	ID, AuthUser     string
	Connected, Ready bool
}
type Service struct {
	settings  aistudio.Settings
	pool      *Pool
	healthURL string
	http      *http.Client
}

func NewService(settings aistudio.Settings, pool *Pool) *Service {
	endpoint := settings.TokenFactoryURL
	parsed, _ := url.Parse(endpoint)
	parsed.Path = "/health"
	parsed.RawQuery = ""
	return &Service{settings: settings, pool: pool, healthURL: parsed.String(), http: &http.Client{}}
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
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Cannot read browser profiles: HTTP %d", response.StatusCode)
	}
	var body struct {
		Browsers []struct {
			ID        string `json:"browserId"`
			AuthUser  string `json:"authUser"`
			Connected bool   `json:"connected"`
			Ready     bool   `json:"ready"`
		} `json:"browsers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, err
	}
	items := make([]BrowserProfile, 0, len(body.Browsers))
	for index, item := range body.Browsers {
		items = append(items, BrowserProfile{Slot: index + 1, ID: item.ID, AuthUser: item.AuthUser, Connected: item.Connected, Ready: item.Ready})
	}
	return items, nil
}
func (s *Service) chooseProfile(ctx context.Context) (BrowserProfile, error) {
	profiles, err := s.profiles(ctx)
	if err != nil {
		return BrowserProfile{}, err
	}
	ready := []BrowserProfile{}
	for _, profile := range profiles {
		if profile.Connected && profile.Ready {
			ready = append(ready, profile)
		}
	}
	if len(ready) == 0 {
		return BrowserProfile{}, aistudio.NewError("CONFIG", "No ready Chrome profile is available")
	}
	return ready[rand.IntN(len(ready))], nil
}
func (s *Service) Generate(ctx context.Context, input aistudio.GenerateInput) (map[string]any, error) {
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
	profile, err := s.chooseProfile(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.profileSettings(profile)
	if err != nil {
		return nil, err
	}
	tab, err := aistudio.NewTab(settings, nil, lease.TabID)
	if err != nil {
		return nil, err
	}
	if _, err = tab.Initialize(ctx); err != nil {
		return nil, err
	}
	input, err = resolveInlineData(ctx, input, tab)
	if err != nil {
		return nil, err
	}
	result, err := tab.Generate(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	state := map[string]any{"browserId": profile.ID, "authUser": profile.AuthUser, "generateCount": tab.GenerateCount}
	if err := s.pool.Release(ctx, lease, state); err != nil {
		return nil, err
	}
	released = true
	return map[string]any{"tabId": tab.ID, "browserId": profile.ID, "authUser": profile.AuthUser, "generateCount": tab.GenerateCount, "result": result}, nil
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
