package chatgptweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Text           string `json:"text"`
	ConversationID string `json:"conversationId"`
	BrowserID      string `json:"browserId"`
	UpstreamStatus int    `json:"upstreamStatus"`
	UpstreamPath   string `json:"upstreamPath"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(factoryOrigin string) *Client {
	return &Client{
		endpoint: strings.TrimRight(factoryOrigin, "/") + "/internal/chatgpt/generate",
		http:     &http.Client{Timeout: 245 * time.Second},
	}
}

func (c *Client) Generate(ctx context.Context, prompt, browserID string) (Result, error) {
	body, err := json.Marshal(map[string]string{"prompt": prompt, "browserId": browserID})
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = http.StatusText(response.StatusCode)
		}
		return Result{}, fmt.Errorf("ChatGPT browser bridge returned HTTP %d: %s", response.StatusCode, failure.Error)
	}
	var result Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Result{}, err
	}
	return result, nil
}
