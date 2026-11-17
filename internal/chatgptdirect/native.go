package chatgptdirect

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// NativeConfig configures authentication, transport, and the Black Box runtime.
// Exactly one of SessionFile and CookieDir must be provided.
type NativeConfig struct {
	SessionFile    string
	CookieDir      string
	ProxyURL       string
	BlackBoxScript string
	BaseURL        string
}

// NewNativeClient creates a direct HTTP client. SessionFile accepts a version-1
// session snapshot; CookieDir accepts Netscape cookie exports.
func NewNativeClient(config NativeConfig) (*Client, error) {
	if (config.SessionFile == "") == (config.CookieDir == "") {
		return nil, fmt.Errorf("provide exactly one of session file or cookie directory")
	}
	origin := strings.TrimRight(config.BaseURL, "/")
	if origin == "" {
		origin = upstreamOrigin
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("invalid ChatGPT upstream origin")
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("ChatGPT upstream must be an origin without credentials or path")
	}
	if parsed.Scheme != "https" {
		ip := net.ParseIP(parsed.Hostname())
		if parsed.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("upstream requires https; http is allowed only for loopback test servers")
		}
	}
	httpClient, err := newBrowserHTTP(config.ProxyURL)
	if err != nil {
		return nil, err
	}
	blackBox := newBlackBoxClient(httpClient, config.CookieDir, origin)
	script := config.BlackBoxScript
	if script == "" {
		script = blackBoxScriptPath()
	}
	blackBox.runtime = newBlackBoxRunner(script)
	if config.SessionFile != "" {
		if _, err := loadSavedHeaders(config.SessionFile); err != nil {
			return nil, err
		}
		blackBox.sessionFile = config.SessionFile
	}
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("black box runtime runner not found: %w", err)
	}
	return &Client{http: httpClient, blackBox: blackBox, origin: origin, nativeGate: make(chan struct{}, 1), parents: map[string]string{}}, nil
}
