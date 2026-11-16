package chatgptdirect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

func loadSavedHeaders(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open saved ChatGPT session: %w", err)
	}
	defer closeReadBody(file)
	var snapshot struct {
		Version int               `json:"version"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&snapshot); err != nil {
		// Decoder errors can contain credential material; report only the stage.
		return nil, fmt.Errorf("saved ChatGPT session is not valid JSON")
	}
	origin, err := url.Parse(snapshot.URL)
	if err != nil || snapshot.Version != 1 {
		return nil, fmt.Errorf("saved ChatGPT session has an unsupported format")
	}
	if origin.Scheme != "https" || origin.Host != "chatgpt.com" || origin.User != nil {
		return nil, fmt.Errorf("saved ChatGPT session has an invalid origin")
	}
	headers := make(map[string]string, len(snapshot.Headers))
	for name, value := range snapshot.Headers {
		if strings.ContainsAny(name+value, "\r\n") {
			return nil, fmt.Errorf("saved ChatGPT session has invalid headers")
		}
		headers[strings.ToLower(name)] = value
	}
	if headers["cookie"] == "" || !strings.HasPrefix(headers["authorization"], "Bearer ") {
		return nil, fmt.Errorf("saved ChatGPT session requires cookie and bearer credentials")
	}
	delete(headers, "x-openai-web-sse-compression")
	return headers, nil
}

func bearerExpired(authorization string) bool {
	parts := strings.Split(strings.TrimPrefix(authorization, "Bearer "), ".")
	if len(parts) != 3 {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Expires int64 `json:"exp"`
	}
	if json.Unmarshal(data, &claims) != nil || claims.Expires == 0 {
		return false
	}
	return claims.Expires < time.Now().Add(time.Minute).Unix()
}
