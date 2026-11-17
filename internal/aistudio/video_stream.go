package aistudio

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maxVideoResponseBytes = 128 << 20

type VideoResult struct {
	Data          []byte `json:"-"`
	InteractionID string `json:"interactionId"`
	MIMEType      string `json:"mimeType"`
	Usage         any    `json:"usage,omitempty"`
}

func CollectVideoResult(reader io.Reader) (VideoResult, error) {
	result := VideoResult{MIMEType: "video/mp4"}
	shapes := []any{}
	limited := &io.LimitedReader{R: reader, N: maxVideoResponseBytes + 1}
	decoder := json.NewDecoder(limited)
	for {
		var chunk any
		err := decoder.Decode(&chunk)
		if limited.N <= 0 {
			return VideoResult{}, NewError("STREAM", "Video response exceeds 128 MiB")
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return VideoResult{}, fmt.Errorf("decode video stream: %w", err)
		}
		if object, ok := chunk.(map[string]any); ok && object["error"] != nil {
			return VideoResult{}, NewError("STREAM", "CreateInteractionStream returned an error")
		}
		if err := videoStreamError(chunk); err != nil {
			return VideoResult{}, err
		}
		if len(shapes) < 8 {
			shapes = append(shapes, videoResponseShape(chunk, 9))
		}
		if err := collectVideoFrames(&result, chunk); err != nil {
			return VideoResult{}, err
		}
	}
	if len(result.Data) == 0 {
		return VideoResult{}, &ClientError{
			Message: "CreateInteractionStream returned no generated MP4 video", Phase: "STREAM",
			Diagnostics: map[string]any{"responseShape": shapes},
		}
	}
	return result, nil
}

// Describe unexpected protocol frames without exposing prompts, thought text,
// tokens, or base64 media in an error response.
func videoResponseShape(value any, depth int) any {
	if depth == 0 {
		return "..."
	}
	switch value := value.(type) {
	case []any:
		fields := map[int]any{}
		for index, item := range value {
			if item != nil && len(fields) < 24 {
				fields[index] = videoResponseShape(item, depth-1)
			}
		}
		return fields
	case string:
		return fmt.Sprintf("string(%d)", len(value))
	case map[string]any:
		return "object"
	default:
		return value
	}
}

func collectVideoFrames(result *VideoResult, chunk any) error {
	for _, frame := range framesFromChunk(chunk) {
		for _, field := range []int{18, 19} {
			if id, ok := positional(frame, field, 0, 0).(string); ok {
				result.InteractionID = id
			}
		}
		if usage, ok := positional(frame, 19, 0, 12).([]any); ok {
			result.Usage = decodeInteractionUsage(usage)
		}
		encoded, ok := positional(frame, 10, 1, 4, 1).(string)
		if !ok || encoded == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return NewError("STREAM", "Generated video contains invalid base64")
		}
		if !IsMP4(data) {
			return NewError("STREAM", "Generated video does not contain an MP4 header")
		}
		if len(result.Data) != 0 {
			return NewError("STREAM", "Video stream contains more than one video")
		}
		result.Data = data
	}
	return nil
}

func IsMP4(data []byte) bool {
	return len(data) >= 24 && string(data[4:8]) == "ftyp"
}

func decodeInteractionUsage(values []any) map[string]any {
	result := map[string]any{}
	for name, index := range map[string]int{
		"promptTokenCount": 0, "candidatesTokenCount": 4,
		"thoughtsTokenCount": 8, "totalTokenCount": 9,
	} {
		if count, ok := integer(positional(values, index)); ok {
			result[name] = count
		}
	}
	return result
}
