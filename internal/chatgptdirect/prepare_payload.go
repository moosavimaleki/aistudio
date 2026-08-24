package chatgptdirect

import "fmt"

type turnPreparePayload struct {
	Action                 string                  `json:"action"`
	ConversationID         *string                 `json:"conversation_id,omitempty"`
	ParentMessageID        string                  `json:"parent_message_id"`
	Model                  string                  `json:"model"`
	ClientPrepareState     string                  `json:"client_prepare_state"`
	ClientPrepareDispatch  string                  `json:"client_prepare_dispatch"`
	ClientPrepareSource    string                  `json:"client_prepare_source"`
	TimezoneOffsetMin      int                     `json:"timezone_offset_min"`
	Timezone               string                  `json:"timezone"`
	ConversationMode       map[string]string       `json:"conversation_mode"`
	SystemHints            []string                `json:"system_hints"`
	ModelResponseContracts []modelResponseContract `json:"model_response_contracts"`
	PartialQuery           partialQuery            `json:"partial_query"`
	SupportsBuffering      bool                    `json:"supports_buffering"`
	SupportedEncodings     []string                `json:"supported_encodings"`
	ClientContextualInfo   prepareClientContext    `json:"client_contextual_info"`
	ThinkingEffort         string                  `json:"thinking_effort,omitempty"`
	LocalFunctionNames     []string                `json:"local_function_names"`
}

type partialQuery struct {
	ID      string            `json:"id"`
	Author  map[string]string `json:"author"`
	Content messageContent    `json:"content"`
}

type prepareClientContext struct {
	AppName                       string `json:"app_name"`
	HasWebPushCapabilities        bool   `json:"has_web_push_capabilities"`
	WebPushNotificationPermission string `json:"web_push_notification_permission"`
}

func buildTurnPreparePayload(final conversationPayload) (turnPreparePayload, error) {
	if len(final.Messages) == 0 {
		return turnPreparePayload{}, fmt.Errorf("turn prepare requires a user message")
	}
	message := final.Messages[len(final.Messages)-1]
	return turnPreparePayload{
		Action:                 final.Action,
		ConversationID:         final.ConversationID,
		ParentMessageID:        final.ParentMessageID,
		Model:                  final.Model,
		ClientPrepareState:     "success",
		ClientPrepareDispatch:  "immediate",
		ClientPrepareSource:    "context_change",
		TimezoneOffsetMin:      final.TimezoneOffsetMin,
		Timezone:               final.Timezone,
		ConversationMode:       final.ConversationMode,
		SystemHints:            final.SystemHints,
		ModelResponseContracts: final.ModelResponseContracts,
		PartialQuery: partialQuery{
			ID:      message.ID,
			Author:  message.Author,
			Content: message.Content,
		},
		SupportsBuffering:  final.SupportsBuffering,
		SupportedEncodings: final.SupportedEncodings,
		ClientContextualInfo: prepareClientContext{
			AppName:                       final.ClientContextualInfo.AppName,
			HasWebPushCapabilities:        final.ClientContextualInfo.HasWebPushCapabilities,
			WebPushNotificationPermission: final.ClientContextualInfo.WebPushNotificationPermission,
		},
		ThinkingEffort:     final.ThinkingEffort,
		LocalFunctionNames: final.LocalFunctionNames,
	}, nil
}
