package chatgptdirect

import (
	"testing"
	"time"
)

// TestParseExpires covers the session `expires` field, which live chatgpt.com
// returns as an RFC3339 string while older/other clients may send unix seconds.
func TestParseExpires(t *testing.T) {
	rfc := "2027-02-13T11:19:31.096Z"
	wantRFC, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		t.Fatalf("fixture parse: %v", err)
	}
	cases := []struct {
		name    string
		raw     string
		want    time.Time
		wantErr bool
	}{
		{"rfc3339 string", `"` + rfc + `"`, wantRFC, false},
		{"unix seconds", `1763000000`, time.Unix(1763000000, 0), false},
		{"null", `null`, time.Time{}, false},
		{"empty", ``, time.Time{}, false},
		{"garbage string", `"not-a-date"`, time.Time{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExpires([]byte(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseExpires(%s) = %v, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExpires(%s): %v", tc.raw, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("parseExpires(%s) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
