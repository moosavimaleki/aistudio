package aistudio

import "fmt"

type RuntimeConfig struct {
	APIKey             string `json:"apiKey"`
	VisitID            string `json:"visitId"`
	AuthUser           string `json:"authUser"`
	AttestationEnabled bool   `json:"attestationEnabled"`
}

type GenerateInput struct {
	Model             string
	Prompt            string
	Contents          []any
	History           []any
	LatestUserTurn    any
	GenerationConfig  map[string]any
	SafetySettings    any
	SystemInstruction any
	Tools             []any
	ContinuationToken any
	ToolContext       any
}

type GenerateResult struct {
	FinalText            string `json:"finalText"`
	Chunks               []any  `json:"chunks"`
	ModelParts           []any  `json:"modelParts"`
	FinishReason         any    `json:"finishReason,omitempty"`
	Usage                any    `json:"usage,omitempty"`
	ConversationMetadata any    `json:"conversationMetadata,omitempty"`
}

type ClientError struct {
	Message      string         `json:"error"`
	Phase        string         `json:"phase"`
	Status       int            `json:"status,omitempty"`
	ResponseBody string         `json:"responseBody,omitempty"`
	Diagnostics  map[string]any `json:"diagnostics,omitempty"`
	Retryable    bool           `json:"-"`
}

func (e *ClientError) Error() string { return e.Message }

func NewError(phase, message string) *ClientError {
	return &ClientError{Message: message, Phase: phase}
}

func ResponseError(phase string, status int, body string) *ClientError {
	return &ClientError{Message: fmt.Sprintf("%s failed with HTTP %d", phase, status), Phase: phase, Status: status, ResponseBody: body}
}
