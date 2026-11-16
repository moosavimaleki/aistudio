package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"


	tlsclient "github.com/bogdanfinn/tls-client"
)

const upstreamOrigin = "https://chatgpt.com"

type Client struct {
	bridge   *bridgeClient
	sentinel *sentinelClient
	http     tlsclient.HttpClient
	origin   string
}

func NewClient(factoryOrigin, proxyURL string) (*Client, error) {
	httpClient, err := newBrowserHTTP(proxyURL)
	if err != nil {
		return nil, err
	}
	client := &Client{
		bridge: newBridgeClient(factoryOrigin),
		http:   httpClient,
	}
	if nativeSentinelEnabled() {
		client.sentinel = newSentinelClient(httpClient, os.Getenv("CHATGPT_COOKIE_DIR"), upstreamOrigin)
	}
	return client, nil
}

// nativeSentinelEnabled reports whether the cookie-auth native path is requested
// via CHATGPT_DIRECT_NATIVE. When set, the browser/extension bridge is bypassed.
func nativeSentinelEnabled() bool {
	return os.Getenv("CHATGPT_DIRECT_NATIVE") == "true" || os.Getenv("CHATGPT_DIRECT_NATIVE") == "1"
}

// prepareArtifacts returns browser-ready Artifacts for browserID. When the
// native sentinel path is enabled it produces the Sentinel tokens from the
// stored cookie; otherwise it falls back to the browser/extension bridge.
func (c *Client) prepareArtifacts(ctx context.Context, browserID string) (Artifacts, error) {
	if c.sentinel != nil {
		return c.sentinel.prepare(ctx, browserID)
	}
	return c.bridge.prepare(ctx, prepareRequest{BrowserID: browserID})
}

func (c *Client) Generate(ctx context.Context, input Input) (Result, error) {
	model, err := ResolveModel(input.Model)
	if err != nil {
		return Result{}, err
	}
	if input.ConversationID != "" && input.ParentMessageID == "" {
		if c.sentinel == nil {
			return Result{}, fmt.Errorf("parent_message_id is required when conversation_id is set")
		}
		history, historyErr := c.conversation(ctx, input.ConversationID)
		if historyErr != nil {
			return Result{}, historyErr
		}
		input.ParentMessageID = history.CurrentNode
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
	artifacts, err := c.prepareArtifacts(ctx, input.BrowserID)
	if err != nil {
		return Result{}, err
	}
	payload, _, err := buildPayload(input, model, parentMessageID, artifacts.Context)
	if err != nil {
		return Result{}, err
	}
	if c.sentinel != nil {
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
		conduitToken, prepareErr := c.prepareTurn(ctx, artifacts, preparePayload)
		if prepareErr != nil {
			return Result{}, prepareErr
		}
		artifacts.Headers["x-conduit-token"] = conduitToken
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	response, path, err := c.send(ctx, artifacts, body)
	if err != nil {
		return Result{}, err
	}
	defer closeReadBody(response.Body)
	data, err := readResponseBody(response)
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
	parsed := parseEventStream(data)
	if parsed.Error != "" {
		return Result{}, fmt.Errorf("ChatGPT direct stream error: %s", parsed.Error)
	}
	if parsed.ConversationID == "" {
		parsed.ConversationID = input.ConversationID
	}
	if c.sentinel != nil {
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
