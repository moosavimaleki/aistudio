package aistudio

import "testing"

func TestGenerateDoesNotRetryRateLimitOnSameProfile(t *testing.T) {
	if retryableGenerateStatus(429) {
		t.Fatal("GenerateContent 429 must fail over to another browser profile")
	}
	if !retryableGenerateStatus(503) {
		t.Fatal("GenerateContent 503 should remain locally retryable")
	}
}
