package aistudio

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (t *Tab) UploadBytes(ctx context.Context, content []byte, mimeType, name string) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("inlineData content must not be empty")
	}
	if t.OAuthAccessToken == "" {
		return "", NewError("UPLOAD", "Tab has no OAuth access token")
	}
	if t.AppFolderID == "" {
		folder, err := t.unary(ctx, "GetAppFolder", []any{}, t.LoggingContextExtension)
		if err != nil {
			return "", err
		}
		fields, ok := folder.([]any)
		if !ok || len(fields) == 0 {
			return "", fmt.Errorf("GetAppFolder returned an invalid folder id")
		}
		t.AppFolderID, _ = fields[0].(string)
		if t.AppFolderID == "" {
			return "", fmt.Errorf("GetAppFolder returned an invalid folder id")
		}
	}
	if name == "" {
		name = "attachment"
	}
	upstream, err := LoadUpstream()
	if err != nil {
		return "", err
	}
	headers := map[string]string{"Authorization": "Bearer " + t.OAuthAccessToken, "Origin": t.Auth.Origin, "Referer": t.Auth.Origin + "/", "X-Goog-AuthUser": t.Runtime.AuthUser, "X-Goog-Encode-Response-If-Executable": "base64", "X-JavaScript-User-Agent": "google-api-javascript-client/1.1.0", "X-Requested-With": "XMLHttpRequest", "User-Agent": t.TransportProfile["User-Agent"]}
	endpoint := mustValue(upstream.Drive, "upload_url") + "?uploadType=multipart&key=" + url.QueryEscape(t.Runtime.APIKey)
	boundary := fmt.Sprintf("go-%d", len(content))
	metadata, _ := json.Marshal(map[string]any{"name": name, "parents": []string{t.AppFolderID}})
	body := []byte("--" + boundary + "\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n" + string(metadata) + "\r\n--" + boundary + "\r\nContent-Type: " + mimeType + "\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString(content) + "\r\n--" + boundary + "--\r\n")
	headers["Content-Type"] = "multipart/related; boundary=\"" + boundary + "\""
	response, err := t.HTTP.Request(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	t.Cookies.ApplyResponse(response)
	t.syncSession()
	text, err := ReadBody(response)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", &ClientError{Message: fmt.Sprintf("UPLOAD failed with HTTP %d", response.StatusCode), Phase: "UPLOAD", Status: response.StatusCode, ResponseBody: text}
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return "", fmt.Errorf("Drive upload response has no file id")
	}
	if decoded.ID == "" {
		return "", fmt.Errorf("Drive upload returned an invalid file id")
	}
	return decoded.ID, nil
}
func DecodeInlineData(value string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return data, nil
	}
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}

var _ = bytes.NewBuffer
