package chatgptdirect

import (
	"encoding/json"
	"strings"
)

type streamResult struct {
	Text           string
	ConversationID string
	MessageID      string
	Error          string
}

type patchState struct {
	Path      string
	Operation string
}

func parseEventStream(data []byte) streamResult {
	result := streamResult{}
	state := patchState{}
	stream := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, block := range strings.Split(stream, "\n\n") {
		consumeEvent(strings.TrimSpace(block), &state, &result)
	}
	return result
}

func consumeEvent(block string, state *patchState, result *streamResult) {
	values := []string{}
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "data:") {
			values = append(values, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	data := strings.Join(values, "\n")
	if data == "" || data == "[DONE]" {
		return
	}
	var value any
	if json.Unmarshal([]byte(data), &value) == nil {
		consumeValue(value, state, result)
	}
}

func consumeValue(value any, state *patchState, result *streamResult) {
	if values, ok := value.([]any); ok {
		for _, item := range values {
			consumeValue(item, state, result)
		}
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if conversationID, ok := object["conversation_id"].(string); ok {
		result.ConversationID = conversationID
	}
	if path, ok := object["p"].(string); ok {
		state.Path = path
	}
	if operation, ok := object["o"].(string); ok {
		state.Operation = operation
	}
	if _, hasValue := object["v"]; hasValue {
		applyPatch(map[string]any{
			"p": state.Path,
			"o": state.Operation,
			"v": object["v"],
		}, result)
	}
}

func applyPatch(patch map[string]any, result *streamResult) {
	path, _ := patch["p"].(string)
	operation, _ := patch["o"].(string)
	value := patch["v"]
	if path == "" {
		if values, ok := value.([]any); ok {
			for _, item := range values {
				if child, ok := item.(map[string]any); ok {
					applyPatch(child, result)
				}
			}
			return
		}
		if root, ok := value.(map[string]any); ok {
			consumeRoot(root, result)
		}
		return
	}
	if path == "/message/content/parts/0" && operation == "append" {
		if text, ok := value.(string); ok {
			result.Text += text
		}
	}
	if path == "/message/id" {
		if messageID, ok := value.(string); ok {
			result.MessageID = messageID
		}
	}
}

func consumeRoot(root map[string]any, result *streamResult) {
	if conversationID, ok := root["conversation_id"].(string); ok {
		result.ConversationID = conversationID
	}
	if failure, ok := root["error"].(map[string]any); ok {
		result.Error, _ = failure["message"].(string)
	}
	if failure, ok := root["error"].(string); ok {
		result.Error = failure
	}
	if result.Error == "" {
		result.Error, _ = root["error_code"].(string)
	}
	message, ok := root["message"].(map[string]any)
	if !ok {
		return
	}
	if messageID, ok := message["id"].(string); ok {
		result.MessageID = messageID
	}
	author, _ := message["author"].(map[string]any)
	if author["role"] != "assistant" {
		return
	}
	content, _ := message["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) > 0 {
		if text, ok := parts[0].(string); ok {
			result.Text = text
		}
	}
}
