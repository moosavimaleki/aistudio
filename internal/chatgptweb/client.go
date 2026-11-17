package chatgptweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/metrics"
)

type Result struct {
	Text           string  `json:"text"`
	Images         []Image `json:"images"`
	ConversationID string  `json:"conversationId"`
	BrowserID      string  `json:"browserId"`
	UpstreamStatus int     `json:"upstreamStatus"`
	UpstreamPath   string  `json:"upstreamPath"`
}

type Image struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type Attachment struct {
	Name     string `json:"name"`
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

type BridgeError struct {
	Status  int
	Message string
}

func (e *BridgeError) Error() string {
	return fmt.Sprintf("ChatGPT browser bridge returned HTTP %d: %s", e.Status, e.Message)
}

func NewClient(factoryOrigin string) *Client {
	return &Client{
		endpoint: strings.TrimRight(factoryOrigin, "/") + "/internal/chatgpt/generate",
		http:     &http.Client{Timeout: 420 * time.Second},
	}
}

func (c *Client) Generate(ctx context.Context, prompt, browserID string) (Result, error) {
	return c.generate(ctx, prompt, browserID, false, nil)
}

func (c *Client) GenerateImage(ctx context.Context, prompt, browserID string, attachments []Attachment) (Result, error) {
	return c.generate(ctx, prompt, browserID, true, attachments)
}

func (c *Client) generate(ctx context.Context, prompt, browserID string, image bool, attachments []Attachment) (Result, error) {
	defer metrics.Phase(ctx, "gpt_web_bridge")()
	body, err := json.Marshal(map[string]any{"prompt": prompt, "browserId": browserID, "image": image, "attachments": attachments})
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Lab-Request-ID", metrics.RequestID(ctx))
	response, err := c.http.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	metrics.ImportPhases(ctx, "browser_", response.Header.Get("X-Lab-Timings"))
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = http.StatusText(response.StatusCode)
		}
		return Result{}, &BridgeError{Status: response.StatusCode, Message: failure.Error}
	}
	var result Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Result{}, err
	}
	return result, nil
}
