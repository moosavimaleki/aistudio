package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"strings"

	tlsclient "github.com/bogdanfinn/tls-client"
)

const upstreamOrigin = "https://chatgpt.com"

type Client struct {
	bridge *bridgeClient
	http   tlsclient.HttpClient
}

func NewClient(factoryOrigin, proxyURL string) (*Client, error) {
	httpClient, err := newBrowserHTTP(proxyURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		bridge: newBridgeClient(factoryOrigin),
		http:   httpClient,
	}, nil
}

func (c *Client) Generate(ctx context.Context, input Input) (Result, error) {
	model, err := ResolveModel(input.Model)
	if err != nil {
		return Result{}, err
	}
	if input.ConversationID != "" && input.ParentMessageID == "" {
		return Result{}, fmt.Errorf("parent_message_id is required when conversation_id is set")
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
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, requestErr := c.generateOnce(ctx, input, model, prompt, parentMessageID)
		if requestErr == nil {
			return result, nil
		}
		lastErr = requestErr
		upstream, ok := requestErr.(*UpstreamError)
		if !ok || (upstream.Status != stdhttp.StatusUnauthorized && upstream.Status != stdhttp.StatusForbidden) {
			break
		}
	}
	return Result{}, lastErr
}

func (c *Client) generateOnce(
	ctx context.Context,
	input Input,
	model Model,
	prompt string,
	parentMessageID string,
) (Result, error) {
	artifacts, err := c.bridge.prepare(ctx, prepareRequest{
		BrowserID: input.BrowserID,
	})
	if err != nil {
		return Result{}, err
	}
	payload, _, err := buildPayload(input, model, parentMessageID, artifacts.Context)
	if err != nil {
		return Result{}, err
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
	defer response.Body.Close()
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
