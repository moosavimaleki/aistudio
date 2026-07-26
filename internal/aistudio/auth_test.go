package aistudio

import (
	"strings"
	"testing"
	"time"
)

func TestAuthorizationUsesTimestampedSAPISIDProofs(t *testing.T) {
	auth, err := NewAuthContext(
		"https://aistudio.google.com",
		"SAPISID=primary; __Secure-1PAPISID=one; __Secure-3PAPISID=three",
	)
	if err != nil {
		t.Fatal(err)
	}
	auth.Clock = func() time.Time { return time.Unix(1_700_000_000, 0) }

	header := auth.Authorization()
	for _, expected := range []string{
		"SAPISIDHASH 1700000000_" + sha1Hex("1700000000 primary https://aistudio.google.com"),
		"SAPISID1PHASH 1700000000_" + sha1Hex("1700000000 one https://aistudio.google.com"),
		"SAPISID3PHASH 1700000000_" + sha1Hex("1700000000 three https://aistudio.google.com"),
	} {
		if !strings.Contains(header, expected) {
			t.Fatalf("Authorization %q does not contain %q", header, expected)
		}
	}
}
