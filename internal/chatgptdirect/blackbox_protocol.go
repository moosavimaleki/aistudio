package chatgptdirect

import "encoding/json"

// Wire names belong to the server contract and are independent of local names.
const (
	requirementsPreparePath  = "/backend-api/sentinel/chat-requirements/prepare"
	requirementsFinalizePath = "/backend-api/sentinel/chat-requirements/finalize"
	requirementsTokenHeader  = "openai-sentinel-chat-requirements-token"
	workTokenHeader          = "openai-sentinel-proof-token"
	runtimeTokenHeader       = "openai-sentinel-turnstile-token"
)

type prepareResponse struct {
	PrepareToken string `json:"prepare_token"`
	Token        string `json:"token"`
	ForceLogin   bool   `json:"force_login"`
	Runtime      struct {
		Required bool   `json:"required"`
		DX       string `json:"dx"`
	} `json:"turnstile"`
	Work struct {
		Required bool `json:"required"`
	} `json:"proofofwork"`
	raw json.RawMessage
}
