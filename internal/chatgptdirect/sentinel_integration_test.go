package chatgptdirect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
)

func newTestHttpClient() tlsclient.HttpClient {
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger())
	if err != nil {
		panic(err)
	}
	return client
}

func writeChatGPTCookie(t *testing.T, dir, value string) {
	t.Helper()
	content := "# Netscape HTTP Cookie File\n" +
		".chatgpt.com\tTRUE\t/\tFALSE\t1999999999\t__Secure-next.session.token\t" + value + "\n"
	if err := os.WriteFile(filepath.Join(dir, "chatgpt.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}
}

// TestSentinelPrepareFlow drives the full native sentinel pipeline (session →
// warmup → challenge scrape → prepare PoW → /prepare → proof PoW → /finalize)
// against a fake upstream, asserting evidence binding and per-turn preparation.
func TestSentinelPrepareFlow(t *testing.T) {
	var sessionHits int32
	future := time.Now().Unix() + 3600
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/session", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&sessionHits, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "jwt-token-xyz",
			"account":     map[string]string{"id": "acct-123"},
			"expires":     future,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><script src="https://chatgpt.com/cdn-cgi/challenge-platform/scripts/jsd/api.js?onload=jsdOnload"></script><meta name="build" content="deadbeefcafebabe1234567890abcdef12345678"></html>`)
	})
	for _, p := range []string{"/backend-api/me", "/backend-api/conversations", "/backend-api/models"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "[]")
		})
	}
	mux.HandleFunc("/backend-api/sentinel/chat-requirements/prepare", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"prepare_token": "prep-token-abc",
			"turnstile":     map[string]string{"dx": "dx-value"},
			"proofofwork":   map[string]string{"seed": "0.123456789012345", "difficulty": "0fffff"},
		})
	})
	mux.HandleFunc("/backend-api/sentinel/chat-requirements/finalize", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["proofofwork"] != "fixture-proof" || body["turnstile"] != "fixture-vm" {
			t.Error("finalize must receive original evidence")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":     "requirements-jwt",
			"expire_at": future - 600,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	dir := t.TempDir()
	writeChatGPTCookie(t, dir, "SESSIONVALUE")

	client := newSentinelClient(newTestHttpClient(), dir, server.URL)
	client.node = fixtureProofEngine{}
	artifacts, err := client.prepare(context.Background(), "default")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	checks := map[string]string{
		"authorization":                           "Bearer jwt-token-xyz",
		"chatgpt-account-id":                      "acct-123",
		"openai-sentinel-chat-requirements-token": "requirements-jwt",
		"oai-language":                            "en-US",
		"origin":                                  "https://chatgpt.com",
		"referer":                                 "https://chatgpt.com/",
	}
	for name, want := range checks {
		if got := artifacts.Headers[name]; got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
	if artifacts.Cookies != "__Secure-next.session.token=SESSIONVALUE" {
		t.Errorf("cookies = %q", artifacts.Cookies)
	}
	if artifacts.UserAgent != sentinelUserAgent {
		t.Errorf("user agent = %q", artifacts.UserAgent)
	}
	if artifacts.Context.AcceptLanguage != sentinelAcceptLanguage {
		t.Error("sec-ch-ua not propagated to context")
	}
	if len(artifacts.PrepareHeaders) != 0 {
		t.Error("native client must not require browser preparation")
	}

	// A second send must get fresh credentials and requirements; no token reuse.
	if _, err := client.prepare(context.Background(), "default"); err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if got := atomic.LoadInt32(&sessionHits); got != 2 {
		t.Errorf("session endpoint hit %d times, want 2 (fresh preparation)", got)
	}
}

type fixtureProofEngine struct{}

func (fixtureProofEngine) initialProof(context.Context) (string, error) {
	return "fixture-initial", nil
}
func (fixtureProofEngine) enforcementEvidence(_ context.Context, _ string, initial string) (string, string, error) {
	if initial != "fixture-initial" {
		return "", "", fmt.Errorf("initial proof changed")
	}
	return "fixture-proof", "fixture-vm", nil
}

// TestSentinelPrepareCookieErrors verifies the native path fails cleanly when no
// usable cookie is available, without contacting the network.
func TestSentinelPrepareCookieErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be contacted without a cookie")
	}))
	defer server.Close()

	empty := t.TempDir()
	client := newSentinelClient(newTestHttpClient(), empty, server.URL)
	if _, err := client.prepare(context.Background(), "default"); err == nil {
		t.Error("expected error when CHATGPT_COOKIE_DIR has no cookie files")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client = newSentinelClient(newTestHttpClient(), dir, server.URL)
	if _, err := client.prepare(context.Background(), "default"); err == nil {
		t.Error("expected error when the cookie file has no chatgpt.com cookies")
	}
}
