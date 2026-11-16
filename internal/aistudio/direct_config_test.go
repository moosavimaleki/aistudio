package aistudio

import (
	"path/filepath"
	"testing"
)

func TestDirectChatStartupDoesNotRequireGoogleCookies(t *testing.T) {
	t.Setenv("CHATGPT_MODE", "direct")
	t.Setenv("AISTUDIO_UPSTREAM_CONFIG", "../../config/upstream.yaml")
	t.Setenv("AISTUDIO_COOKIE_DIR", filepath.Join(t.TempDir(), "missing"))
	settings, err := LoadSettings(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.CookieHeader != "" {
		t.Fatal("direct startup must not load Google credentials")
	}
}
