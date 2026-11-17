package aistudio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func (t *Tab) GenerateVideo(ctx context.Context, input VideoInput) (VideoResult, error) {
	if t.State != TabReady || t.TokenFactory == nil {
		return VideoResult{}, NewError("RPC", "Tab is not ready")
	}
	payload, err := BuildVideoPayload(input)
	if err != nil {
		return VideoResult{}, err
	}
	digest, err := VideoContentDigest(payload)
	if err != nil {
		return VideoResult{}, err
	}
	endpoint, err := RPCURL("CreateInteractionStream")
	if err != nil {
		return VideoResult{}, err
	}
	profile, err := t.videoTransportProfile(ctx)
	if err != nil {
		return VideoResult{}, err
	}
	headers, err := ComposeHeaders(t.Auth, t.Cookies.Header(), t.Runtime, profile, t.LoggingContextExtension)
	if err != nil {
		return VideoResult{}, err
	}
	snapshot, err := t.TokenFactory.Snapshot(ctx, digest, map[string]any{
		"url": endpoint, "method": "POST", "headers": headers, "payload": payload,
	})
	if err != nil {
		return VideoResult{}, err
	}
	t.Cookies.Apply(snapshot.CookieRecords)
	t.syncSession()
	runtime := t.Runtime
	if snapshot.RuntimeConfig != nil {
		runtime = *snapshot.RuntimeConfig
	}
	loggingContext := t.LoggingContextExtension
	if snapshot.LoggingContextExtension != "" {
		loggingContext = snapshot.LoggingContextExtension
	}
	headers, err = ComposeHeaders(
		t.Auth,
		t.Cookies.Header(),
		runtime,
		mergeHeaders(profile, snapshot.TransportProfile),
		loggingContext,
	)
	if err != nil {
		return VideoResult{}, err
	}
	payload[4] = snapshot.Token
	encoded, err := json.Marshal(payload)
	if err != nil {
		return VideoResult{}, err
	}
	t.State = TabGenerating
	defer func() { t.State = TabReady }()
	// Do not replay this billed operation after a timeout or partial response.
	response, err := t.HTTP.Request(ctx, "POST", endpoint, headers, encoded)
	if err != nil {
		return VideoResult{}, fmt.Errorf("CreateInteractionStream transport: %w", err)
	}
	defer response.Body.Close()
	t.Cookies.ApplyResponse(response)
	t.syncSession()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return VideoResult{}, &ClientError{
			Message: fmt.Sprintf("CreateInteractionStream failed with HTTP %d", response.StatusCode),
			Phase:   "RPC", Status: response.StatusCode, ResponseBody: string(body),
		}
	}
	result, err := CollectVideoResult(response.Body)
	if err == nil {
		t.GenerateCount++
	}
	return result, err
}
