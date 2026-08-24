package chatgptdirect

import (
	"fmt"
	"strings"
	"time"
)

type conversationPayload struct {
	Action                 string                  `json:"action"`
	Messages               []conversationMessage   `json:"messages"`
	ConversationID         *string                 `json:"conversation_id,omitempty"`
	ParentMessageID        string                  `json:"parent_message_id"`
	Model                  string                  `json:"model"`
	ClientPrepareState     string                  `json:"client_prepare_state"`
	TimezoneOffsetMin      int                     `json:"timezone_offset_min"`
	Timezone               string                  `json:"timezone"`
	ConversationMode       map[string]string       `json:"conversation_mode"`
	EnableMessageFollowups bool                    `json:"enable_message_followups"`
	SystemHints            []string                `json:"system_hints"`
	ModelResponseContracts []modelResponseContract `json:"model_response_contracts"`
	SupportsBuffering      bool                    `json:"supports_buffering"`
	SupportedEncodings     []string                `json:"supported_encodings"`
	ClientContextualInfo   clientContext           `json:"client_contextual_info"`
	ParagenDisplayOverride string                  `json:"paragen_cot_summary_display_override"`
	ForceParallelSwitch    string                  `json:"force_parallel_switch"`
	ThinkingEffort         string                  `json:"thinking_effort,omitempty"`
	LocalFunctionNames     []string                `json:"local_function_names"`
}

type conversationMessage struct {
	ID         string            `json:"id"`
	Author     map[string]string `json:"author"`
	CreateTime float64           `json:"create_time"`
	Content    messageContent    `json:"content"`
	Metadata   any               `json:"metadata,omitempty"`
}

type messageContent struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

type modelResponseContract struct {
	ID              string   `json:"id"`
	ProtocolVersion int      `json:"protocol_version"`
	Presets         []string `json:"presets"`
}

type clientContext struct {
	IsDarkMode                    bool    `json:"is_dark_mode"`
	TimeSinceLoaded               int     `json:"time_since_loaded"`
	PageHeight                    int     `json:"page_height"`
	PageWidth                     int     `json:"page_width"`
	PixelRatio                    float64 `json:"pixel_ratio"`
	ScreenHeight                  int     `json:"screen_height"`
	ScreenWidth                   int     `json:"screen_width"`
	AppName                       string  `json:"app_name"`
	HasWebPushCapabilities        bool    `json:"has_web_push_capabilities"`
	WebPushNotificationPermission string  `json:"web_push_notification_permission"`
}

func buildPayload(
	input Input,
	model Model,
	parentMessageID string,
	context BrowserContext,
) (conversationPayload, string, error) {
	messages, prompt, err := buildMessages(input.Messages, input.ParentMessageID != "")
	if err != nil {
		return conversationPayload{}, "", err
	}
	timezone := context.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	return conversationPayload{
		Action:                 "next",
		Messages:               messages,
		ConversationID:         optionalString(input.ConversationID),
		ParentMessageID:        parentMessageID,
		Model:                  model.Slug,
		ClientPrepareState:     "success",
		TimezoneOffsetMin:      context.TimezoneOffsetMin,
		Timezone:               timezone,
		ConversationMode:       map[string]string{"kind": "primary_assistant"},
		EnableMessageFollowups: true,
		SystemHints:            []string{},
		ModelResponseContracts: []modelResponseContract{{
			ID:              "photo_upload_action.v1",
			ProtocolVersion: 1,
			Presets:         []string{"cap:image", "cap:file", "placement:end"},
		}},
		SupportsBuffering:      true,
		SupportedEncodings:     []string{"v1"},
		ClientContextualInfo:   newClientContext(context),
		ParagenDisplayOverride: "allow",
		ForceParallelSwitch:    "auto",
		ThinkingEffort:         model.ThinkingEffort,
		LocalFunctionNames:     []string{"local.continue_in_work"},
	}, prompt, nil
}

func buildMessages(input []Message, continuation bool) ([]conversationMessage, string, error) {
	if len(input) == 0 {
		return nil, "", fmt.Errorf("messages is required")
	}
	current := -1
	for index := len(input) - 1; index >= 0; index-- {
		if input[index].Role == "user" {
			current = index
			break
		}
	}
	if current < 0 || strings.TrimSpace(input[current].Content) == "" {
		return nil, "", fmt.Errorf("a non-empty user message is required")
	}
	result := []conversationMessage{}
	context := ""
	if !continuation {
		context = historyContext(input[:current])
	}
	if context != "" {
		message, err := newMessage("system", context, nil)
		if err != nil {
			return nil, "", err
		}
		result = append(result, message)
	}
	metadata := map[string]any{
		"selected_sources":       []string{},
		"serialization_metadata": map[string]any{"custom_symbol_offsets": []int{}},
	}
	message, err := newMessage("user", input[current].Content, metadata)
	if err != nil {
		return nil, "", err
	}
	return append(result, message), input[current].Content, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func newMessage(role, text string, metadata any) (conversationMessage, error) {
	id, err := newUUID()
	if err != nil {
		return conversationMessage{}, err
	}
	return conversationMessage{
		ID:         id,
		Author:     map[string]string{"role": role},
		CreateTime: float64(time.Now().UnixMilli()) / 1000,
		Content:    messageContent{ContentType: "text", Parts: []string{text}},
		Metadata:   metadata,
	}, nil
}

func historyContext(messages []Message) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		role := strings.Title(message.Role)
		if message.Role == "developer" {
			role = "System"
		}
		parts = append(parts, role+":\n"+text)
	}
	return strings.Join(parts, "\n\n")
}

func newClientContext(value BrowserContext) clientContext {
	permission := value.WebPushNotificationPermission
	if permission == "" {
		permission = "default"
	}
	return clientContext{
		IsDarkMode:                    value.IsDarkMode,
		TimeSinceLoaded:               value.TimeSinceLoaded,
		PageHeight:                    value.PageHeight,
		PageWidth:                     value.PageWidth,
		PixelRatio:                    value.PixelRatio,
		ScreenHeight:                  value.ScreenHeight,
		ScreenWidth:                   value.ScreenWidth,
		AppName:                       "chatgpt.com",
		HasWebPushCapabilities:        value.HasWebPushCapabilities,
		WebPushNotificationPermission: permission,
	}
}
