package aistudio

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// A JSON+protobuf stream can end with an RPC status even when HTTP was 200.
// Preserve that status rather than reporting a missing video.
func videoStreamError(chunk any) error {
	code, ok := integer(positional(chunk, 1, 0))
	if !ok || code == 0 {
		return nil
	}
	message, _ := positional(chunk, 1, 1).(string)
	if len(message) > 2048 {
		message = message[:2048]
	}
	status := http.StatusBadGateway
	switch code {
	case 3, 9, 11:
		status = http.StatusBadRequest
	case 4:
		status = http.StatusGatewayTimeout
	case 5:
		status = http.StatusNotFound
	case 7:
		status = http.StatusForbidden
	case 8:
		status = http.StatusTooManyRequests
	case 14:
		status = http.StatusServiceUnavailable
	case 16:
		status = http.StatusUnauthorized
	}
	body, _ := json.Marshal(map[string]any{"code": code, "message": message})
	return &ClientError{
		Message: fmt.Sprintf("CreateInteractionStream RPC error %d: %s", code, message),
		Phase:   "RPC", Status: status, ResponseBody: string(body),
	}
}
