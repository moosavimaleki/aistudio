package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"github.com/hamed/aistudio-api/internal/chromeprocess"
)

type ChromeSession struct {
	spec        BrowserSpec
	port        int
	broker      *Broker
	process     *chromeprocess.Process
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	stateMu     sync.RWMutex
	headersMu   sync.RWMutex
	primeMu     sync.Mutex
	primeResult chan error
	fingerprint string
	runtime     aistudio.RuntimeConfig
	profile     map[string]string
	cookies     []aistudio.CookieRecord
	headers     map[string]string
}

func NewChromeSession(spec BrowserSpec, port int, broker *Broker) *ChromeSession {
	return &ChromeSession{spec: spec, port: port, broker: broker, process: chromeprocess.NewProcess(spec.ID, port, 0), headers: map[string]string{}}
}
func (s *ChromeSession) Start() error {
	if s.process.Running() {
		return nil
	}
	if err := s.process.Start(); err != nil {
		return err
	}
	allocator, allocatorCancel := chromedp.NewRemoteAllocator(context.Background(), s.process.CDPURL())
	s.ctx, s.cancel = chromedp.NewContext(allocator)
	chromedp.ListenTarget(s.ctx, func(event any) {
		switch request := event.(type) {
		case *network.EventRequestWillBeSent:
			if strings.Contains(request.Request.URL, "MakerSuiteService/") {
				s.captureHeaders(request.Request.Headers)
			}
		case *network.EventRequestWillBeSentExtraInfo:
			if headerValue(request.Headers, "x-client-data") != "" {
				s.captureHeaders(request.Headers)
			}
		case *fetch.EventRequestPaused:
			s.captureNativeGenerate(request)
		}
	})
	_ = allocatorCancel
	return chromedp.Run(s.ctx, network.Enable())
}
func (s *ChromeSession) Prepare(cookies, authUser string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Start(); err != nil {
		return nil, err
	}
	fingerprint := SessionFingerprint(cookies, authUser)
	s.stateMu.RLock()
	sameSession := s.fingerprint == fingerprint && s.runtime.APIKey != ""
	s.stateMu.RUnlock()
	if sameSession {
		return s.snapshotLocked()
	}
	for _, cookie := range parseCookieHeader(cookies) {
		err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetCookie(cookie.Name, cookie.Value).WithDomain(".google.com").WithPath("/").WithSecure(true).Do(ctx)
		}))
		if err != nil && requiredSessionCookie(cookie.Name) {
			return nil, fmt.Errorf("Chrome did not apply incoming cookie %s: %w", cookie.Name, err)
		}
	}
	currentCookies, err := s.readCookies()
	if err != nil {
		return nil, err
	}
	actual := map[string]string{}
	for _, cookie := range currentCookies {
		actual[cookie.Name] = cookie.Value
	}
	for _, cookie := range parseCookieHeader(cookies) {
		if requiredSessionCookie(cookie.Name) && actual[cookie.Name] != cookie.Value {
			return nil, fmt.Errorf("Chrome did not apply incoming cookie %s", cookie.Name)
		}
	}
	upstream, err := aistudio.LoadUpstream()
	if err != nil {
		return nil, err
	}
	endpoint := upstream.AIStudio["bootstrap_url"]
	if authUser != "0" {
		endpoint = strings.Replace(upstream.AIStudio["account_bootstrap_url"], "{auth_user}", authUser, 1)
	}
	pageContext, cancel := context.WithTimeout(s.ctx, 35*time.Second)
	defer cancel()
	var html string
	if err := chromedp.Run(pageContext,
		chromedp.ActionFunc(func(ctx context.Context) error { _, _, _, err := page.Navigate(endpoint).Do(ctx); return err }),
		chromedp.Sleep(2*time.Second),
		chromedp.OuterHTML("html", &html),
	); err != nil {
		return nil, err
	}
	if strings.Contains(s.currentURL(), "accounts.google.com") {
		return nil, fmt.Errorf("AI Studio redirected the container browser to sign-in; cookie session is invalid")
	}
	runtime, err := aistudio.ExtractRuntimeConfig(html, authUser)
	if err != nil {
		return nil, err
	}
	var browser struct {
		UserAgent string `json:"userAgent"`
	}
	if err := chromedp.Run(s.ctx, chromedp.Evaluate(`({userAgent:navigator.userAgent})`, &browser)); err != nil {
		return nil, err
	}
	profile := map[string]string{
		"Accept": "*/*", "Accept-Language": "en-US,en;q=0.9,fa;q=0.8",
		"User-Agent": browser.UserAgent, "Priority": "u=1, i",
		"sec-fetch-dest": "empty", "sec-fetch-mode": "cors", "sec-fetch-site": "same-site",
		"x-client-data": upstream.Opaque["x_client_data"],
	}
	primeContext, primeCancel := context.WithTimeout(s.ctx, 50*time.Second)
	defer primeCancel()
	if err := s.primeNativeGenerate(primeContext); err != nil {
		return nil, err
	}
	identityContext, identityCancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer identityCancel()
	if err := s.waitForRPCIdentity(identityContext); err != nil {
		return nil, err
	}
	for name, value := range s.observedTransportProfile() {
		profile[name] = value
	}
	currentCookies, err = s.readCookies()
	if err != nil {
		return nil, err
	}
	s.stateMu.Lock()
	s.runtime = runtime
	s.profile = profile
	s.cookies = currentCookies
	s.fingerprint = SessionFingerprint(joinCookies(currentCookies), authUser)
	s.stateMu.Unlock()
	return s.snapshotLocked()
}
func (s *ChromeSession) Snapshot() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}
func (s *ChromeSession) Ready() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.fingerprint != "" &&
		s.runtime.APIKey != "" &&
		s.profile["User-Agent"] != "" &&
		s.profile["x-client-data"] != ""
}
func (s *ChromeSession) snapshotLocked() (map[string]any, error) {
	s.stateMu.RLock()
	if s.runtime.APIKey == "" {
		s.stateMu.RUnlock()
		return nil, fmt.Errorf("Container browser session is not ready")
	}
	runtimeConfig := s.runtime
	profile := make(map[string]string, len(s.profile))
	for name, value := range s.profile {
		profile[name] = value
	}
	s.stateMu.RUnlock()
	browserCookies, err := s.readBrowserCookies()
	if err != nil {
		return nil, err
	}
	cookies := googleCookieRecords(browserCookies)
	if err := persistCookieFile(s.spec.CookieFile, browserCookies); err != nil {
		return nil, fmt.Errorf("persist Chrome cookies: %w", err)
	}
	s.stateMu.Lock()
	s.cookies = cookies
	s.spec.CookieHeader = joinCookies(cookies)
	s.fingerprint = SessionFingerprint(s.spec.CookieHeader, runtimeConfig.AuthUser)
	s.stateMu.Unlock()
	runtime := map[string]any{"apiKey": runtimeConfig.APIKey, "visitId": runtimeConfig.VisitID, "authUser": runtimeConfig.AuthUser, "attestationEnabled": runtimeConfig.AttestationEnabled}
	return map[string]any{"runtimeConfig": runtime, "transportProfile": profile, "cookieRecords": cookies}, nil
}
func (s *ChromeSession) readCookies() ([]aistudio.CookieRecord, error) {
	cookies, err := s.readBrowserCookies()
	if err != nil {
		return nil, err
	}
	return googleCookieRecords(cookies), nil
}
func (s *ChromeSession) readBrowserCookies() ([]*network.Cookie, error) {
	var cookies []*network.Cookie
	err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var readErr error
		cookies, readErr = network.GetCookies().WithURLs([]string{"https://aistudio.google.com/"}).Do(ctx)
		return readErr
	}))
	if err != nil {
		return nil, err
	}
	return cookies, nil
}
func googleCookieRecords(cookies []*network.Cookie) []aistudio.CookieRecord {
	result := []aistudio.CookieRecord{}
	for _, cookie := range cookies {
		domain := strings.TrimPrefix(strings.ToLower(cookie.Domain), ".")
		if domain == "google.com" || strings.HasSuffix(domain, ".google.com") {
			result = append(result, aistudio.CookieRecord{Name: cookie.Name, Value: cookie.Value})
		}
	}
	return result
}
func (s *ChromeSession) Spec() BrowserSpec {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.spec
}

func (s *ChromeSession) Fingerprint() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.fingerprint
}

func (s *ChromeSession) currentURL() string {
	var value string
	_ = chromedp.Run(s.ctx, chromedp.Location(&value))
	return value
}

func (s *ChromeSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.process.Stop()
}

// Reset discards the live page state. Prepare rebuilds it from the profile's
// persisted cookie source immediately after this method returns.
func (s *ChromeSession) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Start(); err != nil {
		return err
	}

	s.stateMu.Lock()
	s.runtime = aistudio.RuntimeConfig{}
	s.profile = nil
	s.cookies = nil
	s.fingerprint = ""
	s.stateMu.Unlock()
	s.headersMu.Lock()
	s.headers = map[string]string{}
	s.headersMu.Unlock()

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	return chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.ClearBrowserCookies().Do(ctx)
		}),
		chromedp.Navigate("about:blank"),
	)
}
func parseCookieHeader(header string) []aistudio.CookieRecord {
	result := []aistudio.CookieRecord{}
	for _, pair := range strings.Split(header, ";") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 && parts[0] != "" && !strings.HasPrefix(parts[0], "__Host-") {
			result = append(result, aistudio.CookieRecord{Name: parts[0], Value: parts[1]})
		}
	}
	return result
}

func requiredSessionCookie(name string) bool {
	return name == "SID" || name == "SAPISID" || name == "__Secure-1PAPISID" || name == "__Secure-3PAPISID"
}
func joinCookies(records []aistudio.CookieRecord) string {
	values := make([]string, 0, len(records))
	for _, record := range records {
		values = append(values, record.Name+"="+record.Value)
	}
	return strings.Join(values, "; ")
}
