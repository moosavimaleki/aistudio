package aistudio

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type AuthContext struct {
	Origin, CookieHeader string
	Clock                func() time.Time
}

func NewAuthContext(origin, cookieHeader string) (*AuthContext, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid auth origin: %s", origin)
	}
	if cookieHeader == "" {
		return nil, fmt.Errorf("auth context requires a non-empty cookie header")
	}
	return &AuthContext{Origin: parsed.Scheme + "://" + parsed.Host, CookieHeader: cookieHeader, Clock: time.Now}, nil
}

func (a *AuthContext) Cookie(name string) string {
	prefix := name + "="
	for _, raw := range strings.Split(a.CookieHeader, ";") {
		pair := strings.TrimSpace(raw)
		if strings.HasPrefix(pair, prefix) {
			return strings.TrimPrefix(pair, prefix)
		}
	}
	return ""
}
func sha1Hex(value string) string { sum := sha1.Sum([]byte(value)); return hex.EncodeToString(sum[:]) }

func (a *AuthContext) Authorization() string {
	secure := strings.HasPrefix(a.Origin, "https:")
	primary := a.Cookie("APISID")
	scheme := "APISIDHASH"
	if secure {
		primary, scheme = a.Cookie("SAPISID"), "SAPISIDHASH"
		if primary == "" {
			primary = a.Cookie("__Secure-3PAPISID")
		}
	}
	if primary == "" {
		return ""
	}
	timestamp := fmt.Sprint(a.Clock().Unix())
	proof := func(cookie, name string) string {
		digest := sha1Hex(timestamp + " " + cookie + " " + a.Origin)
		return name + " " + timestamp + "_" + digest
	}
	parts := []string{proof(primary, scheme)}
	if secure {
		for _, item := range []struct{ name, scheme string }{{"__Secure-1PAPISID", "SAPISID1PHASH"}, {"__Secure-3PAPISID", "SAPISID3PHASH"}} {
			if value := a.Cookie(item.name); value != "" {
				parts = append(parts, proof(value, item.scheme))
			}
		}
	}
	return strings.Join(parts, " ")
}
