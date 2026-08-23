package browserinterface

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadNetscapeCookiesFiltersChatGPTDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	text := "# Netscape HTTP Cookie File\n" +
		"#HttpOnly_.chatgpt.com\tTRUE\t/\tTRUE\t0\tsecure_session\tvalue\n" +
		".example.com\tTRUE\t/\tTRUE\t0\tignored\tvalue\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}

	cookies, err := readNetscapeCookies(path, "chatgpt.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 1 || cookies[0].Name != "secure_session" || !cookies[0].HTTPOnly {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}
}
