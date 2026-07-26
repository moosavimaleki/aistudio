package browserinterface

import (
	"encoding/json"
	"fmt"
	"strings"

	cdpRuntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type probeResult struct {
	Status       int    `json:"status"`
	Body         string `json:"body"`
	NetworkError string `json:"networkError"`
	Location     string `json:"location"`
}

func (s *ChromeSession) Probe(generateRequest map[string]any, token string) (probeResult, error) {
	payload, ok := generateRequest["payload"].([]any)
	url, urlOK := generateRequest["url"].(string)
	if !ok || !urlOK {
		return probeResult{}, fmt.Errorf("GenerateContent request context is invalid for probe")
	}
	encoded, _ := json.Marshal(payload)
	var requestPayload []any
	if err := json.Unmarshal(encoded, &requestPayload); err != nil || len(requestPayload) < 5 {
		return probeResult{}, fmt.Errorf("GenerateContent payload is invalid for probe")
	}
	requestPayload[4] = token

	headers := probeHeaders(stringMap(generateRequest["headers"]))
	for name, value := range probeHeaders(s.observedHeaders()) {
		headers[name] = value
	}
	s.stateMu.RLock()
	runtime := s.runtime
	s.stateMu.RUnlock()
	headers["x-goog-api-key"] = runtime.APIKey
	headers["x-aistudio-visit-id"] = runtime.VisitID
	headers["x-goog-authuser"] = runtime.AuthUser

	var result probeResult
	input := map[string]any{"url": url, "headers": headers, "payload": requestPayload}
	encodedInput, _ := json.Marshal(input)
	expression := "(" + probeScript + ")(" + string(encodedInput) + ")"
	awaitPromise := func(params *cdpRuntime.EvaluateParams) *cdpRuntime.EvaluateParams {
		return params.WithAwaitPromise(true)
	}
	if err := chromedp.Run(s.ctx, chromedp.Evaluate(expression, &result, awaitPromise)); err != nil {
		return probeResult{}, err
	}
	return result, nil
}

func (s *ChromeSession) Fingerprint() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.fingerprint
}

func stringMap(value any) map[string]string {
	result := map[string]string{}
	source, _ := value.(map[string]any)
	for name, raw := range source {
		result[name] = fmt.Sprint(raw)
	}
	if typed, ok := value.(map[string]string); ok {
		return typed
	}
	return result
}

func probeHeaders(source map[string]string) map[string]string {
	result := map[string]string{}
	for name, value := range source {
		lower := strings.ToLower(name)
		if probeHeaderNames[lower] || strings.HasPrefix(lower, "x-goog-ext-") {
			result[lower] = value
		}
	}
	return result
}

var probeHeaderNames = map[string]bool{
	"authorization": true, "content-type": true,
	"x-aistudio-visit-id": true, "x-goog-api-key": true,
	"x-goog-authuser": true, "x-goog-visitor-id": true,
	"x-user-agent": true,
}

const probeScript = `async function(input) {
  try {
    const response = await fetch(input.url, {
      method: "POST",
      credentials: "include",
      headers: input.headers,
      body: JSON.stringify(input.payload),
    });
    return {status: response.status, body: await response.text(), location: location.href};
  } catch (error) {
    return {
      networkError: error instanceof Error ? error.message : String(error),
      location: location.href,
    };
  }
}`
