package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
)

type conversationState struct {
	ID          string         `json:"id"`
	CurrentNode string         `json:"current_node"`
	Messages    []savedMessage `json:"messages"`
}

type savedMessage struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Channel string `json:"channel"`
	Author  struct {
		Role string `json:"role"`
	} `json:"author"`
	Content struct {
		Parts []json.RawMessage `json:"parts"`
	} `json:"content"`
}

func (c *Client) conversation(ctx context.Context, id string) (conversationState, error) {
	if c.sentinel == nil {
		return conversationState{}, fmt.Errorf("history access requires a native direct client")
	}
	headers, err := c.sentinel.authHeaders(ctx, "")
	if err != nil {
		return conversationState{}, err
	}
	path := "/backend-api/conversations/" + url.PathEscape(id)
	request, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return conversationState{}, fmt.Errorf("create conversation history request: %w", err)
	}
	for key, value := range headers {
		if !blockedHeader(key) {
			request.Header.Set(key, value)
		}
	}
	request.Header.Set("Cookie", headers["cookie"])
	request.Header.Set("Accept-Encoding", "gzip, deflate")
	request.Header.Set("X-OpenAI-Target-Path", path)
	request.Header.Set("X-OpenAI-Target-Route", path)
	response, err := c.http.Do(request)
	if err != nil {
		return conversationState{}, fmt.Errorf("read conversation history: %w", err)
	}
	defer closeReadBody(response.Body)
	if response.StatusCode != fhttp.StatusOK {
		return conversationState{}, fmt.Errorf("conversation history returned HTTP %d", response.StatusCode)
	}
	body, err := readResponseBody(response)
	if err != nil {
		return conversationState{}, fmt.Errorf("read conversation history response: %w", err)
	}
	var result conversationState
	if err := json.Unmarshal(body, &result); err != nil {
		return conversationState{}, fmt.Errorf("conversation history returned invalid JSON")
	}
	if result.CurrentNode == "" {
		return conversationState{}, fmt.Errorf("conversation history has no current node")
	}
	return result, nil
}

// confirmTurn checks server-side persistence, including the exact user message
// ID, before reporting success. A broken stream is never resubmitted.
func (c *Client) confirmTurn(
	ctx context.Context,
	stream streamResult,
	payload conversationPayload,
	model, profile string,
) (Result, error) {
	if stream.ConversationID == "" {
		return Result{}, fmt.Errorf("send outcome unknown: no conversation ID; not retried")
	}
	if payload.ConversationID != nil && stream.ConversationID != *payload.ConversationID {
		return Result{}, fmt.Errorf("send outcome unknown: conversation ID changed; not retried")
	}
	history, err := c.conversation(ctx, stream.ConversationID)
	if err != nil {
		return Result{}, fmt.Errorf("send may have succeeded; verify history before retrying: %w", err)
	}
	sentID := payload.Messages[len(payload.Messages)-1].ID
	attached := false
	var answer savedMessage
	for _, message := range history.Messages {
		if message.ID == sentID && message.Author.Role == "user" {
			attached = true
		}
		final := message.Author.Role == "assistant" && (message.Channel == "final" || message.Channel == "")
		if message.ID == history.CurrentNode && final && message.Status == "finished_successfully" {
			answer = message
		}
	}
	text := []string{}
	for _, part := range answer.Content.Parts {
		var value string
		if json.Unmarshal(part, &value) == nil {
			text = append(text, value)
		}
	}
	reply := strings.Join(text, "\n")
	if !attached || strings.TrimSpace(reply) == "" {
		return Result{}, fmt.Errorf("send outcome unknown: no persisted completed reply for this turn; not retried")
	}
	return Result{
		Text: reply, ConversationID: stream.ConversationID, ParentMessageID: answer.ID,
		BrowserID: profile, Model: model, UpstreamStatus: fhttp.StatusOK, UpstreamPath: conversationPath,
	}, nil
}
