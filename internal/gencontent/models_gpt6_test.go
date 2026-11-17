package gencontent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGPT6ModelsAreListedAndRetrievable(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.openAIModels(response, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var listing struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, model := range listing.Data {
		listed[model.ID] = true
	}
	for _, name := range []string{"chatgpt/gpt-6", "chatgpt/gpt-6-thinking-standard", "chatgpt/gpt-6-thinking-extended", "chatgpt/gpt-5.6-thinking-standard"} {
		if !listed[name] {
			t.Fatalf("model not listed: %s", name)
		}
		response = httptest.NewRecorder()
		server.openAIModels(response, httptest.NewRequest(http.MethodGet, "/v1/models/"+name, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("model %s returned %d", name, response.Code)
		}
	}
}
