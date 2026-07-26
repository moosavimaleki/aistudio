package browserinterface

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/hamed/aistudio-api/internal/aistudio"
	"strings"
)

func ValidateTokenRequest(body map[string]any) (map[string]string, string, error) {
	digest, _ := body["digest"].(string)
	cookies, _ := body["cookies"].(string)
	authorization, _ := body["authorization"].(string)
	authUser := fmt.Sprint(body["authUser"])
	if authUser == "" || authUser == "<nil>" {
		authUser = "0"
	}
	if !isDigest(digest) {
		return nil, "", fmt.Errorf("digest must be lowercase SHA-256 hex")
	}
	if cookies == "" || authorization == "" {
		return nil, "", fmt.Errorf("cookies and authorization are required")
	}
	upstream, err := aistudio.LoadUpstream()
	if err != nil {
		return nil, "", err
	}
	if body["waaApiKey"] != upstream.Opaque["waa_api_key"] {
		return nil, "", fmt.Errorf("waaApiKey does not match upstream config")
	}
	request, ok := body["generateRequest"].(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("pending GenerateContent request context is required")
	}
	payload, ok := request["payload"].([]any)
	if !ok {
		return nil, "", fmt.Errorf("pending GenerateContent request context is required")
	}
	computed, err := aistudio.ContentBindingDigest(payload)
	if err != nil || computed != digest {
		return nil, "", fmt.Errorf("GenerateContent body does not match digest")
	}
	headers := normalizeHeaders(request["headers"])
	for _, name := range []string{"authorization", "cookie", "origin", "user-agent", "x-client-data", "x-goog-api-key", "x-goog-authuser"} {
		if headers[name] == "" {
			return nil, "", fmt.Errorf("GenerateContent context is missing %s", name)
		}
	}
	if headers["origin"] != upstream.AIStudio["origin"] || headers["cookie"] != cookies || headers["x-goog-authuser"] != authUser {
		return nil, "", fmt.Errorf("GenerateContent request session differs from Token Factory")
	}
	return headers, authUser, nil
}
func normalizeHeaders(value any) map[string]string {
	result := map[string]string{}
	if headers, ok := value.(map[string]any); ok {
		for name, value := range headers {
			result[strings.ToLower(name)] = fmt.Sprint(value)
		}
	}
	if headers, ok := value.(map[string]string); ok {
		for name, value := range headers {
			result[strings.ToLower(name)] = value
		}
	}
	return result
}
func isDigest(value string) bool {
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
func SessionFingerprint(header, authUser string) string {
	values := map[string]string{}
	for _, pair := range strings.Split(header, ";") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{authUser, values["SID"], values["SAPISID"], values["__Secure-1PAPISID"], values["__Secure-3PAPISID"]}, "\000")))
	return hex.EncodeToString(sum[:])
}
