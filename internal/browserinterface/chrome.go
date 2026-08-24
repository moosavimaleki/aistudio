package browserinterface

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"github.com/hamed/aistudio-api/internal/chromeprocess"
)

type ChromeSession struct {
	spec          BrowserSpec
	port          int
	broker        *Broker
	process       *chromeprocess.Process
	browserCtx    context.Context
	browserCancel context.CancelFunc
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	stateMu       sync.RWMutex
	headersMu     sync.RWMutex
	primeMu       sync.Mutex
	primeResult   chan error
	fingerprint   string
	runtime       aistudio.RuntimeConfig
	profile       map[string]string
	cookies       []aistudio.CookieRecord
	headers       map[string]string
	providerReady bool
	chatRequests  sync.Map
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
	s.browserCtx, s.browserCancel = chromedp.NewRemoteAllocator(context.Background(), s.process.CDPURL())
	s.ctx, s.cancel = chromedp.NewContext(s.browserCtx)
	chromedp.ListenTarget(s.ctx, func(event any) {
		switch request := event.(type) {
		case *network.EventRequestWillBeSent:
			if path := chatConversationPath(request.Request.URL); path != "" {
				s.chatRequests.Store(request.RequestID, path)
				log.Printf("ChatGPT page conversation request browser=%s path=%s hasPostData=%t", s.spec.ID, path, request.Request.HasPostData)
			}
			if strings.Contains(request.Request.URL, "MakerSuiteService/") {
				s.captureHeaders(request.Request.Headers)
			}
		case *network.EventRequestWillBeSentExtraInfo:
			if headerValue(request.Headers, "x-client-data") != "" {
				s.captureHeaders(request.Headers)
			}
		case *fetch.EventRequestPaused:
			s.captureNativeGenerate(request)
		case *network.EventResponseReceived:
			if path := chatConversationPath(request.Response.URL); path != "" {
				s.chatRequests.Delete(request.RequestID)
				log.Printf("ChatGPT page conversation response browser=%s path=%s status=%d", s.spec.ID, path, request.Response.Status)
			}
		case *network.EventLoadingFailed:
			if value, ok := s.chatRequests.LoadAndDelete(request.RequestID); ok {
				log.Printf("ChatGPT page conversation failed browser=%s path=%s canceled=%t error=%s", s.spec.ID, value, request.Canceled, request.ErrorText)
			}
		}
	})
	return chromedp.Run(s.ctx, network.Enable())
}

func chatConversationPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsed.Path == "/backend-api/f/conversation" || parsed.Path == "/backend-anon/f/conversation" {
		return parsed.Path
	}
	return ""
}
func (s *ChromeSession) Prepare(cookies, authUser string) (map[string]any, error) {
	if s.spec.Provider != "" && s.spec.Provider != ProviderAIStudio {
		return nil, fmt.Errorf("browser %s is not an AI Studio profile", s.spec.ID)
	}
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

func (s *ChromeSession) PrepareChatGPT() error {
	return s.prepareChatGPT(true)
}

func (s *ChromeSession) prepareChatGPT(importCookies bool) error {
	if s.spec.Provider != ProviderChatGPT {
		return fmt.Errorf("browser %s is not a ChatGPT profile", s.spec.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Start(); err != nil {
		return err
	}
	if importCookies {
		if err := s.refreshChatGPTCookies(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
	defer cancel()
	navigationErr := chromedp.Run(ctx, chromedp.Navigate("https://chatgpt.com/"))
	if navigationErr != nil && !strings.Contains(navigationErr.Error(), "net::ERR_ABORTED") {
		return fmt.Errorf("open ChatGPT profile: %w", navigationErr)
	}
	if err := chromedp.Run(ctx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		return fmt.Errorf("open ChatGPT profile: %w", err)
	}
	if err := s.waitForChatGPTComposer(ctx); err != nil {
		return err
	}
	cookies, err := s.readCookiesForURL("https://chatgpt.com/")
	if err != nil {
		return err
	}
	records := make([]aistudio.CookieRecord, 0, len(cookies))
	for _, cookie := range cookies {
		records = append(records, aistudio.CookieRecord{Name: cookie.Name, Value: cookie.Value})
	}
	s.stateMu.Lock()
	s.cookies = records
	s.fingerprint = SessionFingerprint(joinCookies(records), ProviderChatGPT)
	s.providerReady = true
	s.stateMu.Unlock()
	return nil
}

func (s *ChromeSession) RestartChatGPT() error {
	s.MarkProviderUnready()
	s.Close()
	return s.prepareChatGPT(false)
}

func (s *ChromeSession) MarkProviderUnready() {
	s.stateMu.Lock()
	s.providerReady = false
	s.stateMu.Unlock()
}

func (s *ChromeSession) markProviderReady() {
	s.stateMu.Lock()
	s.providerReady = true
	s.stateMu.Unlock()
}

func (s *ChromeSession) refreshChatGPTCookies() error {
	if s.spec.ChatGPTCookieFile == "" {
		return nil
	}
	cookies, err := readNetscapeCookies(s.spec.ChatGPTCookieFile, "chatgpt.com")
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		params := network.SetCookie(cookie.Name, cookie.Value).
			WithPath(cookie.Path).
			WithSecure(cookie.Secure).
			WithHTTPOnly(cookie.HTTPOnly)
		if strings.HasPrefix(cookie.Name, "__Host-") {
			params = params.WithURL("https://chatgpt.com/")
		} else {
			params = params.WithDomain(cookie.Domain)
		}
		if err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			return params.Do(ctx)
		})); err != nil {
			return fmt.Errorf("Chrome did not apply ChatGPT cookie %s: %w", cookie.Name, err)
		}
	}
	return nil
}

func (s *ChromeSession) PressChatGPTEnter(submitNonce string) error {
	encodedNonce, err := json.Marshal(submitNonce)
	if err != nil {
		return err
	}
	// The extension may reload an existing ChatGPT tab before preparing the
	// composer. The session context owns that same tab and survives navigation.
	time.Sleep(2 * time.Second)
	predicate := fmt.Sprintf(`document.querySelector("#prompt-textarea")?.dataset.aistudioSubmitNonce === %s`, encodedNonce)
	// The extension can spend up to 45 seconds waiting for a reloaded ChatGPT
	// tab before it writes the nonce. Keep the trusted Enter step outside that
	// smaller browser-side deadline so a slow but healthy tab can finish.
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	probeLogged := false
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		var ready bool
		lastErr = chromedp.Run(ctx, chromedp.Evaluate(predicate, &ready))
		if !probeLogged {
			log.Printf("ChatGPT composer probe browser=%s composerContextReady=%t nonceMatches=%t error=%v", s.spec.ID, lastErr == nil, ready, lastErr)
			probeLogged = true
		}
		if lastErr == nil && ready {
			var clicked bool
			lastErr = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
				const button = document.querySelector('[data-testid="send-button"]');
				if (!button || button.disabled) return false;
				button.click();
				return true;
			})()`, &clicked))
			if lastErr == nil && !clicked {
				lastErr = fmt.Errorf("ChatGPT send button is unavailable")
			}
			if lastErr != nil {
				lastErr = chromedp.Run(ctx,
					chromedp.Focus("#prompt-textarea", chromedp.ByQuery),
					chromedp.KeyEvent(kb.Enter),
				)
			}
			if lastErr == nil {
				cancel()
				return nil
			}
		}
		cancel()
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = context.DeadlineExceeded
	}
	return fmt.Errorf("ChatGPT composer did not become ready: %w", lastErr)
}

func (s *ChromeSession) ChatGPTTransport() (map[string]any, error) {
	cookies, err := s.readCookiesForURL("https://chatgpt.com/")
	if err != nil {
		return nil, err
	}
	if s.spec.ChatGPTCookieFile != "" {
		if err := persistDomainCookieFile(s.spec.ChatGPTCookieFile, cookies, "chatgpt.com"); err != nil {
			return nil, fmt.Errorf("persist ChatGPT cookies: %w", err)
		}
	}
	values := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		values = append(values, cookie.Name+"="+cookie.Value)
	}
	var identity struct {
		UserAgent string `json:"userAgent"`
	}
	if err := chromedp.Run(s.ctx, chromedp.Evaluate(`({userAgent:navigator.userAgent})`, &identity)); err != nil {
		return nil, err
	}
	return map[string]any{
		"cookies":   strings.Join(values, "; "),
		"userAgent": identity.UserAgent,
	}, nil
}

func (s *ChromeSession) Snapshot() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}
func (s *ChromeSession) Ready() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.spec.Provider == ProviderChatGPT {
		return s.providerReady
	}
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
	return s.readCookiesForURL("https://aistudio.google.com/")
}

func (s *ChromeSession) readCookiesForURL(rawURL string) ([]*network.Cookie, error) {
	return s.readCookiesForURLContext(s.ctx, rawURL)
}

func (s *ChromeSession) readCookiesForURLContext(base context.Context, rawURL string) ([]*network.Cookie, error) {
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	var cookies []*network.Cookie
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var readErr error
		cookies, readErr = network.GetCookies().WithURLs([]string{rawURL}).Do(ctx)
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
	if s.browserCancel != nil {
		s.browserCancel()
		s.browserCancel = nil
		s.browserCtx = nil
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
	s.providerReady = false
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
