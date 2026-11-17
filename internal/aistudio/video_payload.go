package aistudio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const VideoModel = "models/gemini-omni-1.1-flash"

type VideoInput struct {
	Model  string
	Prompt string
}

// BuildVideoPayload follows the CreateInteractionStream video request captured
// from AI Studio. Duration and scene details belong in the prompt.
func BuildVideoPayload(input VideoInput) ([]any, error) {
	if input.Model != VideoModel {
		return nil, fmt.Errorf("video generation supports only %s", VideoModel)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	config := make([]any, 25)
	config[5], config[6], config[7], config[24] = 4, 1, 65536, []any{}
	interaction := make([]any, 54)
	interaction[17] = []any{input.Model, config}
	interaction[26] = []any{[]any{[]any{[]any{[]any{[]any{[]any{input.Prompt}}}}}}}
	interaction[53] = []any{[]any{[]any{nil, nil, nil, []any{nil, nil, nil, nil, nil, 3}}}}
	return []any{1, 1, nil, interaction, nil, 1}, nil
}

// VideoContentDigest binds only the public prompt; token creation stays behind
// the Token Factory API, just as it does for GenerateContent.
func VideoContentDigest(payload []any) (string, error) {
	prompt, ok := positional(payload, 3, 26, 0, 0, 0, 0, 0, 0, 0).(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("CreateInteractionStream payload must contain a prompt")
	}
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:]), nil
}

func positional(value any, indices ...int) any {
	for _, index := range indices {
		items, ok := value.([]any)
		if !ok || index < 0 || index >= len(items) {
			return nil
		}
		value = items[index]
	}
	return value
}
