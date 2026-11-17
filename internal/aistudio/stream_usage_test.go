package aistudio

import "testing"

func TestUsageFromPositionalChunk(t *testing.T) {
	frame := []any{nil, nil, []any{float64(4), float64(1), float64(5)}}
	usage := UsageFromChunk([]any{[]any{frame}})
	if usage == nil || usage["promptTokenCount"] != int64(4) || usage["candidatesTokenCount"] != int64(1) || usage["totalTokenCount"] != int64(5) {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestUsageFromChunkKeepsLastCumulativeFrame(t *testing.T) {
	frames := []any{
		[]any{nil, nil, []any{float64(9), float64(1), float64(10)}},
		[]any{nil, nil, []any{float64(40), float64(35), float64(75)}},
		[]any{nil, nil, nil},
	}
	usage := UsageFromChunk([]any{frames})
	if usage["promptTokenCount"] != int64(40) || usage["candidatesTokenCount"] != int64(35) {
		t.Fatalf("usage must use the last cumulative frame, got %#v", usage)
	}
	if usage["totalTokenCount"] != int64(75) {
		t.Fatalf("total = %#v", usage["totalTokenCount"])
	}
}
