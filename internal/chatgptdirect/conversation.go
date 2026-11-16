package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

func (c *Client) conversation(ctx context.Context, id, profile string) (conversationState, error) {
	for attempt := 0; ; attempt++ {
		state, wait, err := c.readConversation(ctx, id, profile)
		if err == nil || wait <= 0 || attempt >= 2 {
			return state, err
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return conversationState{}, ctx.Err()
		}
	}
}

func (c *Client) readConversation(ctx context.Context, id, profile string) (conversationState, time.Duration, error) {
	if c.sentinel == nil {
		return conversationState{}, 0, fmt.Errorf("history access requires a native direct client")
	}
	headers, err := c.sentinel.authHeaders(ctx, profile)
	if err != nil {
		return conversationState{}, 0, err
	}
	path := "/backend-api/conversations/" + url.PathEscape(id)
	request, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return conversationState{}, 0, fmt.Errorf("create conversation history request: %w", err)
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
		return conversationState{}, 0, fmt.Errorf("read conversation history: %w", err)
	}
	defer closeReadBody(response.Body)
	if response.StatusCode != fhttp.StatusOK {
		wait := time.Duration(0)
		if response.StatusCode == 429 {
			wait = historyRetryDelay(response.Header.Get("Retry-After"), time.Now())
		}
		return conversationState{}, wait, fmt.Errorf("conversation history returned HTTP %d", response.StatusCode)
	}
	body, err := readResponseBody(response)
	if err != nil {
		return conversationState{}, 0, fmt.Errorf("read conversation history response: %w", err)
	}
	var result conversationState
	if err := json.Unmarshal(body, &result); err != nil {
		return conversationState{}, 0, fmt.Errorf("conversation history returned invalid JSON")
	}
	if result.CurrentNode == "" {
		return conversationState{}, 0, fmt.Errorf("conversation history has no current node")
	}
	return result, 0, nil
}

func historyRetryDelay(header string, now time.Time) time.Duration {
	seconds, err := strconv.Atoi(header)
	if err != nil {
		date, parseErr := http.ParseTime(header)
		if parseErr != nil {
			return 0
		}
		seconds = int(date.Sub(now).Seconds())
	}
	if seconds < 0 || seconds > 60 {
		return 0
	}
	return time.Duration(seconds+1) * time.Second
}

// confirmTurn accepts a fully completed final SSE snapshot, or checks persisted
// history when that acknowledgement is missing. A send is never resubmitted.
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
	// A complete final assistant snapshot plus the SSE terminator is the
	// upstream's success acknowledgement. Do not query history after every
	// turn: that endpoint has a separate, much lower request limit.
	if stream.Done && stream.Final && stream.MessageID != "" && strings.TrimSpace(stream.Text) != "" {
		if c.parents != nil {
			c.parents[profile+"\x00"+stream.ConversationID] = stream.MessageID
		}
		return Result{Text: stream.Text, ConversationID: stream.ConversationID, ParentMessageID: stream.MessageID,
			BrowserID: profile, Model: model, UpstreamStatus: fhttp.StatusOK, UpstreamPath: conversationPath}, nil
	}
	history, err := c.conversation(ctx, stream.ConversationID, profile)
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
	if c.parents != nil {
		c.parents[profile+"\x00"+stream.ConversationID] = answer.ID
	}
	return Result{
		Text: reply, ConversationID: stream.ConversationID, ParentMessageID: answer.ID,
		BrowserID: profile, Model: model, UpstreamStatus: fhttp.StatusOK, UpstreamPath: conversationPath,
	}, nil
}
