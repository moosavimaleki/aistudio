package chatgptdirect

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	conversationPath = "/backend-api/f/conversation"
	preparePath      = conversationPath + "/prepare"
)

func newBrowserHTTP(proxyURL string) (tlsclient.HttpClient, error) {
	options := []tlsclient.HttpClientOption{
		tlsclient.WithClientProfile(profiles.Chrome_133),
		tlsclient.WithRandomTLSExtensionOrder(),
		tlsclient.WithTimeoutSeconds(240),
		tlsclient.WithNotFollowRedirects(),
	}
	if proxyURL != "" {
		options = append(options, tlsclient.WithProxyUrl(proxyURL))
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("create Chrome-compatible ChatGPT transport: %w", err)
	}
	return client, nil
}

func (c *Client) send(
	ctx context.Context,
	artifacts Artifacts,
	body []byte,
) (*fhttp.Response, string, error) {
	path := artifacts.UpstreamPath
	if path == "" {
		path = conversationPath
	}
	if path != conversationPath {
		return nil, "", fmt.Errorf("unexpected ChatGPT upstream path: %s", path)
	}
	request, err := newUpstreamRequest(ctx, path, artifacts.Headers, artifacts, body, "text/event-stream")
	if err != nil {
		return nil, "", err
	}
	response, err := c.http.Do(request)
	return response, path, err
}

func (c *Client) sendPrepare(
	ctx context.Context,
	artifacts Artifacts,
	body []byte,
) (*fhttp.Response, error) {
	request, err := newUpstreamRequest(
		ctx,
		preparePath,
		artifacts.PrepareHeaders,
		artifacts,
		body,
		"*/*",
	)
	if err != nil {
		return nil, err
	}
	return c.http.Do(request)
}

func newUpstreamRequest(
	ctx context.Context,
	path string,
	headers map[string]string,
	artifacts Artifacts,
	body []byte,
	accept string,
) (*fhttp.Request, error) {
	request, err := fhttp.NewRequestWithContext(
		ctx,
		fhttp.MethodPost,
		upstreamOrigin+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		if !blockedHeader(name) {
			request.Header.Set(name, value)
		}
	}
	setBrowserHeaders(request, artifacts, accept)
	setHeaderOrder(request, path)
	return request, nil
}

func setBrowserHeaders(request *fhttp.Request, artifacts Artifacts, accept string) {
	request.Header.Set("Accept", accept)
	request.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", artifacts.Cookies)
	request.Header.Set("Origin", upstreamOrigin)
	request.Header.Set("Referer", upstreamOrigin+"/")
	request.Header.Set("Priority", "u=1, i")
	request.Header.Set("Sec-Fetch-Dest", "empty")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	setHeader(request, "Accept-Language", artifacts.Context.AcceptLanguage)
	setHeader(request, "Sec-CH-UA", artifacts.Context.SecCHUA)
	setHeader(request, "Sec-CH-UA-Mobile", artifacts.Context.SecCHUAMobile)
	setHeader(request, "Sec-CH-UA-Platform", quoteHeader(artifacts.Context.SecCHUAPlatform))
	setHeader(request, "User-Agent", artifacts.UserAgent)
}

func setHeaderOrder(request *fhttp.Request, path string) {
	request.Header[fhttp.PHeaderOrderKey] = []string{
		":method", ":authority", ":scheme", ":path",
	}
	order := []string{
		"accept",
		"accept-encoding",
		"accept-language",
		"content-type",
		"oai-client-build-number",
		"oai-client-version",
		"oai-device-id",
		"oai-language",
		"oai-session-id",
	}
	if path == conversationPath {
		order = append(order,
			"oai-echo-logs",
			"oai-telemetry",
			"openai-sentinel-chat-requirements-token",
			"openai-sentinel-proof-token",
			"openai-sentinel-turnstile-token",
		)
	}
	order = append(order,
		"origin",
		"priority",
		"referer",
		"sec-ch-ua",
		"sec-ch-ua-mobile",
		"sec-ch-ua-platform",
		"sec-fetch-dest",
		"sec-fetch-mode",
		"sec-fetch-site",
		"user-agent",
	)
	if path == conversationPath {
		order = append(order, "x-conduit-token")
	}
	order = append(order,
		"x-oai-is-client-observation",
		"x-oai-turn-trace-id",
		"x-openai-target-path",
		"x-openai-target-route",
		"cookie",
	)
	request.Header[fhttp.HeaderOrderKey] = order
}

func setHeader(request *fhttp.Request, name, value string) {
	if value != "" {
		request.Header.Set(name, value)
	}
}

func quoteHeader(value string) string {
	if value == "" || strings.HasPrefix(value, "\"") {
		return value
	}
	return "\"" + strings.ReplaceAll(value, "\"", "") + "\""
}

func blockedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "accept-encoding", "connection", "content-length", "cookie", "host", "proxy-connection", "transfer-encoding":
		return true
	default:
		return false
	}
}
