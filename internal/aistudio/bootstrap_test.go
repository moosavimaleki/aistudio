package aistudio

import "testing"

func TestExtractRuntimeConfigReportsUnsupportedRegion(t *testing.T) {
	t.Setenv("AISTUDIO_UPSTREAM_CONFIG", "../../config/upstream.yaml")
	_, err := ExtractRuntimeConfig(`<a href="/gemini-api/docs/available-regions">Available regions</a>`, "0")
	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("expected ClientError, got %T: %v", err, err)
	}
	if clientErr.Diagnostics["looksLikeUnsupportedRegion"] != true {
		t.Fatalf("expected unsupported-region diagnostic, got %#v", clientErr.Diagnostics)
	}
}
