package gencontent

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

const maxReferenceImageBytes = 8 << 20

func readImageGenerationRequest(writer http.ResponseWriter, request *http.Request, body *imageGenerationRequest) ([]chatgptweb.Attachment, bool) {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, decodeJSON(writer, request, body)
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 32<<20)
	if err := request.ParseMultipartForm(8 << 20); err != nil {
		openAIError(writer, http.StatusBadRequest, "invalid multipart request: "+err.Error(), "invalid_request_error")
		return nil, false
	}
	defer request.MultipartForm.RemoveAll()
	body.Model = request.FormValue("model")
	body.Prompt = request.FormValue("prompt")
	body.BrowserID = request.FormValue("browser_id")
	body.ResponseFormat = request.FormValue("response_format")
	if raw := request.FormValue("n"); raw != "" {
		var err error
		body.N, err = strconv.Atoi(raw)
		if err != nil {
			openAIError(writer, http.StatusBadRequest, "n must be an integer", "invalid_request_error")
			return nil, false
		}
	}
	files := append([]*multipart.FileHeader{}, request.MultipartForm.File["image"]...)
	files = append(files, request.MultipartForm.File["image[]"]...)
	if len(files) > 4 {
		openAIError(writer, http.StatusBadRequest, "at most four reference images are supported", "invalid_request_error")
		return nil, false
	}
	attachments := make([]chatgptweb.Attachment, 0, len(files))
	for index, header := range files {
		attachment, err := readReferenceImage(header, index)
		if err != nil {
			openAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error")
			return nil, false
		}
		attachments = append(attachments, attachment)
	}
	return attachments, true
}

func readReferenceImage(header *multipart.FileHeader, index int) (chatgptweb.Attachment, error) {
	if header.Size > maxReferenceImageBytes {
		return chatgptweb.Attachment{}, fmt.Errorf("reference image %d exceeds 8 MiB", index+1)
	}
	file, err := header.Open()
	if err != nil {
		return chatgptweb.Attachment{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxReferenceImageBytes+1))
	if err != nil {
		return chatgptweb.Attachment{}, err
	}
	if len(data) > maxReferenceImageBytes {
		return chatgptweb.Attachment{}, fmt.Errorf("reference image %d exceeds 8 MiB", index+1)
	}
	mimeType := http.DetectContentType(data)
	if mimeType != "image/png" && mimeType != "image/jpeg" && mimeType != "image/webp" {
		return chatgptweb.Attachment{}, fmt.Errorf("reference image %d must be PNG, JPEG, or WebP", index+1)
	}
	name := filepath.Base(header.Filename)
	if name == "." || name == "" {
		name = fmt.Sprintf("reference-%d", index+1)
	}
	return chatgptweb.Attachment{Name: name, MIMEType: mimeType, Data: base64.StdEncoding.EncodeToString(data)}, nil
}
