package gencontent

import (
	"testing"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

func TestProfileFailuresMarkInvalidSession(t *testing.T) {
	failures := newProfileFailures()
	profile := BrowserProfile{ID: "browser2"}
	failures.Mark(profile, aistudio.ResponseError("RPC", 403, "denied"))

	if !failures.Has(profile.ID) {
		t.Fatal("expected invalid profile to be quarantined")
	}
	failure, found := failures.Status(profile.ID)
	if !found || failure.Status != 403 || failure.Phase != "RPC" {
		t.Fatalf("failure = %#v, found = %v", failure, found)
	}
}

func TestProfileFailureExpires(t *testing.T) {
	failures := newProfileFailures()
	failures.items["default"] = profileFailure{
		At:    time.Now().Add(-profileFailureCooldown - time.Second),
		Phase: "RPC", Status: 403,
	}

	if failures.Has("default") {
		t.Fatal("expected expired profile failure to leave quarantine")
	}
}

func TestProfileFailureCanBeClearedAfterRecovery(t *testing.T) {
	failures := newProfileFailures()
	profile := BrowserProfile{ID: "default"}
	failures.Mark(profile, aistudio.ResponseError("RPC", 403, "denied"))
	failures.Clear(profile.ID)

	if failures.Has(profile.ID) {
		t.Fatal("expected recovered profile to leave quarantine")
	}
}

func TestProfileErrorQuarantinesRateLimitedProfile(t *testing.T) {
	service := &Service{profileFailures: newProfileFailures()}
	profile := BrowserProfile{ID: "default", AuthUser: "0"}
	err := aistudio.ResponseError("RPC", 429, "quota exceeded")

	service.profileError(profile, err)
	if !service.profileFailures.Has(profile.ID) {
		t.Fatal("expected rate-limited profile to enter cooldown")
	}
}
