package chatgptdirect

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildTurnPreparePayload(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		parentID       string
	}{
		{name: "new conversation"},
		{name: "continuation", conversationID: "conversation-1", parentID: "message-1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := ResolveModel("chatgpt/gpt-5.6-pro")
			if err != nil {
				t.Fatal(err)
			}
			input := Input{
				Model:           model.Name,
				ConversationID:  test.conversationID,
				ParentMessageID: test.parentID,
				Messages:        []Message{{Role: "user", Content: "hello"}},
			}
			parentID := test.parentID
			if parentID == "" {
				parentID = "client-created-root"
			}
			final, _, err := buildPayload(input, model, parentID, BrowserContext{
				Timezone:                      "Asia/Tehran",
				TimezoneOffsetMin:             -210,
				HasWebPushCapabilities:        true,
				WebPushNotificationPermission: "default",
			})
			if err != nil {
				t.Fatal(err)
			}
			prepare, err := buildTurnPreparePayload(final)
			if err != nil {
				t.Fatal(err)
			}
			if prepare.ClientPrepareState != "success" ||
				prepare.ClientPrepareDispatch != "immediate" ||
				prepare.ClientPrepareSource != "context_change" {
				t.Fatalf("unexpected prepare state: %#v", prepare)
			}
			if prepare.PartialQuery.Content.Parts[0] != "hello" ||
				prepare.ClientContextualInfo.AppName != "chatgpt.com" {
				t.Fatalf("unexpected browser prepare payload: %#v", prepare)
			}
			body, err := json.Marshal(prepare)
			if err != nil {
				t.Fatal(err)
			}
			text := string(body)
			for _, forbidden := range []string{"\"messages\"", "\"enable_message_followups\"", "\"force_parallel_switch\""} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("prepare payload contains final-only field %s: %s", forbidden, text)
				}
			}
		})
	}
}

func TestPrepareRequestUsesCapturedBrowserHeaders(t *testing.T) {
	artifacts := Artifacts{
		PrepareHeaders: map[string]string{
			"oai-client-version":          "prod-test",
			"x-oai-turn-trace-id":         "trace-1",
			"x-openai-target-path":        preparePath,
			"x-openai-target-route":       preparePath,
			"x-oai-is-client-observation": "observation-1",
		},
		Cookies:   "session=test",
		UserAgent: "Chrome/Test",
	}
	request, err := newUpstreamRequest(
		context.Background(),
		preparePath,
		artifacts.PrepareHeaders,
		artifacts,
		[]byte(`{"action":"next"}`),
		"*/*",
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.Header.Get("Accept") != "*/*" || request.Header.Get("Cookie") != artifacts.Cookies {
		t.Fatalf("unexpected prepare transport headers: %#v", request.Header)
	}
	if request.Header.Get("X-OpenAI-Target-Path") != preparePath ||
		request.Header.Get("X-OAI-Is-Client-Observation") != "observation-1" {
		t.Fatalf("captured prepare headers were not preserved: %#v", request.Header)
	}
}
