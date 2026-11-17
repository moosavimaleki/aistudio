package chatgptdirect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const workUserAgent = "TestUA/1.0"
const workRuntimeScript = "https://chatgpt.com/cdn-cgi/challenge-platform/scripts/jsd/api.js"
const workBuild = "ef0c3fb9"
const workID = "uuid-1234"

// Keep compatibility fixtures aligned with the legacy runtime format.
func TestBuildRuntimeConfigStructure(t *testing.T) {
	cfg := buildRuntimeConfig(workUserAgent, workRuntimeScript, workBuild, workID)
	if len(cfg) != 18 {
		t.Fatalf("config length = %d, want 18", len(cfg))
	}
	wants := map[int]any{
		2:  float64(4294705152),
		4:  workUserAgent,
		5:  workRuntimeScript,
		6:  workBuild,
		7:  "en-US",
		8:  "en-US,en",
		9:  float64(0),
		14: workID,
	}
	for idx, want := range wants {
		if !scalarEqual(cfg[idx], want) {
			t.Fatalf("cfg[%d] = %#v, want %#v", idx, cfg[idx], want)
		}
	}
	if !containsInt(runtimeScreenSizes, toInt(cfg[0])) {
		t.Fatalf("cfg[0] = %#v not in screen sizes", cfg[0])
	}
	if !containsStr(runtimeNavigatorKeys, cfg[10].(string)) {
		t.Fatalf("cfg[10] = %q not in navigator keys", cfg[10])
	}
	if !containsStr(runtimeDocumentKeys, cfg[11].(string)) {
		t.Fatalf("cfg[11] = %q not in document keys", cfg[11])
	}
	if !containsStr(runtimeWindowKeys, cfg[12].(string)) {
		t.Fatalf("cfg[12] = %q not in window keys", cfg[12])
	}
	if !containsInt(runtimeCores, toInt(cfg[16])) {
		t.Fatalf("cfg[16] = %#v not in cores", cfg[16])
	}
	// cfg[13] (performance.now) + cfg[17] (epoch offset) reconstruct the current
	// epoch-millis used by the initialization format.
	perfNow, _ := cfg[13].(float64)
	epochOffset, _ := cfg[17].(float64)
	if delta := int64(perfNow+epochOffset) - time.Now().UnixMilli(); delta < -2000 || delta > 2000 {
		t.Fatalf("cfg[13]+cfg[17] = %d ms off from now (%d)", delta, time.Now().UnixMilli())
	}
	// The navigator field preserves its U+2212 MINUS SIGN encoding.
	if !strings.Contains(runtimeNavigatorKeys[0], "\u2212") {
		t.Fatal("navigator probe[0] missing U+2212")
	}
}

func scalarEqual(a, b any) bool {
	if af, ok := a.(float64); ok {
		if bf, ok := b.(float64); ok {
			return af == bf
		}
	}
	if ai, ok := a.(int); ok {
		if bf, ok := b.(float64); ok {
			return float64(ai) == bf
		}
	}
	return a == b
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsStr(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return -1
}

func verifyWorkToken(b64, seed, target string) bool {
	digest := workDigest(seed + b64)
	return digest[:len(target)] <= target
}

func workNonce(b64 string) (int, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0, err
	}
	var cfg []any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return 0, err
	}
	number, ok := cfg[3].(float64)
	if !ok {
		return 0, fmt.Errorf("config[3] is not a JSON number")
	}
	return int(number), nil
}

func TestComputeWorkToken(t *testing.T) {
	cases := []struct {
		name   string
		seed   string
		target string
		prefix string
		suffix string
	}{
		{"prepare", "", "0fffff", initialTokenPrefix, ""},
		{"conversation", "0.8199128186806912", "078b22", workTokenPrefix, workTokenSuffix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build one config; the solver mutates cfg[3] in place, so the base
			// after solving IS the solved config. Reuse it for the minimality
			// scan rather than rebuilding (buildRuntimeConfig is randomized).
			base := buildRuntimeConfig(workUserAgent, workRuntimeScript, workBuild, workID)
			token, err := computeWorkToken(base, tc.seed, tc.target, tc.prefix, tc.suffix, 1<<20)
			if err != nil {
				t.Fatalf("computeWorkToken: %v", err)
			}
			if !strings.HasPrefix(token, tc.prefix) {
				t.Fatalf("token missing prefix %q: %s", tc.prefix, token[:8])
			}
			if tc.suffix != "" && !strings.HasSuffix(token, tc.suffix) {
				t.Fatalf("token missing suffix %q", tc.suffix)
			}
			body := strings.TrimSuffix(strings.TrimPrefix(token, tc.prefix), tc.suffix)

			if !verifyWorkToken(body, tc.seed, tc.target) {
				t.Fatalf("solved token does not satisfy target %q", tc.target)
			}
			nonce, err := workNonce(body)
			if err != nil {
				t.Fatalf("decode nonce: %v", err)
			}
			for n := 0; n < nonce; n++ {
				check := append([]any(nil), base...)
				check[3] = n
				b64, err := encodeRuntimeConfig(check)
				if err != nil {
					t.Fatalf("encodeRuntimeConfig: %v", err)
				}
				if verifyWorkToken(b64, tc.seed, tc.target) {
					t.Fatalf("nonce %d also satisfies target; solver returned %d (not minimal)", n, nonce)
				}
			}
		})
	}
}

func TestComputeWorkTokenExhausts(t *testing.T) {
	// A target no nonce can meet (leading 'f') must error, not hang forever.
	cfg := buildRuntimeConfig(workUserAgent, workRuntimeScript, workBuild, workID)
	if _, err := computeWorkToken(cfg, "", "000001", initialTokenPrefix, "", 2); err == nil {
		t.Fatal("expected error when no nonce satisfies within the limit")
	}
}
