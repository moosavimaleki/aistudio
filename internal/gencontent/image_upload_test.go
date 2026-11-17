package gencontent

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImageGenerationAcceptsReferenceImages(t *testing.T) {
	stub := &chatStub{}
	server := &Server{chat: stub}
	var payload bytes.Buffer
	form := multipart.NewWriter(&payload)
	_ = form.WriteField("model", "chatgpt-web")
	_ = form.WriteField("prompt", "Use both screenshots")
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.png", "second.png"} {
		part, err := form.CreateFormFile("image", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(png); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", &payload)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(stub.attachments) != 2 ||
		stub.attachments[0].Name != "first.png" || stub.attachments[1].MIMEType != "image/png" ||
		!strings.Contains(stub.imagePrompt, "Use both screenshots") {
		t.Fatalf("status=%d attachments=%#v prompt=%q body=%s", response.Code, stub.attachments, stub.imagePrompt, response.Body.String())
	}
}

func TestImageGenerationRejectsNonImageUpload(t *testing.T) {
	var payload bytes.Buffer
	form := multipart.NewWriter(&payload)
	_ = form.WriteField("prompt", "draw")
	part, _ := form.CreateFormFile("image", "not-image.txt")
	_, _ = part.Write([]byte("not an image"))
	_ = form.Close()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", &payload)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	(&Server{chat: &chatStub{}}).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
