package aistudio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type videoTestTransport func(*http.Request) (*http.Response, error)

func (f videoTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVideoTransportUsesFreshFactoryTokenAndNeverReplays(t *testing.T) {
	t.Setenv("AISTUDIO_UPSTREAM_CONFIG", "../../config/upstream.yaml")
	for _, status := range []int{200, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			tokenCalls, videoCalls := 0, 0
			httpClient, _ := NewHTTPClient("")
			httpClient.client.Transport = videoTestTransport(func(request *http.Request) (*http.Response, error) {
				response := httptest.NewRecorder()
				if strings.HasSuffix(request.URL.Path, "/GetAiStudioBenefitTier") {
					response.WriteString(`[1]`)
					return response.Result(), nil
				}
				if request.URL.Path == "/get-token" {
					tokenCalls++
					var body map[string]any
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					pending := body["generateRequest"].(map[string]any)
					payload := pending["payload"].([]any)
					if payload[4] != nil || !strings.HasSuffix(pending["url"].(string), "/CreateInteractionStream") {
						t.Fatal("factory did not receive unsigned video request")
					}
					response.WriteString(`{"token":"fresh-native-token"}`)
					return response.Result(), nil
				}
				videoCalls++
				var payload []any
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload[4] != "fresh-native-token" || request.Header.Get("Authorization") == "" {
					t.Fatal("video request lacks fresh token or authorization")
				}
				if request.Header.Get("X-AIStudio-G1-Tier") != "TIER1" {
					t.Fatal("video request lacks the account's subscription tier")
				}
				response.WriteHeader(status)
				if status == 200 {
					response.WriteString(videoStreamFixture(base64.StdEncoding.EncodeToString(syntheticMP4())))
				} else {
					response.WriteString(`{"error":"test upstream failure"}`)
				}
				return response.Result(), nil
			})
			tab, err := NewTab(Settings{
				OriginURL: "https://aistudio.google.com", CookieHeader: "SAPISID=test",
			}, httpClient, "video-test")
			if err != nil {
				t.Fatal(err)
			}
			tab.State = TabReady
			tab.Runtime = RuntimeConfig{APIKey: "test", AuthUser: "0", AttestationEnabled: true}
			tab.TokenFactory = &StagingTokenFactory{
				HTTP: httpClient, URL: "http://factory/get-token", Auth: tab.Auth, Runtime: tab.Runtime,
			}
			result, err := tab.GenerateVideo(context.Background(), VideoInput{Model: VideoModel, Prompt: "test"})
			if (status != 200) != (err != nil) {
				t.Fatalf("status=%d error=%v", status, err)
			}
			if status == 200 && !IsMP4(result.Data) {
				t.Fatal("no MP4 returned")
			}
			if status != 200 && err.(*ClientError).Status != status {
				t.Fatal("upstream error status lost")
			}
			if tokenCalls != 1 || videoCalls != 1 || tab.State != TabReady {
				t.Fatalf("token calls=%d video calls=%d state=%s", tokenCalls, videoCalls, tab.State)
			}
		})
	}
}
