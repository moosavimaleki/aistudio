package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
)

var rpcIdentityHeaders = map[string]bool{
	"accept-language": true, "user-agent": true,
	"sec-ch-ua": true, "sec-ch-ua-arch": true, "sec-ch-ua-bitness": true,
	"sec-ch-ua-form-factors": true, "sec-ch-ua-full-version": true,
	"sec-ch-ua-full-version-list": true, "sec-ch-ua-mobile": true,
	"sec-ch-ua-model": true, "sec-ch-ua-platform": true,
	"sec-ch-ua-platform-version": true, "sec-ch-ua-wow64": true,
	"x-browser-channel": true, "x-browser-copyright": true,
	"x-browser-validation": true, "x-browser-year": true, "x-client-data": true,
}

func (s *ChromeSession) waitForRPCIdentity(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.observedHeaders()["x-client-data"] != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("container Chrome did not expose X-Client-Data from an AI Studio RPC: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *ChromeSession) observedTransportProfile() map[string]string {
	result := map[string]string{}
	for name, value := range s.observedHeaders() {
		if rpcIdentityHeaders[name] && value != "" {
			result[name] = value
		}
	}
	return result
}

func (s *ChromeSession) observedHeaders() map[string]string {
	s.headersMu.RLock()
	defer s.headersMu.RUnlock()
	result := make(map[string]string, len(s.headers))
	for name, value := range s.headers {
		result[strings.ToLower(name)] = value
	}
	return result
}

func (s *ChromeSession) captureHeaders(source network.Headers) {
	s.headersMu.Lock()
	defer s.headersMu.Unlock()
	for name, value := range source {
		s.headers[strings.ToLower(name)] = fmt.Sprint(value)
	}
}

func headerValue(headers network.Headers, target string) string {
	for name, value := range headers {
		if strings.EqualFold(name, target) {
			return fmt.Sprint(value)
		}
	}
	return ""
}
