package gencontent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

func (s *Service) Generate(ctx context.Context, input aistudio.GenerateInput) (map[string]any, error) {
	for attempt := 0; attempt < 2; attempt++ {
		result, err := s.generateOnce(ctx, input)
		if err == nil {
			return result, nil
		}
		if browserID, rateLimited := rateLimitedBrowser(err); rateLimited {
			if attempt == 1 {
				return nil, err
			}
			log.Printf("browser %s is rate-limited; failing over to another profile", browserID)
			continue
		}
		browserID, recoverable := recoveryBrowser(err)
		if !recoverable {
			return nil, err
		}
		log.Printf("recovering browser %s after invalid session: %v", browserID, err)
		if recoveryErr := s.resetProfile(ctx, browserID); recoveryErr != nil {
			attachRecoveryError(err, recoveryErr)
			return nil, err
		}
		if attempt == 1 {
			return nil, err
		}
	}
	return nil, fmt.Errorf("browser recovery exhausted")
}

func rateLimitedBrowser(err error) (string, bool) {
	var client *aistudio.ClientError
	if !errors.As(err, &client) || client.Status != http.StatusTooManyRequests {
		return "", false
	}
	browserID := strings.TrimSpace(fmt.Sprint(client.Diagnostics["browserId"]))
	return browserID, browserID != "" && browserID != "<nil>"
}

func recoveryBrowser(err error) (string, bool) {
	var client *aistudio.ClientError
	if !errors.As(err, &client) || !aistudio.InvalidatesTab(client) {
		return "", false
	}
	browserID := strings.TrimSpace(fmt.Sprint(client.Diagnostics["browserId"]))
	return browserID, browserID != "" && browserID != "<nil>"
}

func (s *Service) resetProfile(ctx context.Context, browserID string) error {
	endpoint := s.recoveryURL + "/" + url.PathEscape(browserID) + "/reset"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := s.recoveryHTTP.Do(request)
	if err != nil {
		return fmt.Errorf("browser recovery request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	return fmt.Errorf("browser recovery HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
}

func (s *Service) recoverProfiles(ctx context.Context, profiles []BrowserProfile) {
	for _, profile := range profiles {
		if s.profileFailures.Has(profile.ID) {
			continue
		}
		log.Printf("recovering unavailable browser %s", profile.ID)
		if err := s.resetProfile(ctx, profile.ID); err != nil {
			log.Printf("browser %s recovery failed: %v", profile.ID, err)
			s.profileFailures.Mark(profile, &aistudio.ClientError{
				Message: "Browser recovery failed", Phase: "RECOVERY", Status: http.StatusServiceUnavailable,
			})
			continue
		}
		s.profileFailures.Clear(profile.ID)
		return
	}
}

func (s *Service) waitForProfileCooldown(ctx context.Context, profiles []BrowserProfile) (bool, error) {
	wait := s.profileCooldownRetryAfter(profiles)
	if wait <= 0 {
		return false, nil
	}

	log.Printf("all usable browser profiles are cooling down; retrying in %s", wait.Round(time.Millisecond))
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true, nil
	case <-ctx.Done():
		return false, &aistudio.ClientError{
			Message: "No browser profile became available before request deadline",
			Phase:   "HEALTH",
			Status:  http.StatusServiceUnavailable,
			Diagnostics: map[string]any{
				"cause":    ctx.Err().Error(),
				"profiles": s.profileFailureDiagnostics(profiles)["profiles"],
			},
		}
	}
}

func (s *Service) profileCooldownRetryAfter(profiles []BrowserProfile) time.Duration {
	preferred := os.Getenv("AISTUDIO_DEFAULT_BROWSER_ID")
	var shortest time.Duration
	for _, profile := range profiles {
		if !profile.Connected || !profile.Ready || preferred != "" && profile.ID != preferred {
			continue
		}
		remaining := s.profileFailures.RetryAfter(profile.ID)
		if remaining > 0 && (shortest == 0 || remaining < shortest) {
			shortest = remaining
		}
	}
	return shortest
}

func attachRecoveryError(err, recoveryErr error) {
	var client *aistudio.ClientError
	if errors.As(err, &client) {
		if client.Diagnostics == nil {
			client.Diagnostics = map[string]any{}
		}
		client.Diagnostics["recoveryError"] = recoveryErr.Error()
	}
}
