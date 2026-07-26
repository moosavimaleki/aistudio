package aistudio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

func RPCURL(method string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`).MatchString(method) {
		return "", fmt.Errorf("invalid RPC method")
	}
	upstream, err := LoadUpstream()
	if err != nil {
		return "", err
	}
	return mustValue(upstream.MakerSuite, "base_url") + "/$rpc/" + mustValue(upstream.MakerSuite, "service") + "/" + method, nil
}

func ComposeHeaders(auth *AuthContext, cookies string, runtime RuntimeConfig, profile map[string]string, loggingContext string) (map[string]string, error) {
	authorization := auth.Authorization()
	if authorization == "" {
		return nil, fmt.Errorf("no SAPISID-family cookie is available for Authorization")
	}
	upstream, err := LoadUpstream()
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"Cookie": cookies, "Authorization": authorization, "X-Goog-Api-Key": runtime.APIKey, "X-AIStudio-Visit-Id": runtime.VisitID, "X-Goog-AuthUser": runtime.AuthUser, "Origin": auth.Origin, "Referer": auth.Origin + "/", "Content-Type": "application/json+protobuf", "X-User-Agent": "grpc-web-javascript/0.1"}
	for name, value := range profile {
		headers[name] = value
	}
	if loggingContext != "" {
		headers[mustValue(upstream.MakerSuite, "logging_context_header")] = loggingContext
	}
	return headers, nil
}

var defaultSafetySettings = []any{[]any{nil, nil, 7, 5}, []any{nil, nil, 8, 5}, []any{nil, nil, 9, 5}, []any{nil, nil, 10, 5}}

func BuildGeneratePayload(input GenerateInput) ([]any, error) {
	if !strings.HasPrefix(input.Model, "models/") {
		return nil, fmt.Errorf("model must start with models/")
	}
	contents := input.Contents
	if len(contents) == 0 {
		latest := input.LatestUserTurn
		if latest == nil {
			latest = map[string]any{"role": "user", "text": input.Prompt}
		}
		contents = append(append([]any{}, input.History...), latest)
	}
	encodedContents, err := EncodeContents(contents)
	if err != nil {
		return nil, err
	}
	payload := make([]any, 11)
	payload[0], payload[1] = input.Model, encodedContents
	if input.SafetySettings == nil {
		payload[2] = defaultSafetySettings
	} else {
		payload[2] = input.SafetySettings
	}
	config, err := EncodeGenerationConfig(input.GenerationConfig)
	if err != nil {
		return nil, err
	}
	payload[3] = config
	system, err := EncodeSystemInstruction(input.SystemInstruction)
	if err != nil {
		return nil, err
	}
	payload[5] = system
	tools, err := EncodeTools(input.Tools)
	if err != nil {
		return nil, err
	}
	payload[6], payload[10] = tools, 1
	if input.ContinuationToken != nil {
		payload = setField(payload, 11, input.ContinuationToken)
	}
	if input.ToolContext != nil {
		payload = setField(payload, 13, input.ToolContext)
	}
	return payload, nil
}

func setField(payload []any, index int, value any) []any {
	for len(payload) <= index {
		payload = append(payload, nil)
	}
	payload[index] = value
	return payload
}
func ContentBindingDigest(payload []any) (string, error) {
	if len(payload) < 2 {
		return "", fmt.Errorf("GenerateContent payload must contain contents at index 1")
	}
	turns, ok := payload[1].([]any)
	if !ok {
		return "", fmt.Errorf("GenerateContent payload must contain contents at index 1")
	}
	values := []string{}
	for _, rawTurn := range turns {
		turn, ok := rawTurn.([]any)
		if !ok || len(turn) == 0 {
			continue
		}
		parts, ok := turn[0].([]any)
		if !ok {
			continue
		}
		for _, rawPart := range parts {
			values = append(values, digestPart(rawPart))
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(values, " ")))
	return hex.EncodeToString(sum[:]), nil
}
func digestPart(value any) string {
	part, ok := value.([]any)
	if !ok {
		return ""
	}
	if len(part) > 1 {
		if text, ok := part[1].(string); ok {
			return text
		}
	}
	if len(part) > 5 {
		if files, ok := part[5].([]any); ok && len(files) > 0 {
			if id, ok := files[0].(string); ok {
				return id
			}
		}
	}
	return ""
}
