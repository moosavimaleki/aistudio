package aistudio

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	multipartMaxBytes   = 5 * 1024 * 1024
	resumableChunkBytes = 8 * 1024 * 1024
)

func useResumableUpload(size int) bool {
	return size > multipartMaxBytes
}

func (t *Tab) uploadResumable(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	metadata []byte,
	mimeType string,
	content []byte,
) (*http.Response, error) {
	initHeaders := copyHeaders(headers)
	initHeaders["Content-Type"] = "application/json; charset=UTF-8"
	initHeaders["X-Upload-Content-Type"] = mimeType
	initHeaders["X-Upload-Content-Length"] = strconv.Itoa(len(content))
	response, err := t.HTTP.Request(ctx, http.MethodPost, endpoint, initHeaders, metadata)
	if err != nil {
		return nil, err
	}
	t.Cookies.ApplyResponse(response)
	t.syncSession()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := ReadBody(response)
		response.Body.Close()
		return nil, &ClientError{Message: fmt.Sprintf("UPLOAD failed with HTTP %d", response.StatusCode), Phase: "UPLOAD", Status: response.StatusCode, ResponseBody: body}
	}
	location := response.Header.Get("Location")
	response.Body.Close()
	if location == "" {
		return nil, NewError("UPLOAD", "Drive resumable upload response has no location")
	}
	uploadURL, err := resolveUploadURL(endpoint, location)
	if err != nil {
		return nil, err
	}

	for start := 0; start < len(content); start += resumableChunkBytes {
		end := min(start+resumableChunkBytes, len(content))
		chunkHeaders := copyHeaders(headers)
		chunkHeaders["Content-Type"] = mimeType
		chunkHeaders["Content-Length"] = strconv.Itoa(end - start)
		chunkHeaders["Content-Range"] = fmt.Sprintf("bytes %d-%d/%d", start, end-1, len(content))
		response, err = t.HTTP.Request(ctx, http.MethodPut, uploadURL, chunkHeaders, content[start:end])
		if err != nil {
			return nil, err
		}
		t.Cookies.ApplyResponse(response)
		t.syncSession()
		if response.StatusCode == http.StatusPermanentRedirect {
			response.Body.Close()
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			body, _ := ReadBody(response)
			response.Body.Close()
			return nil, &ClientError{Message: fmt.Sprintf("UPLOAD failed with HTTP %d", response.StatusCode), Phase: "UPLOAD", Status: response.StatusCode, ResponseBody: body}
		}
		return response, nil
	}
	return nil, NewError("UPLOAD", "Drive resumable upload ended without a final response")
}

func copyHeaders(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+3)
	for name, value := range source {
		result[name] = value
	}
	return result
}

func resolveUploadURL(endpoint, location string) (string, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	reference, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(reference).String(), nil
}
