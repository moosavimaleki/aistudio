package browserinterface

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/hamed/aistudio-api/go/aistudio"
)

type ChromeSession struct {
	spec        BrowserSpec
	port        int
	broker      *Broker
	process     *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	fingerprint string
	runtime     aistudio.RuntimeConfig
	profile     map[string]string
	cookies     []aistudio.CookieRecord
	headers     map[string]string
}

func NewChromeSession(spec BrowserSpec, port int, broker *Broker) *ChromeSession {
	return &ChromeSession{spec: spec, port: port, broker: broker, headers: map[string]string{}}
}
func (s *ChromeSession) Start() error {
	if s.process != nil {
		return nil
	}
	profile := filepath.Join(defaultValue(os.Getenv("CHROME_RUNTIME_DIR"), "/app/browser-profiles"), "profiles", s.spec.ID)
	extension := filepath.Join(defaultValue(os.Getenv("CHROME_RUNTIME_DIR"), "/app/browser-profiles"), "extensions", s.spec.ID)
	if err := os.MkdirAll(profile, 0755); err != nil {
		return err
	}
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(profile, name))
	}
	if err := copyExtension(defaultValue(os.Getenv("EXTENSION_SOURCE_DIR"), "/app/extension"), extension, s.spec.ID); err != nil {
		return err
	}
	args := []string{"--no-sandbox", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-search-engine-choice-screen", "--remote-debugging-address=127.0.0.1", fmt.Sprintf("--remote-debugging-port=%d", s.port), "--user-data-dir=" + profile, "--disable-extensions-except=" + extension, "--load-extension=" + extension, "about:blank"}
	if proxy := os.Getenv("LAB_PROXY_URL"); proxy != "" {
		args = append(args, "--proxy-server="+proxy, "--proxy-bypass-list=127.0.0.1;localhost;<-loopback>")
	}
	s.process = exec.Command(defaultValue(os.Getenv("CHROME_EXECUTABLE"), "/usr/bin/google-chrome"), args...)
	if err := s.process.Start(); err != nil {
		return err
	}
	for attempt := 0; attempt < 100; attempt++ {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.port), 200*time.Millisecond)
		if err == nil {
			connection.Close()
			break
		}
		if attempt == 99 {
			return fmt.Errorf("Chrome %s CDP endpoint did not become ready", s.spec.ID)
		}
		time.Sleep(100 * time.Millisecond)
	}
	allocator, allocatorCancel := chromedp.NewRemoteAllocator(context.Background(), fmt.Sprintf("http://127.0.0.1:%d", s.port))
	s.ctx, s.cancel = chromedp.NewContext(allocator)
	chromedp.ListenTarget(s.ctx, func(event any) {
		if request, ok := event.(*network.EventRequestWillBeSent); ok && strings.Contains(request.Request.URL, "MakerSuiteService/") {
			headers := map[string]string{}
			for name, value := range request.Request.Headers {
				headers[strings.ToLower(name)] = fmt.Sprint(value)
			}
			s.mu.Lock()
			s.headers = headers
			s.mu.Unlock()
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
	if s.fingerprint == fingerprint && s.runtime.APIKey != "" {
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
	var html string
	if err := chromedp.Run(s.ctx, chromedp.Navigate(endpoint), chromedp.OuterHTML("html", &html)); err != nil {
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
	s.runtime = runtime
	s.profile = map[string]string{"Accept": "*/*", "Accept-Language": "en-US,en;q=0.9,fa;q=0.8", "User-Agent": browser.UserAgent, "X-Client-Data": upstream.Opaque["x_client_data"]}
	s.cookies, err = s.readCookies()
	if err != nil {
		return nil, err
	}
	s.fingerprint = SessionFingerprint(joinCookies(s.cookies), authUser)
	return s.snapshotLocked()
}
func (s *ChromeSession) Snapshot() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}
func (s *ChromeSession) snapshotLocked() (map[string]any, error) {
	if s.runtime.APIKey == "" {
		return nil, fmt.Errorf("Container browser session is not ready")
	}
	cookies, err := s.readCookies()
	if err != nil {
		return nil, err
	}
	s.cookies = cookies
	runtime := map[string]any{"apiKey": s.runtime.APIKey, "visitId": s.runtime.VisitID, "authUser": s.runtime.AuthUser, "attestationEnabled": s.runtime.AttestationEnabled}
	return map[string]any{"runtimeConfig": runtime, "transportProfile": s.profile, "cookieRecords": cookies}, nil
}
func (s *ChromeSession) readCookies() ([]aistudio.CookieRecord, error) {
	var cookies []*network.Cookie
	err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var readErr error
		cookies, readErr = network.GetCookies().WithUrls([]string{"https://aistudio.google.com/"}).Do(ctx)
		return readErr
	}))
	if err != nil {
		return nil, err
	}
	result := []aistudio.CookieRecord{}
	for _, cookie := range cookies {
		domain := strings.TrimPrefix(strings.ToLower(cookie.Domain), ".")
		if domain == "google.com" || strings.HasSuffix(domain, ".google.com") {
			result = append(result, aistudio.CookieRecord{Name: cookie.Name, Value: cookie.Value})
		}
	}
	return result, nil
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
	if s.process != nil && s.process.Process != nil {
		_ = s.process.Process.Kill()
		_, _ = s.process.Process.Wait()
		s.process = nil
	}
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
func copyExtension(source, target, id string) error {
	_ = os.RemoveAll(target)
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
}
func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var _ = json.RawMessage{}
