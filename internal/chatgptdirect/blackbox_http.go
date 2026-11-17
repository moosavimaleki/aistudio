package chatgptdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	fhttp "github.com/bogdanfinn/fhttp"
)

func (s *blackBoxClient) postPrepare(ctx context.Context, headers map[string]string, initial string) (prepareResponse, error) {
	var raw json.RawMessage
	err := s.postBlackBox(ctx, headers, requirementsPreparePath, map[string]string{"p": initial}, &raw)
	if err != nil {
		return prepareResponse{}, err
	}
	var result prepareResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return prepareResponse{}, fmt.Errorf("black box preparation returned invalid requirements")
	}
	if result.ForceLogin {
		return prepareResponse{}, fmt.Errorf("ChatGPT requires a fresh authenticated session")
	}
	if result.PrepareToken == "" && result.Token == "" {
		return prepareResponse{}, fmt.Errorf("black box preparation returned no token")
	}
	if result.Runtime.Required && result.Runtime.DX == "" {
		return prepareResponse{}, fmt.Errorf("black box requested runtime data without a payload")
	}
	result.raw = raw
	return result, nil
}

func (s *blackBoxClient) postFinalize(
	ctx context.Context,
	headers map[string]string,
	prepare prepareResponse,
	workToken, runtimeToken string,
) (string, error) {
	if prepare.Token != "" {
		return prepare.Token, nil
	}
	body := map[string]string{"prepare_token": prepare.PrepareToken}
	if workToken != "" {
		body["proofofwork"] = workToken
	}
	if runtimeToken != "" {
		body["turnstile"] = runtimeToken
	}
	var result struct {
		Token      string `json:"token"`
		ForceLogin bool   `json:"force_login"`
	}
	if err := s.postBlackBox(ctx, headers, requirementsFinalizePath, body, &result); err != nil {
		return "", err
	}
	if result.ForceLogin || result.Token == "" {
		return "", fmt.Errorf("black box finalization returned no usable token")
	}
	return result.Token, nil
}

func (s *blackBoxClient) postBlackBox(
	ctx context.Context,
	headers map[string]string,
	path string,
	input, output any,
) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode black box request: %w", err)
	}
	request, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodPost, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create black box request: %w", err)
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
		return fmt.Errorf("black box request %s: %w", path, err)
	}
	defer closeReadBody(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("black box %s returned HTTP %d", path, response.StatusCode)
	}
	data, err := readResponseBody(response)
	if err != nil {
		return fmt.Errorf("read black box response: %w", err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("black box %s returned invalid JSON", path)
	}
	return nil
}
