package chatgptdirect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// Default HTTP metadata used when the session does not supply client headers.
const (
	clientUserAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	clientSecCHUA        = `"Not;A=Brand";v="8", "Chromium";v="150", "Google Chrome";v="150"`
	clientPlatform       = "Linux"
	clientAcceptLanguage = "en-US,en;q=0.9,fa;q=0.8,cs;q=0.7"
	clientRuntimeScript  = "https://chatgpt.com/cdn-cgi/challenge-platform/scripts/jsd/api.js?onload=jsdOnload"
	clientBuildFallback  = "ef0c3fb941611420d8683bf88f61acf1a8ac6b19"
)

type tokenEntry struct {
	AccessToken string
	AccountID   string
	Expires     time.Time
}

// getSession calls GET /api/auth/session with the cookie and returns the JWT
// access token, account id, and expiry. ChatGPT authenticates upstream calls
// with this token as `Authorization: Bearer` alongside the session cookie.
func (s *blackBoxClient) getSession(ctx context.Context, cookie string) (tokenEntry, error) {
	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, s.baseURL+"/api/auth/session", nil)
	if err != nil {
		return tokenEntry{}, err
	}
	s.applyHeaders(req, cookie)
	resp, err := s.http.Do(req)
	if err != nil {
		return tokenEntry{}, fmt.Errorf("chatgpt session request: %w", err)
	}
	defer closeReadBody(resp.Body)
	data, err := readResponseBody(resp)
	if err != nil {
		return tokenEntry{}, fmt.Errorf("read chatgpt session: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenEntry{}, fmt.Errorf("chatgpt /api/auth/session returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"accessToken"`
		Account     struct {
			ID string `json:"id"`
		} `json:"account"`
		Expires json.RawMessage `json:"expires"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return tokenEntry{}, fmt.Errorf("decode chatgpt session: %w", err)
	}
	if body.AccessToken == "" {
		return tokenEntry{}, fmt.Errorf("chatgpt session returned no access token")
	}
	expires, err := parseExpires(body.Expires)
	if err != nil {
		return tokenEntry{}, fmt.Errorf("decode chatgpt session expires: %w", err)
	}
	accountID := body.Account.ID
	if accountID == "" {
		accountID = accountFromJWT(body.AccessToken)
	}
	return tokenEntry{
		AccessToken: body.AccessToken,
		AccountID:   accountID,
		Expires:     expires,
	}, nil
}

func accountFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Auth struct {
			AccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(data, &claims) != nil {
		return ""
	}
	return claims.Auth.AccountID
}

// parseExpires decodes the session `expires` field. ChatGPT returns it as an
// RFC3339 string; older/other clients may send a unix-seconds number, and a
// missing value means no explicit expiry.
func parseExpires(raw json.RawMessage) (time.Time, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return time.Time{}, nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return time.Time{}, err
		}
		return time.Parse(time.RFC3339, str)
	}
	var secs int64
	if err := json.Unmarshal(raw, &secs); err != nil {
		return time.Time{}, err
	}
	return time.Unix(secs, 0), nil
}
func (s *blackBoxClient) applyHeaders(req *fhttp.Request, cookie string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", clientAcceptLanguage)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Origin", upstreamOrigin)
	req.Header.Set("Referer", upstreamOrigin+"/")
	req.Header.Set("Sec-CH-UA", clientSecCHUA)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"Linux"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", clientUserAgent)
}

// deriveDeviceID builds a stable per-cookie device id (UUID form) for the
// oai-device-id / oai-did headers, matching the browser's cookie-derived id.
func deriveDeviceID(cookie string) string {
	sum := sha256.Sum256([]byte(cookie))
	d := sum[:]
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr(d[0:4]), hexStr(d[4:6]), hexStr(d[6:8]), hexStr(d[8:10]), hexStr(d[10:16]))
}

func hexStr(b []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for _, c := range b {
		sb.WriteByte(digits[c>>4])
		sb.WriteByte(digits[c&0x0f])
	}
	return sb.String()
}
