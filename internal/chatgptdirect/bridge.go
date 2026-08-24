package chatgptdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type bridgeClient struct {
	endpoint string
	http     *http.Client
}

type prepareRequest struct {
	BrowserID string `json:"browserId"`
}

func newBridgeClient(factoryOrigin string) *bridgeClient {
	return &bridgeClient{
		endpoint: strings.TrimRight(factoryOrigin, "/") + "/internal/chatgpt/prepare",
		http:     &http.Client{Timeout: 245 * time.Second},
	}
}

func (b *bridgeClient) prepare(ctx context.Context, input prepareRequest) (Artifacts, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return Artifacts{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(body))
	if err != nil {
		return Artifacts{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := b.http.Do(request)
	if err != nil {
		return Artifacts{}, err
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
		return Artifacts{}, &BridgeError{Status: response.StatusCode, Message: failure.Error}
	}
	result := Artifacts{
		Headers:        map[string]string{},
		PrepareHeaders: map[string]string{},
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Artifacts{}, err
	}
	missing := make([]string, 0, 2)
	if len(result.Headers) == 0 {
		missing = append(missing, "final headers")
	}
	if result.Cookies == "" {
		missing = append(missing, "cookies")
	}
	if len(missing) > 0 {
		return Artifacts{}, fmt.Errorf(
			"ChatGPT browser preparation returned incomplete transport artifacts: missing %s",
			strings.Join(missing, ", "),
		)
	}
	return result, nil
}
