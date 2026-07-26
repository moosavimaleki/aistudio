package aistudio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func (t *Tab) unary(ctx context.Context, method string, body any, loggingContext string) (any, error) {
	endpoint, err := RPCURL(method)
	if err != nil {
		return nil, err
	}
	headers, err := ComposeHeaders(t.Auth, t.Cookies.Header(), t.Runtime, t.TransportProfile, loggingContext)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(body)
	response, err := t.HTTP.Request(ctx, "POST", endpoint, headers, encoded)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	t.Cookies.ApplyResponse(response)
	t.syncSession()
	text, err := ReadBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &ClientError{Message: fmt.Sprintf("%s failed with HTTP %d", method, response.StatusCode), Phase: "RPC", Status: response.StatusCode, ResponseBody: text}
	}
	var decoded any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return nil, nil
	}
	return decoded, nil
}

func (t *Tab) initializeStartup(ctx context.Context) error {
	first, err := t.accessToken(ctx, "")
	if err != nil {
		return err
	}
	second, err := t.accessToken(ctx, "")
	if err != nil {
		return err
	}
	if first != "" {
		t.OAuthAccessToken = first
	} else {
		t.OAuthAccessToken = second
	}
	context, err := t.unary(ctx, "GetLoggingContext", []any{0}, "")
	if err != nil {
		return err
	}
	if fields, ok := context.([]any); ok {
		t.LoggingContextExtension, err = EncodeLoggingContextExtension(fields)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tab) accessToken(ctx context.Context, loggingContext string) (string, error) {
	value, err := t.unary(ctx, "GenerateAccessToken", []any{"users/me"}, loggingContext)
	if err != nil {
		return "", err
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return "", nil
	}
	token, _ := items[0].(string)
	return token, nil
}

func EncodeLoggingContextExtension(context []any) (string, error) {
	encoded := []byte{}
	for index, value := range context {
		if value == nil {
			continue
		}
		switch item := value.(type) {
		case bool:
			if item {
				encoded = append(encoded, encodeVarint(uint64((index+1)<<3))...)
				encoded = append(encoded, 1)
			} else {
				encoded = append(encoded, encodeVarint(uint64((index+1)<<3))...)
				encoded = append(encoded, 0)
			}
		case float64:
			encoded = append(encoded, encodeVarint(uint64((index+1)<<3))...)
			encoded = append(encoded, encodeVarint(uint64(item))...)
		case string:
			encoded = append(encoded, encodeVarint(uint64((index+1)<<3|2))...)
			encoded = append(encoded, encodeVarint(uint64(len(item)))...)
			encoded = append(encoded, []byte(item)...)
		default:
			return "", fmt.Errorf("Unsupported GetLoggingContext field %d", index+1)
		}
	}
	if len(encoded) == 0 {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}
func encodeVarint(value uint64) []byte {
	encoded := []byte{}
	for {
		next := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			next |= 0x80
		}
		encoded = append(encoded, next)
		if value == 0 {
			return encoded
		}
	}
}
