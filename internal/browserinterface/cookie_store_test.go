package browserinterface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestPersistCookieFileWritesGoogleCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.txt")
	err := persistCookieFile(path, []*network.Cookie{
		{Name: "SID", Value: "session", Domain: ".google.com", Path: "/", Secure: true, HTTPOnly: true},
		{Name: "ignored", Value: "value", Domain: ".example.com", Path: "/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "#HttpOnly_.google.com\tTRUE\t/\tTRUE\t0\tSID\tsession") {
		t.Fatalf("unexpected cookie file: %q", text)
	}
	if strings.Contains(text, "example.com") {
		t.Fatalf("non-Google cookie was persisted: %q", text)
	}
}
