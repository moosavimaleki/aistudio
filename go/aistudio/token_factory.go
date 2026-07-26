package aistudio

import (
	"context"
	"encoding/json"
	"fmt"
)

type TokenSnapshot struct {
	Token                   string            `json:"token"`
	CookieRecords           []CookieRecord    `json:"cookieRecords"`
	TransportProfile        map[string]string `json:"transportProfile"`
	RuntimeConfig           *RuntimeConfig    `json:"runtimeConfig"`
	LoggingContextExtension string            `json:"loggingContextExtension"`
}
type StagingTokenFactory struct {
	HTTP                      *HTTPClient
	URL, WAAAPIKey, BrowserID string
	Auth                      *AuthContext
	Runtime                   RuntimeConfig
}

func (f *StagingTokenFactory) Snapshot(ctx context.Context, digest string, generateRequest map[string]any) (TokenSnapshot, error) {
	if !regexpDigest(digest) {
		return TokenSnapshot{}, fmt.Errorf("content digest must be lowercase SHA-256 hex")
	}
	authorization := f.Auth.Authorization()
	if authorization == "" {
		return TokenSnapshot{}, fmt.Errorf("No session authorization is available for token factory")
	}
	body, _ := json.Marshal(map[string]any{"digest": digest, "cookies": f.Auth.CookieHeader, "authorization": authorization, "waaApiKey": f.WAAAPIKey, "visitId": f.Runtime.VisitID, "authUser": f.Runtime.AuthUser, "attestationEnabled": f.Runtime.AttestationEnabled, "generateRequest": generateRequest, "browserId": f.BrowserID})
	response, err := f.HTTP.Request(ctx, "POST", f.URL, map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return TokenSnapshot{}, err
	}
	defer response.Body.Close()
	text, err := ReadBody(response)
	if err != nil {
		return TokenSnapshot{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TokenSnapshot{}, &ClientError{Message: fmt.Sprintf("Token factory failed with HTTP %d", response.StatusCode), Phase: "ATTESTATION", Status: response.StatusCode, ResponseBody: text}
	}
	var snapshot TokenSnapshot
	if err := json.Unmarshal([]byte(text), &snapshot); err != nil {
		return TokenSnapshot{}, err
	}
	if snapshot.Token == "" {
		return TokenSnapshot{}, fmt.Errorf("Token factory response does not contain a token")
	}
	return snapshot, nil
}
func regexpDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
