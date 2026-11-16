package chatgptdirect

import "testing"

func TestParseChatGPTCookies(t *testing.T) {
	netscape := "# Netscape HTTP Cookie File\n" +
		"#HttpOnly_.chatgpt.com\tTRUE\t/\tFALSE\t1999999999\t__Secure-next.session.token\tSESSIONTOKEN\n" +
		".chatgpt.com\tTRUE\t/\tFALSE\t1999999999\t__cf_bm\tCFBVALUE\n" +
		".example.com\tTRUE\t/\tFALSE\t1999999999\tother-cookie\tshould-be-skipped\n" +
		".chatgpt.com\tTRUE\t/\tTRUE\t1999999999\tsome-secure\tsecureval\n"
	header, err := parseChatGPTCookies(netscape, "chatgpt.com")
	if err != nil {
		t.Fatalf("parseChatGPTCookies: %v", err)
	}
	want := "__Secure-next.session.token=SESSIONTOKEN; __cf_bm=CFBVALUE; some-secure=secureval"
	if header != want {
		t.Fatalf("cookie header mismatch:\n got=%s\nwant=%s", header, want)
	}
}

func TestParseChatGPTCookiesDedup(t *testing.T) {
	// The first value per name wins; a later duplicate name is ignored.
	netscape := ".chatgpt.com\tTRUE\t/\tFALSE\t1\ttoken\tfirst\n" +
		".chatgpt.com\tTRUE\t/\tFALSE\t2\ttoken\tsecond\n"
	header, err := parseChatGPTCookies(netscape, "chatgpt.com")
	if err != nil {
		t.Fatalf("parseChatGPTCookies: %v", err)
	}
	if header != "token=first" {
		t.Fatalf("expected first value to win, got %q", header)
	}
}

func TestParseChatGPTCookiesEmpty(t *testing.T) {
	if _, err := parseChatGPTCookies("# Netscape HTTP Cookie File\n", "chatgpt.com"); err == nil {
		t.Fatal("expected error when no chatgpt.com cookies present")
	}
}

func TestDeriveDeviceIDStable(t *testing.T) {
	a := deriveDeviceID("cookie-A")
	b := deriveDeviceID("cookie-A")
	c := deriveDeviceID("cookie-B")
	if a != b {
		t.Fatalf("device id not stable: %s vs %s", a, b)
	}
	if a == c {
		t.Fatalf("different cookies produced the same device id")
	}
	if len(a) != 36 {
		t.Fatalf("device id not in UUID form (36 chars): %q", a)
	}
}

func TestCookieIndexFor(t *testing.T) {
	if got := cookieIndexFor("", 3); got != 0 {
		t.Fatalf("default id → slot %d, want 0", got)
	}
	if got := cookieIndexFor("chatgpt", 3); got != 0 {
		t.Fatalf("chatgpt id → slot %d, want 0", got)
	}
	if got := cookieIndexFor("browser2", 3); got != 1 {
		t.Fatalf("browser2 id → slot %d, want 1", got)
	}
	if got := cookieIndexFor("x", 1); got != 0 {
		t.Fatalf("single file → slot %d, want 0", got)
	}
}
