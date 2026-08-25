package aistudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TabState string

const (
	TabNew          TabState = "NEW"
	TabInitializing TabState = "INITIALIZING"
	TabReady        TabState = "READY"
	TabGenerating   TabState = "GENERATING"
	TabInvalid      TabState = "INVALID"
	TabFailed       TabState = "FAILED"
	TabClosed       TabState = "CLOSED"
)

type Tab struct {
	ID                      string
	Settings                Settings
	HTTP                    *HTTPClient
	State                   TabState
	Cookies                 *CookieJar
	Auth                    *AuthContext
	Runtime                 RuntimeConfig
	TransportProfile        map[string]string
	TokenFactory            *StagingTokenFactory
	LoggingContextExtension string
	OAuthAccessToken        string
	AppFolderID             string
	GenerateCount           int
}

func NewTab(settings Settings, httpClient *HTTPClient, id string) (*Tab, error) {
	if httpClient == nil {
		var err error
		httpClient, err = NewHTTPClient(settings.ProxyURL)
		if err != nil {
			return nil, err
		}
	}
	auth, err := NewAuthContext(settings.OriginURL, settings.CookieHeader)
	if err != nil {
		return nil, err
	}
	if id == "" {
		id = fmt.Sprintf("tab-%d", time.Now().UnixNano())
	}
	return &Tab{ID: id, Settings: settings, HTTP: httpClient, State: TabNew, Cookies: NewCookieJar(settings.CookieHeader), Auth: auth}, nil
}
func (t *Tab) syncSession() {
	t.Auth.CookieHeader = t.Cookies.Header()
	if t.TokenFactory != nil {
		t.TokenFactory.Auth = t.Auth
		t.TokenFactory.Runtime = t.Runtime
	}
}
func (t *Tab) Initialize(ctx context.Context) (*Tab, error) {
	if t.State != TabNew {
		return nil, NewError("CONFIG", "Cannot initialize tab outside NEW state")
	}
	t.State = TabInitializing
	runtime, profile, err := FetchRuntimeConfig(ctx, t.HTTP, t.Cookies, t.Settings.TokenFactoryURL, t.Settings.AuthUser, t.Settings.BrowserID)
	if err != nil {
		t.State = TabFailed
		return nil, err
	}
	t.Runtime, t.TransportProfile = runtime, profile
	t.syncSession()
	t.TokenFactory = &StagingTokenFactory{HTTP: t.HTTP, URL: t.Settings.TokenFactoryURL, WAAAPIKey: t.Settings.WAAAPIKey, Auth: t.Auth, Runtime: runtime, BrowserID: t.Settings.BrowserID}
	if err := t.initializeStartup(ctx); err != nil {
		t.State = TabFailed
		return nil, err
	}
	t.State = TabReady
	return t, nil
}
func (t *Tab) Generate(ctx context.Context, input GenerateInput, onChunk func(any)) (GenerateResult, error) {
	if t.State != TabReady || t.TokenFactory == nil {
		return GenerateResult{}, NewError("RPC", "Tab is not ready")
	}
	payload, err := BuildGeneratePayload(input)
	if err != nil {
		return GenerateResult{}, err
	}
	digest, err := ContentBindingDigest(payload)
	if err != nil {
		return GenerateResult{}, err
	}
	t.State = TabGenerating
	defer func() {
		if t.State == TabGenerating {
			t.State = TabReady
		}
	}()
	rpcURL, err := RPCURL("GenerateContent")
	if err != nil {
		return GenerateResult{}, err
	}
	for attempt := 0; attempt <= 4; attempt++ {
		payload[4] = nil
		headers, err := ComposeHeaders(t.Auth, t.Cookies.Header(), t.Runtime, t.TransportProfile, t.LoggingContextExtension)
		if err != nil {
			return GenerateResult{}, err
		}
		snapshot, err := t.TokenFactory.Snapshot(ctx, digest, map[string]any{"url": rpcURL, "method": "POST", "headers": headers, "payload": append([]any(nil), payload...)})
		if err != nil {
			return GenerateResult{}, err
		}
		t.Cookies.Apply(snapshot.CookieRecords)
		t.syncSession()
		runtime := t.Runtime
		if snapshot.RuntimeConfig != nil {
			runtime = *snapshot.RuntimeConfig
		}
		payload[4] = snapshot.Token
		loggingContext := t.LoggingContextExtension
		if snapshot.LoggingContextExtension != "" {
			loggingContext = snapshot.LoggingContextExtension
		}
		headers, err = ComposeHeaders(t.Auth, t.Cookies.Header(), runtime, mergeHeaders(t.TransportProfile, snapshot.TransportProfile), loggingContext)
		if err != nil {
			return GenerateResult{}, err
		}
		encoded, _ := json.Marshal(payload)
		response, err := t.HTTP.Request(ctx, "POST", rpcURL, headers, encoded)
		if err != nil {
			if attempt < 4 && retryableGenerateTransportError(ctx, err) {
				log.Printf("GenerateContent transport attempt %d failed; retrying with a fresh token: %v", attempt+1, err)
				time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
				continue
			}
			return GenerateResult{}, err
		}
		t.Cookies.ApplyResponse(response)
		t.syncSession()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			result, collectErr := CollectGenerateResult(response.Body, onChunk)
			response.Body.Close()
			if collectErr != nil {
				return result, collectErr
			}
			t.GenerateCount++
			t.State = TabReady
			return result, nil
		}
		body, _ := ReadBody(response)
		response.Body.Close()
		if !retryableGenerateStatus(response.StatusCode) || attempt == 4 {
			return GenerateResult{}, &ClientError{Message: fmt.Sprintf("GenerateContent failed with HTTP %d", response.StatusCode), Phase: "RPC", Status: response.StatusCode, ResponseBody: body}
		}
		time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
	}
	return GenerateResult{}, NewError("RPC", "GenerateContent retries exhausted")
}
func mergeHeaders(base, override map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
func retryableStatus(status int) bool {
	return status == 408 || status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}

func retryableGenerateStatus(status int) bool {
	return status != http.StatusTooManyRequests && retryableStatus(status)
}

func retryableGenerateTransportError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var transportError *url.Error
	return errors.As(err, &transportError)
}

func InvalidatesTab(err error) bool {
	value, ok := err.(*ClientError)
	if !ok {
		return false
	}
	return value.Status == 401 || value.Status == 403 || (value.Phase == "ATTESTATION" && (strings.Contains(value.ResponseBody, "differs from container Chrome") ||
		strings.Contains(value.ResponseBody, "Container browser session differs") ||
		strings.Contains(value.ResponseBody, "No native provider was accepted")))
}
