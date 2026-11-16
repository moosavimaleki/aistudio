package chatgptdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	fhttp "github.com/bogdanfinn/fhttp"
)

func (s *sentinelClient) postPrepare(ctx context.Context, headers map[string]string, initial string) (prepareResponse, error) {
	var raw json.RawMessage
	err := s.postSentinel(ctx, headers, "/backend-api/sentinel/chat-requirements/prepare", map[string]string{"p": initial}, &raw)
	if err != nil {
		return prepareResponse{}, err
	}
	var result prepareResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return prepareResponse{}, fmt.Errorf("sentinel prepare returned invalid requirements")
	}
	if result.ForceLogin {
		return prepareResponse{}, fmt.Errorf("ChatGPT requires a fresh authenticated session")
	}
	if result.PrepareToken == "" && result.Token == "" {
		return prepareResponse{}, fmt.Errorf("sentinel prepare returned no token")
	}
	if result.Turnstile.Required && result.Turnstile.DX == "" {
		return prepareResponse{}, fmt.Errorf("sentinel requested VM evidence without a payload")
	}
	result.raw = raw
	return result, nil
}

func (s *sentinelClient) postFinalize(
	ctx context.Context,
	headers map[string]string,
	prepare prepareResponse,
	proof, turnstile string,
) (string, error) {
	if prepare.Token != "" {
		return prepare.Token, nil
	}
	body := map[string]string{"prepare_token": prepare.PrepareToken}
	if proof != "" {
		body["proofofwork"] = proof
	}
	if turnstile != "" {
		body["turnstile"] = turnstile
	}
	var result struct {
		Token      string `json:"token"`
		ForceLogin bool   `json:"force_login"`
	}
	if err := s.postSentinel(ctx, headers, "/backend-api/sentinel/chat-requirements/finalize", body, &result); err != nil {
		return "", err
	}
	if result.ForceLogin || result.Token == "" {
		return "", fmt.Errorf("sentinel finalize returned no usable token")
	}
	return result.Token, nil
}

func (s *sentinelClient) postSentinel(
	ctx context.Context,
	headers map[string]string,
	path string,
	input, output any,
) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode Sentinel request: %w", err)
	}
	request, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodPost, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Sentinel request: %w", err)
	}
	for name, value := range headers {
		if !blockedHeader(name) {
			request.Header.Set(name, value)
		}
	}
	request.Header.Set("Cookie", headers["cookie"])
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Encoding", "gzip, deflate")
	request.Header.Set("X-OpenAI-Target-Path", path)
	request.Header.Set("X-OpenAI-Target-Route", path)
	response, err := s.http.Do(request)
	if err != nil {
		return fmt.Errorf("sentinel request %s: %w", path, err)
	}
	defer closeReadBody(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sentinel %s returned HTTP %d", path, response.StatusCode)
	}
	data, err := readResponseBody(response)
	if err != nil {
		return fmt.Errorf("read Sentinel response: %w", err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("sentinel %s returned invalid JSON", path)
	}
	return nil
}

type prepareResponse struct {
	PrepareToken string `json:"prepare_token"`
	Token        string `json:"token"`
	ForceLogin   bool   `json:"force_login"`
	Turnstile    struct {
		Required bool   `json:"required"`
		DX       string `json:"dx"`
	} `json:"turnstile"`
	ProofOfWork struct {
		Required bool `json:"required"`
	} `json:"proofofwork"`
	raw json.RawMessage
}

func truncate(text string, limit int) string {
	if len(text) > limit {
		return text[:limit]
	}
	return text
}
