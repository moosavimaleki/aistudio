package gencontent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProfilesReadsBrowserListFromDegradedHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"browsers":[{"browserId":"default","authUser":"0","connected":true,"ready":false},{"browserId":"browser2","authUser":"0","connected":true,"ready":true}]}`))
	}))
	defer server.Close()

	service := &Service{healthURL: server.URL, http: server.Client()}
	profiles, err := service.profiles(context.Background())
	if err != nil {
		t.Fatalf("profiles: %v", err)
	}
	if len(profiles) != 2 || !profiles[1].Ready {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestChooseProfileRecoversWhenEveryBrowserIsUnready(t *testing.T) {
	recovered := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost {
			recovered = true
			_, _ = writer.Write([]byte(`{"ok":true}`))
			return
		}
		status := http.StatusServiceUnavailable
		ready := "false"
		if recovered {
			status = http.StatusOK
			ready = "true"
		}
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(`{"browsers":[{"browserId":"default","authUser":"0","connected":true,"ready":` + ready + `}]}`))
	}))
	defer server.Close()

	service := &Service{
		healthURL: server.URL, http: server.Client(),
		recoveryURL: server.URL + "/internal/browsers", recoveryHTTP: server.Client(),
		profileFailures: newProfileFailures(),
	}
	profile, err := service.chooseProfile(context.Background())
	if err != nil {
		t.Fatalf("choose profile: %v", err)
	}
	if !recovered || profile.ID != "default" {
		t.Fatalf("recovered = %v, profile = %#v", recovered, profile)
	}
}

func TestChooseProfileWaitsForRateLimitCooldown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"browsers":[{"browserId":"default","authUser":"0","connected":true,"ready":true}]}`))
	}))
	defer server.Close()

	failures := newProfileFailures()
	failures.items["default"] = profileFailure{
		At:     time.Now().Add(-profileFailureCooldown + 100*time.Millisecond),
		Phase:  "RPC",
		Status: http.StatusTooManyRequests,
	}
	service := &Service{
		healthURL:       server.URL,
		http:            server.Client(),
		profileFailures: failures,
	}

	started := time.Now()
	profile, err := service.chooseProfile(context.Background())
	if err != nil {
		t.Fatalf("choose profile: %v", err)
	}
	if profile.ID != "default" {
		t.Fatalf("profile = %#v", profile)
	}
	if elapsed := time.Since(started); elapsed < 50*time.Millisecond {
		t.Fatalf("choose profile returned before cooldown elapsed: %s", elapsed)
	}
}
