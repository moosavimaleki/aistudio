package aistudio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func ExtractRuntimeConfig(document, authUser string) (RuntimeConfig, error) {
	upstream, err := LoadUpstream()
	if err != nil {
		return RuntimeConfig{}, err
	}
	apiProperty, visitProperty, attestationProperty := mustValue(upstream.Runtime, "api_key_property"), mustValue(upstream.Runtime, "visit_id_property"), mustValue(upstream.Runtime, "attestation_enabled_property")
	extract := func(name string) string {
		match := regexp.MustCompile(`"` + regexp.QuoteMeta(name) + `":"([^"\\]+)"`).FindStringSubmatch(document)
		if len(match) == 2 {
			return match[1]
		}
		return ""
	}
	apiKey, rawVisitID := extract(apiProperty), extract(visitProperty)
	if apiKey == "" || rawVisitID == "" {
		return RuntimeConfig{}, &ClientError{Message: "Bootstrap response does not contain configured runtime markers", Phase: "CONFIG", Diagnostics: map[string]any{"bodyBytes": len(document), "hasAPIKeyMarker": strings.Contains(document, `"`+apiProperty+`"`), "hasVisitIDMarker": strings.Contains(document, `"`+visitProperty+`"`), "looksLikeSignIn": regexp.MustCompile(`(?i)accounts\.google\.com|sign in`).MatchString(document)}}
	}
	disabled := regexp.MustCompile(`"` + regexp.QuoteMeta(attestationProperty) + `"\s*:\s*(?:false|"false")`).MatchString(document)
	return RuntimeConfig{APIKey: apiKey, VisitID: "v1_" + base64.RawURLEncoding.EncodeToString([]byte(rawVisitID)), AuthUser: authUser, AttestationEnabled: !disabled}, nil
}

func FetchRuntimeConfig(ctx context.Context, httpClient *HTTPClient, cookies *CookieJar, tokenFactoryURL, authUser, browserID string) (RuntimeConfig, map[string]string, error) {
	if tokenFactoryURL == "" {
		return RuntimeConfig{}, nil, fmt.Errorf("direct bootstrap is not supported in the Go migration; TOKEN_FACTORY_URL is required")
	}
	endpoint := strings.TrimRight(tokenFactoryURL, "/")
	endpoint = endpoint[:strings.LastIndex(endpoint, "/")+1] + "bootstrap"
	body, _ := json.Marshal(map[string]any{"cookies": cookies.Header(), "authUser": authUser, "browserId": browserID})
	response, err := httpClient.Request(ctx, "POST", endpoint, map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return RuntimeConfig{}, nil, err
	}
	defer response.Body.Close()
	text, err := ReadBody(response)
	if err != nil {
		return RuntimeConfig{}, nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return RuntimeConfig{}, nil, &ClientError{Message: fmt.Sprintf("Browser bootstrap failed with HTTP %d", response.StatusCode), Phase: "CONFIG", Status: response.StatusCode, ResponseBody: text}
	}
	var decoded struct {
		RuntimeConfig    RuntimeConfig     `json:"runtimeConfig"`
		TransportProfile map[string]string `json:"transportProfile"`
		CookieRecords    []CookieRecord    `json:"cookieRecords"`
	}
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return RuntimeConfig{}, nil, err
	}
	if decoded.RuntimeConfig.APIKey == "" || decoded.RuntimeConfig.VisitID == "" {
		return RuntimeConfig{}, nil, NewError("CONFIG", "Browser bootstrap does not contain runtime config")
	}
	if decoded.TransportProfile["User-Agent"] == "" || decoded.TransportProfile["x-client-data"] == "" {
		return RuntimeConfig{}, nil, NewError("CONFIG", "Browser bootstrap does not contain shared browser fingerprint")
	}
	cookies.Apply(decoded.CookieRecords)
	return decoded.RuntimeConfig, decoded.TransportProfile, nil
}
