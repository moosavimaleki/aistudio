package chatgptdirect

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
)

// Legacy work-token encoding retained for compatibility fixtures.
// The active Black Box runtime delegates calculations to the SDK.
const (
	initialTokenPrefix = "gAAAAAC" // prepare prekey token
	workTokenPrefix    = "gAAAAAB" // conversation work token
	workTokenSuffix    = "~S"      // conversation work-token trailer
	initialWorkTarget  = "0fffff"  // initial work target (empty seed)
)

// Runtime metadata fields used by the legacy initialization format.
var (
	runtimeScreenSizes   = []int{3000, 4000, 3120, 4160}
	runtimeCores         = []int{8, 16, 24, 32}
	runtimeNavigatorKeys = []string{
		"webdriver−false", "geolocation", "languages", "language", "platform",
		"userAgent", "vendor", "hardwareConcurrency", "deviceMemory",
		"permissions", "plugins", "mediaDevices",
	}
	runtimeDocumentKeys = []string{
		"_reactListeningkfj3eavmks", "_reactListeningo743lnnpvdg",
		"location", "scrollingElement", "documentElement",
	}
	runtimeWindowKeys = []string{
		"webpackChunk_N_E", "__NEXT_DATA__", "chrome", "history",
		"screen", "navigation", "scrollX", "scrollY",
	}
)

// buildRuntimeConfig builds the 18-field legacy initialization context.
// Slot 3 is the mutable work counter; slots 13 and 17 reconstruct epoch time.
func buildRuntimeConfig(userAgent, jsdScript, dplHash, uuid string) []any {
	now := time.Now()
	epochMillis := float64(now.UnixMilli())
	perfNow := 1000.0 + rand.Float64()*59000.0 // ms since (simulated) load
	return []any{
		runtimeScreenSizes[rand.IntN(len(runtimeScreenSizes))],
		now.UTC().Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)"),
		4294705152,
		0, // nonce, set by computeWorkToken
		userAgent,
		jsdScript,
		dplHash,
		"en-US",
		"en-US,en",
		0,
		runtimeNavigatorKeys[rand.IntN(len(runtimeNavigatorKeys))],
		runtimeDocumentKeys[rand.IntN(len(runtimeDocumentKeys))],
		runtimeWindowKeys[rand.IntN(len(runtimeWindowKeys))],
		perfNow,
		uuid,
		"",
		runtimeCores[rand.IntN(len(runtimeCores))],
		epochMillis - perfNow,
	}
}

// configBase64 renders config as compact JSON with no HTML escaping and literal
// non-ASCII (matching JS JSON.stringify), then returns its standard base64.
func encodeRuntimeConfig(cfg []any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cfg); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// workDigest computes the legacy 32-bit work digest.
func workDigest(value string) string {
	h := uint32(2166136261)
	for i := 0; i < len(value); i++ {
		h ^= uint32(value[i])
		h *= 16777619
	}
	h ^= h >> 16
	h *= 2246822507
	h ^= h >> 13
	h *= 3266489909
	h ^= h >> 16
	return fmt.Sprintf("%08x", h)
}

func computeWorkToken(cfg []any, seed, target, prefix, suffix string, maxNonce int) (string, error) {
	if maxNonce <= 0 {
		maxNonce = 1 << 20
	}
	for nonce := range maxNonce {
		cfg[3] = nonce
		b64, err := encodeRuntimeConfig(cfg)
		if err != nil {
			return "", err
		}
		digest := workDigest(seed + b64)
		if digest[:len(target)] <= target {
			return prefix + b64 + suffix, nil
		}
	}
	return "", fmt.Errorf("black box work computation: no nonce satisfies target %q in %d tries", target, maxNonce)
}
