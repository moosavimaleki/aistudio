package aistudio

import (
	"fmt"
	"regexp"
	"strings"
)

func EncodeContents(contents []any) ([]any, error) {
	if len(contents) == 0 {
		return nil, fmt.Errorf("contents must contain at least one turn")
	}
	result := make([]any, 0, len(contents))
	for _, content := range contents {
		encoded, err := EncodeContent(content)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded)
	}
	return result, nil
}
func EncodeContent(value any) ([]any, error) {
	content, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Content must be an object")
	}
	role, _ := content["role"].(string)
	if role != "user" && role != "model" {
		return nil, fmt.Errorf("Content role must be 'user' or 'model'")
	}
	parts, ok := content["parts"].([]any)
	if !ok {
		parts = []any{content["text"]}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("Content parts must be a non-empty list")
	}
	encoded := make([]any, 0, len(parts))
	for _, part := range parts {
		item, err := EncodePart(part)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, item)
	}
	return []any{encoded, role}, nil
}
func EncodePart(value any) ([]any, error) {
	if raw, ok := value.([]any); ok {
		return raw, nil
	}
	if text, ok := value.(string); ok {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("Text parts must be non-empty strings")
		}
		return []any{nil, text}, nil
	}
	part, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Part must be text, an object, or a raw positional list")
	}
	return EncodeNamedPart(part)
}
func EncodeSystemInstruction(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok {
		value = map[string]any{"parts": []any{map[string]any{"text": text}}}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("systemInstruction must contain parts")
	}
	parts, ok := object["parts"].([]any)
	if !ok || len(parts) == 0 {
		return nil, fmt.Errorf("systemInstruction must contain parts")
	}
	encoded := make([]any, 0, len(parts))
	for _, part := range parts {
		item, err := EncodePart(part)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, item)
	}
	return []any{encoded, "user"}, nil
}
func EncodeNamedPart(part map[string]any) ([]any, error) {
	if raw, ok := part["raw"].([]any); ok {
		return raw, nil
	}
	encoded := make([]any, 16)
	switch {
	case part["text"] != nil:
		encoded[1] = part["text"]
	case part["fileData"] != nil:
		file, ok := part["fileData"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("fileData must be an object")
		}
		id, _ := file["fileId"].(string)
		if id == "" {
			return nil, fmt.Errorf("fileData.fileId is required")
		}
		encoded[5] = []any{id}
	case part["functionCall"] != nil:
		value, err := encodeFunctionCall(part["functionCall"])
		if err != nil {
			return nil, err
		}
		encoded[10] = value
	case part["functionResponse"] != nil:
		value, err := encodeFunctionResponse(part["functionResponse"])
		if err != nil {
			return nil, err
		}
		encoded[11] = value
	case part["executableCode"] != nil:
		value, err := encodeExecutableCode(part["executableCode"])
		if err != nil {
			return nil, err
		}
		encoded[7] = value
	case part["codeExecutionResult"] != nil:
		value, err := encodeExecutionResult(part["codeExecutionResult"])
		if err != nil {
			return nil, err
		}
		encoded[8] = value
	default:
		return nil, fmt.Errorf("Unsupported Part payload")
	}
	encoded[12] = part["thought"]
	encoded[14] = part["thoughtSignature"]
	if value := part["videoMetadata"]; value != nil {
		metadata, err := encodeVideoMetadata(value)
		if err != nil {
			return nil, err
		}
		encoded[15] = metadata
	}
	return trim(encoded), nil
}
func encodeFunctionCall(value any) ([]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("functionCall must be an object")
	}
	name, _ := object["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("functionCall.name is required")
	}
	return trim([]any{name, EncodeStruct(object["args"]), object["id"]}), nil
}
func encodeFunctionResponse(value any) ([]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("functionResponse must be an object")
	}
	name, _ := object["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("functionResponse.name is required")
	}
	return trim([]any{name, EncodeStruct(object["response"]), object["id"]}), nil
}
func encodeExecutableCode(value any) ([]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("executableCode must be an object")
	}
	lang, _ := object["language"].(string)
	code, _ := object["code"].(string)
	enum, ok := map[string]int{"LANGUAGE_UNSPECIFIED": 0, "PYTHON": 1}[lang]
	if !ok || code == "" {
		return nil, fmt.Errorf("invalid executableCode")
	}
	return []any{enum, code}, nil
}
func encodeExecutionResult(value any) ([]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("codeExecutionResult must be an object")
	}
	outcome, _ := object["outcome"].(string)
	output, _ := object["output"].(string)
	enum, ok := map[string]int{"OUTCOME_UNSPECIFIED": 0, "OUTCOME_OK": 1, "OUTCOME_FAILED": 2, "OUTCOME_DEADLINE_EXCEEDED": 3}[outcome]
	if !ok {
		return nil, fmt.Errorf("invalid codeExecutionResult")
	}
	return []any{enum, output}, nil
}

var durationPattern = regexp.MustCompile(`^(\d+)(?:\.(\d{1,9}))?s$`)

func encodeVideoMetadata(value any) ([]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("videoMetadata must be an object")
	}
	start, err := encodeDuration(object["startOffset"])
	if err != nil {
		return nil, err
	}
	end, err := encodeDuration(object["endOffset"])
	if err != nil {
		return nil, err
	}
	return trim([]any{start, end, object["fps"]}), nil
}
func encodeDuration(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("Video offsets must use protobuf duration syntax such as 1.5s")
	}
	matches := durationPattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("Video offsets must use protobuf duration syntax such as 1.5s")
	}
	seconds := 0
	fmt.Sscanf(matches[1], "%d", &seconds)
	nanos := strings.TrimRight(matches[2]+"000000000", "0")
	if nanos == "" {
		return []any{seconds}, nil
	}
	for len(nanos) < 9 {
		nanos += "0"
	}
	nano := 0
	fmt.Sscanf(nanos, "%d", &nano)
	return []any{seconds, nano}, nil
}
func trim(values []any) []any {
	last := len(values)
	for last > 0 && values[last-1] == nil {
		last--
	}
	return values[:last]
}
