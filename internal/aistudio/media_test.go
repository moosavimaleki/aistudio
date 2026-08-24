package aistudio

import "testing"

func TestLargeMediaUsesResumableUpload(t *testing.T) {
	if useResumableUpload(multipartMaxBytes) {
		t.Fatal("multipart limit itself must still use multipart upload")
	}
	if !useResumableUpload(multipartMaxBytes + 1) {
		t.Fatal("content above multipart limit must use resumable upload")
	}
}
