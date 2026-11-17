package aistudio

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestVideoStreamPreservesRPCStatusInsideHTTP200(t *testing.T) {
	for _, test := range []struct {
		stream string
		status int
	}{
		{stream: `[[[]],[8,"Video generation quota exceeded",[["private-details"]]]]`, status: 429},
		{stream: `[[],[7,"Permission denied"]]`, status: 403},
		{stream: `[[],[16,"Unauthenticated"]]`, status: 401},
	} {
		result, err := CollectVideoResult(strings.NewReader(test.stream))
		var client *ClientError
		if !errors.As(err, &client) || client.Status != test.status || len(result.Data) != 0 {
			t.Fatalf("status %d: result=%#v error=%v", test.status, result, err)
		}
		if strings.Contains(client.ResponseBody, "private-details") {
			t.Fatal("RPC details must not be copied into diagnostic body")
		}
	}
}

func TestVideoStreamDoesNotReturnSuccessBeforeErrorTrailer(t *testing.T) {
	stream := videoStreamFixture(base64.StdEncoding.EncodeToString(syntheticMP4())) + "\n[[],[8,\"Quota exhausted\"]]"
	result, err := CollectVideoResult(strings.NewReader(stream))
	var client *ClientError
	if !errors.As(err, &client) || client.Status != 429 || len(result.Data) != 0 {
		t.Fatal("an error trailer must override partial stream contents")
	}
}
