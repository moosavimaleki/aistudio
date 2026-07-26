package aistudio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPClient struct{ client *http.Client }

func NewHTTPClient(proxyURL string) (*HTTPClient, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			if localHost(request.URL.Hostname()) {
				return nil, nil
			}
			return proxy, nil
		}
	}
	return &HTTPClient{client: &http.Client{Transport: transport, Timeout: 90 * time.Second}}, nil
}

func localHost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || !strings.Contains(host, ".")
}

func (h *HTTPClient) Request(ctx context.Context, method, endpoint string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return h.client.Do(req)
}

func (h *HTTPClient) RequestRetry(
	ctx context.Context,
	method, endpoint string,
	headers map[string]string,
	body []byte,
	retries int,
) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		response, err := h.Request(ctx, method, endpoint, headers, body)
		if err == nil && !retryableStatus(response.StatusCode) {
			return response, nil
		}
		if err == nil {
			if attempt == retries {
				return response, nil
			}
			response.Body.Close()
		} else {
			lastErr = err
		}
		if attempt == retries {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 150 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}
func ReadBody(response *http.Response) (string, error) {
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
