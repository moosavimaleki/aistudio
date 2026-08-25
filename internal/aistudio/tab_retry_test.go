package aistudio

import (
	"context"
	"io"
	"net/url"
	"testing"
)

func TestGenerateDoesNotRetryRateLimitOnSameProfile(t *testing.T) {
	if retryableGenerateStatus(429) {
		t.Fatal("GenerateContent 429 must fail over to another browser profile")
	}
	if !retryableGenerateStatus(503) {
		t.Fatal("GenerateContent 503 should remain locally retryable")
	}
}

func TestRetryableGenerateTransportError(t *testing.T) {
	err := &url.Error{Op: "Post", URL: "https://staging.invalid", Err: io.ErrUnexpectedEOF}
	if !retryableGenerateTransportError(context.Background(), err) {
		t.Fatal("unexpected EOF from the HTTP transport should be retried")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if retryableGenerateTransportError(ctx, err) {
		t.Fatal("transport errors must not be retried after request cancellation")
	}
}
