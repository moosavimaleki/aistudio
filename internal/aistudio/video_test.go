package aistudio

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestVideoPayloadMatchesCapture(t *testing.T) {
	payload, err := BuildVideoPayload(VideoInput{Model: VideoModel, Prompt: "a dancing elephant"})
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 6 || payload[4] != nil {
		t.Fatal("expected six fields and an empty token slot")
	}
	if positional(payload, 3, 17, 0) != VideoModel || positional(payload, 3, 53, 0, 0, 3, 5) != 3 {
		t.Fatal("model or video modality does not match capture")
	}
	digest, err := VideoContentDigest(payload)
	want := sha256.Sum256([]byte("a dancing elephant"))
	if err != nil || digest != hex.EncodeToString(want[:]) {
		t.Fatalf("unexpected digest: %v", err)
	}
	for _, input := range []VideoInput{{Model: "models/veo-3", Prompt: "test"}, {Model: VideoModel, Prompt: " "}} {
		if _, err := BuildVideoPayload(input); err == nil {
			t.Fatal("invalid input was accepted")
		}
	}
}

func syntheticMP4() []byte {
	return append([]byte{0, 0, 0, 32}, []byte("ftypisom\x00\x00\x02\x00isomiso2avc1mp41")...)
}

func videoStreamFixture(encoded string) string {
	frame := make([]any, 11)
	frame[10] = []any{1, []any{nil, nil, nil, nil, []any{1, encoded}}}
	data, _ := json.Marshal([]any{[]any{frame}})
	return string(data)
}

func TestCollectVideoResult(t *testing.T) {
	data := syntheticMP4()
	stream := videoStreamFixture(base64.StdEncoding.EncodeToString(data))
	result, err := CollectVideoResult(strings.NewReader(stream))
	if err != nil || !bytes.Equal(result.Data, data) || result.MIMEType != "video/mp4" {
		t.Fatalf("video extraction failed: %v", err)
	}
	for _, stream := range []string{
		"", "[[]]", `{"error":{"code":429}}`,
		videoStreamFixture("%%%"), videoStreamFixture("dGV4dA=="),
		stream[:len(stream)-2], stream + "broken",
	} {
		if _, err := CollectVideoResult(strings.NewReader(stream)); err == nil {
			t.Fatal("invalid or incomplete stream was accepted")
		}
	}
}

func TestCollectVideoLargerThanOldScannerLimit(t *testing.T) {
	data := append(syntheticMP4(), make([]byte, 13<<20)...)
	result, err := CollectVideoResult(strings.NewReader(videoStreamFixture(base64.StdEncoding.EncodeToString(data))))
	if err != nil || len(result.Data) != len(data) {
		t.Fatalf("large video extraction failed: %v", err)
	}
}

// Optional offline verification against a local capture. No cookie, token,
// prompt, or media contents are printed or copied into repository fixtures.
func TestVideoHAR(t *testing.T) {
	path := os.Getenv("AISTUDIO_VIDEO_HAR")
	if path == "" {
		t.Skip("AISTUDIO_VIDEO_HAR not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Log struct {
			Entries []struct {
				Request struct {
					URL      string                `json:"url"`
					PostData struct{ Text string } `json:"postData"`
				}
				Response struct{ Content struct{ Text string } }
			}
		}
	}
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	for _, entry := range capture.Log.Entries {
		if !strings.HasSuffix(entry.Request.URL, "/CreateInteractionStream") {
			continue
		}
		var payload []any
		if err := json.Unmarshal([]byte(entry.Request.PostData.Text), &payload); err != nil {
			t.Fatal(err)
		}
		prompt, _ := positional(payload, 3, 26, 0, 0, 0, 0, 0, 0, 0).(string)
		built, err := BuildVideoPayload(VideoInput{Model: VideoModel, Prompt: prompt})
		payload[4] = nil
		if err != nil || !equalJSON(payload, built) {
			t.Fatal("video payload differs from captured request")
		}
		result, err := CollectVideoResult(strings.NewReader(entry.Response.Content.Text))
		if err != nil || len(result.Data) == 0 || result.InteractionID == "" {
			t.Fatalf("captured video cannot be extracted: %v", err)
		}
		t.Logf("MP4 extracted: %d bytes; usage present: %t", len(result.Data), result.Usage != nil)
		return
	}
	t.Fatal("capture contains no CreateInteractionStream")
}
