package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
)

type turnPrepareResponse struct {
	Status       string `json:"status"`
	ConduitToken string `json:"conduit_token"`
}

func (c *Client) prepareTurn(
	ctx context.Context,
	artifacts Artifacts,
	payload turnPreparePayload,
) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal ChatGPT turn prepare payload: %w", err)
	}
	response, err := c.sendPrepare(ctx, artifacts, body)
	if err != nil {
		return "", fmt.Errorf("send ChatGPT turn prepare: %w", err)
	}
	defer response.Body.Close()
	data, err := readResponseBody(response)
	if err != nil {
		return "", fmt.Errorf("read ChatGPT turn prepare response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf(
			"ChatGPT turn prepare returned HTTP %d: %s",
			response.StatusCode,
			diagnosticBody(data, response.Header.Get("Content-Type")),
		)
	}
	var result turnPrepareResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode ChatGPT turn prepare response: %w", err)
	}
	if result.ConduitToken == "" {
		return "", fmt.Errorf("ChatGPT turn prepare returned no conduit token")
	}
	return result.ConduitToken, nil
}
