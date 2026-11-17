package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hamed/aistudio-api/internal/metrics"

	tlsclient "github.com/bogdanfinn/tls-client"
)

const upstreamOrigin = "https://chatgpt.com"

type Client struct {
	bridge     *bridgeClient
	blackBox   *blackBoxClient
	http       tlsclient.HttpClient
	origin     string
	nativeGate chan struct{}
	parents    map[string]string // accessed only while nativeGate is held
}

func NewClient(factoryOrigin, proxyURL string) (*Client, error) {
	if directModeEnabled() {
		return NewNativeClient(NativeConfig{
			SessionFile: os.Getenv("CHATGPT_SESSION_FILE"),
			CookieDir:   nativeCookieDirectory(), ProxyURL: proxyURL,
			BlackBoxScript: os.Getenv("CHATGPT_BLACK_BOX_SCRIPT"),
			BaseURL:        os.Getenv("CHATGPT_UPSTREAM_ORIGIN"),
		})
	}
	httpClient, err := newBrowserHTTP(proxyURL)
	if err != nil {
		return nil, err
	}
	client := &Client{
		bridge: newBridgeClient(factoryOrigin),
		http:   httpClient,
	}
	return client, nil
}

// directModeEnabled selects the HTTP client and local Black Box runtime.
// CHATGPT_DIRECT_NATIVE is accepted when CHATGPT_MODE is unset.
func directModeEnabled() bool {
	mode := os.Getenv("CHATGPT_MODE")
	if mode != "" {
		return mode == "direct"
	}
	return os.Getenv("CHATGPT_DIRECT_NATIVE") == "true" || os.Getenv("CHATGPT_DIRECT_NATIVE") == "1"
}

func nativeCookieDirectory() string {
	if os.Getenv("CHATGPT_SESSION_FILE") != "" {
		return ""
	}
	return os.Getenv("CHATGPT_COOKIE_DIR")
}

// prepareArtifacts obtains per-request credentials from the configured provider.
func (c *Client) prepareArtifacts(ctx context.Context, browserID string) (Artifacts, error) {
	if c.blackBox != nil {
		return c.blackBox.prepare(ctx, browserID)
	}
	return c.bridge.prepare(ctx, prepareRequest{BrowserID: browserID})
}

func (c *Client) Generate(ctx context.Context, input Input) (Result, error) {
	if c.nativeGate != nil {
		queued := metrics.Phase(ctx, "direct_queue")
		select {
		case c.nativeGate <- struct{}{}:
			queued()
			defer func() { <-c.nativeGate }()
		case <-ctx.Done():
			queued()
			return Result{}, ctx.Err()
		}
	}
	model, err := ResolveModel(input.Model)
	if err != nil {
		return Result{}, err
	}
	if input.ConversationID != "" && input.ParentMessageID == "" {
		if c.blackBox == nil {
			return Result{}, fmt.Errorf("parent_message_id is required when conversation_id is set")
		}
		input.ParentMessageID = c.parents[input.BrowserID+"\x00"+input.ConversationID]
		if input.ParentMessageID == "" {
			history, historyErr := c.conversation(ctx, input.ConversationID, input.BrowserID)
			if historyErr != nil {
				return Result{}, historyErr
			}
			input.ParentMessageID = history.CurrentNode
		}
	}
	prompt, err := latestUserPrompt(input.Messages)
	if err != nil {
		return Result{}, err
	}
	parentMessageID := input.ParentMessageID
	if parentMessageID == "" {
		parentMessageID, err = newUUID()
		if err != nil {
			return Result{}, err
		}
	}
	return c.generateOnce(ctx, input, model, prompt, parentMessageID)
}

func (c *Client) generateOnce(
	ctx context.Context,
	input Input,
	model Model,
	prompt string,
	parentMessageID string,
) (Result, error) {
	prepared := metrics.Phase(ctx, "gpt_artifacts")
	artifacts, err := c.prepareArtifacts(ctx, input.BrowserID)
	prepared()
	if err != nil {
		return Result{}, err
	}
	payload, _, err := buildPayload(input, model, parentMessageID, artifacts.Context)
	if err != nil {
		return Result{}, err
	}
	if c.blackBox != nil {
		// Match the verified direct protocol: full SSE snapshots, no browser
		// conduit preparation and no dependency on the frontend patch codec.
		payload.SupportsBuffering = false
		payload.SupportedEncodings = []string{}
	}
	if len(artifacts.PrepareHeaders) > 0 {
		preparePayload, prepareErr := buildTurnPreparePayload(payload)
		if prepareErr != nil {
			return Result{}, prepareErr
		}
		turnTraceID, traceErr := newUUID()
		if traceErr != nil {
			return Result{}, fmt.Errorf("create ChatGPT turn trace ID: %w", traceErr)
		}
		artifacts.Headers["x-oai-turn-trace-id"] = turnTraceID
		artifacts.PrepareHeaders["x-oai-turn-trace-id"] = turnTraceID
		turnPrepared := metrics.Phase(ctx, "gpt_turn_prepare")
		conduitToken, prepareErr := c.prepareTurn(ctx, artifacts, preparePayload)
		turnPrepared()
		if prepareErr != nil {
			return Result{}, prepareErr
		}
		artifacts.Headers["x-conduit-token"] = conduitToken
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	headersReceived := metrics.Phase(ctx, "gpt_upstream_headers")
	response, path, err := c.send(ctx, artifacts, body)
	headersReceived()
	if err != nil {
		return Result{}, err
	}
	defer closeReadBody(response.Body)
	bodyRead := metrics.Phase(ctx, "gpt_response_body")
	data, err := readResponseBody(response)
	bodyRead()
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		contentType := response.Header.Get("Content-Type")
		return Result{}, &UpstreamError{
			Status:      response.StatusCode,
			Body:        diagnosticBody(data, contentType),
			BrowserID:   artifacts.BrowserID,
			ContentType: contentType,
			CFMitigated: response.Header.Get("Cf-Mitigated"),
		}
	}
	parsedDone := metrics.Phase(ctx, "gpt_parse")
	parsed := parseEventStream(data)
	parsedDone()
	if parsed.Error != "" {
		return Result{}, fmt.Errorf("ChatGPT direct stream error: %s", parsed.Error)
	}
	if parsed.ConversationID == "" {
		parsed.ConversationID = input.ConversationID
	}
	if c.blackBox != nil {
		return c.confirmTurn(ctx, parsed, payload, input.Model, artifacts.BrowserID)
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return Result{}, fmt.Errorf(
			"ChatGPT direct response contained no assistant text (%s)",
			responseShape(response, data),
		)
	}
	return Result{
		Text:            parsed.Text,
		ConversationID:  parsed.ConversationID,
		ParentMessageID: parsed.MessageID,
		BrowserID:       artifacts.BrowserID,
		Model:           input.Model,
		UpstreamStatus:  response.StatusCode,
		UpstreamPath:    path,
	}, nil
}

func latestUserPrompt(messages []Message) (string, error) {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" && strings.TrimSpace(messages[index].Content) != "" {
			return messages[index].Content, nil
		}
	}
	return "", fmt.Errorf("a non-empty user message is required")
}

func diagnosticBody(data []byte, contentType string) string {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return "upstream returned an HTML challenge page"
	}
	const limit = 2_048
	if len(data) > limit {
		data = data[:limit]
	}
	return strings.TrimSpace(string(data))
}
