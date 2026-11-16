package chatgptdirect

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// NativeConfig selects authentication and the SDK runner for direct HTTP chat.
// Exactly one of SessionFile and CookieDir must be provided. No browser or
// Token Factory is contacted by this client.
type NativeConfig struct {
	SessionFile    string
	CookieDir      string
	ProxyURL       string
	SentinelScript string
	BaseURL        string
}

// NewNativeClient creates a direct client with no browser fallback. SessionFile
// accepts the version-1 snapshot produced by chatgpt-web; CookieDir accepts the
// project's existing Netscape cookie exports.
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
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.User != nil {
		return nil, fmt.Errorf("ChatGPT upstream must be an origin without credentials or path")
	}
	httpClient, err := newBrowserHTTP(config.ProxyURL)
	if err != nil {
		return nil, err
	}
	sentinel := newSentinelClient(httpClient, config.CookieDir, origin)
	script := config.SentinelScript
	if script == "" {
		script = sentinelSidecarPath()
	}
	sentinel.node = newSentinelNodeRunner(script)
	if config.SessionFile != "" {
		if _, err := loadSavedHeaders(config.SessionFile); err != nil {
			return nil, err
		}
		sentinel.sessionFile = config.SessionFile
	}
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("sentinel SDK runner not found: %w", err)
	}
	return &Client{http: httpClient, sentinel: sentinel, origin: origin}, nil
}
