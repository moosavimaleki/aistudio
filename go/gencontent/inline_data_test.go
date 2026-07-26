package gencontent

import "testing"

func TestReadInlinePartAcceptsSDKFieldNames(t *testing.T) {
	part, found, err := readInlinePart(map[string]any{
		"inlineData": map[string]any{
			"data":         "c2FtcGxl",
			"mime_type":    "text/plain",
			"display_name": "sample.txt",
		},
	})
	if err != nil || !found {
		t.Fatalf("readInlinePart() = %#v, %v, %v", part, found, err)
	}
	if part.mimeType != "text/plain" || part.displayName != "sample.txt" {
		t.Fatalf("unexpected inline part: %#v", part)
	}
}
