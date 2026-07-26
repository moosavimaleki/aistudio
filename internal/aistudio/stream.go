package aistudio

import (
	"bufio"
	"encoding/json"
	"io"
)

func CollectGenerateResult(reader io.Reader, onChunk func(any)) (GenerateResult, error) {
	result := GenerateResult{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk any
		if err := json.Unmarshal(line, &chunk); err != nil {
			chunk = string(line)
		}
		result.Chunks = append(result.Chunks, chunk)
		result.FinalText += VisibleTextFromChunk(chunk)
		result.ModelParts = append(result.ModelParts, ModelPartsFromChunk(chunk)...)
		if object, ok := chunk.(map[string]any); ok {
			if value := object["finishReason"]; value != nil {
				result.FinishReason = value
			}
			if value := object["usage"]; value != nil {
				result.Usage = value
			}
			if value := object["conversationMetadata"]; value != nil {
				result.ConversationMetadata = value
			}
		}
		if onChunk != nil {
			onChunk(chunk)
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if len(result.Chunks) == 0 {
		return result, NewError("STREAM", "GenerateContent returned an empty stream")
	}
	return result, nil
}

func VisibleTextFromChunk(chunk any) string {
	if text, ok := chunk.(string); ok {
		return text
	}
	if object, ok := chunk.(map[string]any); ok {
		if value, ok := object["text"].(string); ok {
			return value
		}
		if value, ok := object["delta"].(string); ok {
			return value
		}
	}
	text := ""
	for _, frame := range framesFromChunk(chunk) {
		content, ok := modelContent(frame)
		if !ok {
			continue
		}
		for _, raw := range content {
			part, ok := raw.([]any)
			if !ok || len(part) < 2 {
				continue
			}
			if value, ok := part[1].(string); ok {
				text += value
			}
		}
	}
	return text
}

func ModelPartsFromChunk(chunk any) []any {
	result := []any{}
	for _, frame := range framesFromChunk(chunk) {
		content, ok := modelContent(frame)
		if !ok {
			continue
		}
		for _, raw := range content {
			if part, ok := raw.([]any); ok {
				if decoded := DecodePart(part); decoded != nil {
					result = append(result, decoded)
				}
			}
		}
	}
	return result
}

func framesFromChunk(chunk any) []any {
	root, ok := chunk.([]any)
	if !ok || len(root) == 0 {
		return nil
	}
	frames, _ := root[0].([]any)
	return frames
}
func modelContent(frame any) ([]any, bool) {
	first, ok := frame.([]any)
	if !ok || len(first) == 0 {
		return nil, false
	}
	second, ok := first[0].([]any)
	if !ok || len(second) == 0 {
		return nil, false
	}
	third, ok := second[0].([]any)
	if !ok || len(third) == 0 {
		return nil, false
	}
	content, ok := third[0].([]any)
	if !ok || len(content) < 2 {
		return nil, false
	}
	role, _ := content[1].(string)
	parts, _ := content[0].([]any)
	return parts, role == "model"
}
func DecodePart(part []any) any {
	if len(part) > 1 {
		if text, ok := part[1].(string); ok {
			return map[string]any{"text": text}
		}
	}
	if len(part) > 5 {
		if files, ok := part[5].([]any); ok && len(files) > 0 {
			return map[string]any{"fileData": map[string]any{"fileId": files[0]}}
		}
	}
	return nil
}
