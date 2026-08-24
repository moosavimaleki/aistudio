package chatgptdirect

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/klauspost/compress/zstd"
)

func responseShape(response *fhttp.Response, data []byte) string {
	contentType := response.Header.Get("Content-Type")
	encoding := response.Header.Get("Content-Encoding")
	return fmt.Sprintf(
		"content-type=%q content-encoding=%q bytes=%d sse=%t",
		contentType,
		encoding,
		len(data),
		bytes.Contains(data, []byte("data:")),
	)
}

func readResponseBody(response *fhttp.Response) ([]byte, error) {
	if response.Uncompressed {
		return io.ReadAll(response.Body)
	}
	reader := io.Reader(response.Body)
	encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding")))
	switch encoding {
	case "", "identity":
	case "br":
		reader = brotli.NewReader(reader)
	case "gzip":
		compressed, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer compressed.Close()
		reader = compressed
	case "deflate":
		compressed, err := zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer compressed.Close()
		reader = compressed
	case "zstd":
		compressed, err := zstd.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer compressed.Close()
		reader = compressed
	default:
		return nil, fmt.Errorf("unsupported ChatGPT content encoding: %s", encoding)
	}
	return io.ReadAll(reader)
}
