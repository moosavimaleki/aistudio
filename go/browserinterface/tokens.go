package browserinterface

import (
	"fmt"
	"os"
)

type TokenService struct {
	broker *Broker
	fleet  *Fleet
}

func NewTokenService(broker *Broker, fleet *Fleet) *TokenService {
	return &TokenService{broker: broker, fleet: fleet}
}
func (s *TokenService) Create(body map[string]any) (map[string]any, error) {
	if enabled, ok := body["attestationEnabled"].(bool); ok && !enabled {
		return map[string]any{"token": ""}, nil
	}
	_, authUser, err := ValidateTokenRequest(body)
	if err != nil {
		return nil, err
	}
	browserID, err := s.fleet.Resolve(fmt.Sprint(body["browserId"]))
	if err != nil {
		return nil, err
	}
	spec, err := s.fleet.Spec(browserID)
	if err != nil {
		return nil, err
	}
	cookies, _ := body["cookies"].(string)
	if SessionFingerprint(cookies, authUser) != SessionFingerprint(spec.CookieHeader, spec.AuthUser) {
		return nil, fmt.Errorf("Cookies do not belong to selected browserId: %s", browserID)
	}
	session, err := s.fleet.Session(browserID)
	if err != nil {
		return nil, err
	}
	prepared, err := session.Prepare(cookies, authUser)
	if err != nil {
		return nil, err
	}
	request, err := s.broker.Request(map[string]any{"digest": body["digest"], "authUser": authUser}, browserID)
	if err != nil {
		return nil, err
	}
	token, _ := request["token"].(string)
	if token == "" {
		return nil, fmt.Errorf("Container extension returned an empty token")
	}
	if os.Getenv("TOKEN_FACTORY_SAME_BROWSER_PROBE") == "1" { /* Extension's token is bound to this persistent Chrome session; the staging factory owns provider selection. */
	}
	current, err := session.Snapshot()
	if err != nil {
		return nil, err
	}
	runtime, _ := prepared["runtimeConfig"].(map[string]any)
	if runtime == nil {
		runtime = map[string]any{}
	}
	if extensionRuntime, ok := request["runtimeConfig"].(map[string]any); ok {
		for name, value := range extensionRuntime {
			runtime[name] = value
		}
	}
	runtime["authUser"] = authUser
	return map[string]any{"token": token, "cookieRecords": current["cookieRecords"], "transportProfile": current["transportProfile"], "runtimeConfig": runtime, "browserId": browserID}, nil
}
