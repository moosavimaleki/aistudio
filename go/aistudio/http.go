package aistudio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
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
		transport.Proxy = http.ProxyURL(proxy)
	}
	return &HTTPClient{client: &http.Client{Transport: transport, Timeout: 90 * time.Second}}, nil
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
func ReadBody(response *http.Response) (string, error) {
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
