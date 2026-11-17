package gencontent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

type videoStub struct {
	input aistudio.VideoInput
	calls int
	err   error
}

func (s *videoStub) GenerateVideo(_ context.Context, input aistudio.VideoInput, _ string) (videoOutcome, error) {
	s.input = input
	s.calls++
	return videoOutcome{
		Result: aistudio.VideoResult{
			Data: testVideoBytes(), MIMEType: "video/mp4", InteractionID: "test-interaction",
		},
		BrowserID: "default",
	}, s.err
}

func testVideoBytes() []byte {
	return append([]byte{0, 0, 0, 32}, []byte("ftypisom\x00\x00\x02\x00isomiso2avc1mp41")...)
}

func TestVideoGenerationAndDownload(t *testing.T) {
	stub := &videoStub{}
	server := &Server{video: stub, videoFiles: &videoStore{directory: t.TempDir()}}
	handler := server.Handler()
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/v1/videos/generations", strings.NewReader(`{"prompt":"test"}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("generation status %d: %s", response.Code, response.Body.String())
	}
	var body struct{ Data []struct{ URL string } }
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Data) != 1 {
		t.Fatalf("missing video URL: %v", err)
	}
	if stub.input.Model != aistudio.VideoModel {
		t.Fatalf("unexpected model: %s", stub.input.Model)
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequest("GET", body.Data[0].URL, nil)
	handler.ServeHTTP(response, request)
	if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), testVideoBytes()) {
		t.Fatalf("download failed: status %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "video/mp4" {
		t.Fatal("download MIME type is not video/mp4")
	}
	request.Header.Set("Range", "bytes=0-7")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.Len() != 8 {
		t.Fatal("video download does not support byte ranges")
	}
}

func TestVideoRejectsInvalidInputAndPreservesUpstreamErrors(t *testing.T) {
	stub := &videoStub{}
	server := &Server{video: stub, videoFiles: &videoStore{directory: t.TempDir()}}
	for _, body := range []string{
		`{}`, `{"prompt":"test","model":"veo-3"}`, `{"prompt":"test","n":2}`,
		`{"prompt":"test","response_format":"invalid"}`, `{`,
	} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest("POST", "/v1/videos/generations", strings.NewReader(body)))
		if response.Code != 400 || stub.calls != 0 {
			t.Fatalf("invalid request status=%d calls=%d", response.Code, stub.calls)
		}
	}
	stub.err = aistudio.ResponseError("RPC", 429, "video quota exhausted")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("POST", "/v1/videos/generations", strings.NewReader(`{"prompt":"test"}`)))
	if response.Code != 429 || !strings.Contains(response.Body.String(), "video quota exhausted") {
		t.Fatal("upstream quota error was not preserved")
	}
}

func TestVideoBase64DoesNotRequireServerStorage(t *testing.T) {
	server := &Server{video: &videoStub{}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/v1/videos/generations", strings.NewReader(
		`{"prompt":"test","response_format":"b64_json"}`,
	))
	server.Handler().ServeHTTP(response, request)
	var body struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Data) != 1 {
		t.Fatalf("missing base64 video: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(body.Data[0].Base64)
	if response.Code != 200 || err != nil || !bytes.Equal(decoded, testVideoBytes()) {
		t.Fatal("base64 response did not contain the generated video")
	}
}

func TestVideoStoreExpirationAndPathIsolation(t *testing.T) {
	store := &videoStore{directory: t.TempDir()}
	id, err := store.save(testVideoBytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../secret", "video_../secret", "", strings.Repeat("a", 100)} {
		if file, err := store.open(id); err == nil {
			file.Close()
			t.Fatal("invalid video ID was accepted")
		}
	}
	old := time.Now().Add(-2 * videoRetention)
	if err := os.Chtimes(store.path(id), old, old); err != nil {
		t.Fatal(err)
	}
	if file, err := store.open(id); err == nil {
		file.Close()
		t.Fatal("expired video was served")
	}
	if _, err := store.save(testVideoBytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.path(id)); !os.IsNotExist(err) {
		t.Fatal("expired video was not removed when saving another video")
	}
}
