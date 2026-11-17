package gencontent

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

type videoOutcome struct {
	Result    aistudio.VideoResult
	BrowserID string
}

type videoGenerator interface {
	GenerateVideo(context.Context, aistudio.VideoInput, string) (videoOutcome, error)
}

func (s *Service) GenerateVideo(
	ctx context.Context,
	input aistudio.VideoInput,
	browserID string,
) (videoOutcome, error) {
	profile, err := s.videoProfile(ctx, browserID)
	if err != nil {
		return videoOutcome{}, err
	}
	lease, err := s.pool.Acquire(ctx)
	if err != nil {
		return videoOutcome{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.pool.Discard(cleanup, lease)
	}()
	settings, err := s.profileSettings(profile)
	if err != nil {
		return videoOutcome{}, err
	}
	tab, err := aistudio.NewTab(settings, nil, lease.TabID)
	if err != nil {
		return videoOutcome{}, err
	}
	if _, err = tab.Initialize(ctx); err != nil {
		return videoOutcome{}, s.profileError(profile, err)
	}
	result, err := tab.GenerateVideo(ctx, input)
	if err != nil {
		return videoOutcome{}, s.profileError(profile, err)
	}
	s.profileFailures.Clear(profile.ID)
	return videoOutcome{Result: result, BrowserID: profile.ID}, nil
}

func (s *Service) videoProfile(ctx context.Context, browserID string) (BrowserProfile, error) {
	if browserID == "" {
		return s.chooseProfile(ctx)
	}
	profiles, err := s.profiles(ctx)
	if err != nil {
		return BrowserProfile{}, err
	}
	for _, profile := range profiles {
		if profile.ID != browserID {
			continue
		}
		if profile.Connected && profile.Ready && !s.profileFailures.Has(profile.ID) {
			return profile, nil
		}
		return BrowserProfile{}, &aistudio.ClientError{
			Message: "AI Studio browser is not ready: " + browserID,
			Phase:   "HEALTH", Status: http.StatusServiceUnavailable,
		}
	}
	return BrowserProfile{}, &aistudio.ClientError{
		Message: fmt.Sprintf("Unknown AI Studio browser_id: %s", browserID),
		Phase:   "CONFIG", Status: http.StatusBadRequest,
	}
}
