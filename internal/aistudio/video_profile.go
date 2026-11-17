package aistudio

import (
	"context"
	"fmt"
	"strings"
)

func (t *Tab) videoTransportProfile(ctx context.Context) (map[string]string, error) {
	profile := mergeHeaders(t.TransportProfile, nil)
	for name, value := range profile {
		if strings.EqualFold(name, "X-AIStudio-G1-Tier") && value != "" {
			return profile, nil
		}
	}
	// Bootstrap RPCs do not always carry this header. Ask for the account's
	// actual subscription instead of assuming a tier from the browser ID.
	response, err := t.unary(ctx, "GetAiStudioBenefitTier", []any{}, t.LoggingContextExtension)
	if err != nil {
		return nil, err
	}
	tier, ok := integer(positional(response, 0))
	if !ok {
		return nil, fmt.Errorf("GetAiStudioBenefitTier returned no subscription tier")
	}
	name, found := map[int64]string{0: "BENEFIT_TIER_UNSPECIFIED", 1: "TIER1", 2: "TIER2", 3: "TIER0"}[tier]
	if !found {
		return nil, fmt.Errorf("GetAiStudioBenefitTier returned an unknown subscription tier")
	}
	profile["X-AIStudio-G1-Tier"] = name
	return profile, nil
}
