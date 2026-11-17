package browserinterface

import (
	"context"
	"errors"
	"testing"
)

func TestRotationBatchesAndSkipsBrokenAccounts(t *testing.T) {
	specs := []BrowserSpec{{ID: "a", Provider: ProviderChatGPT}, {ID: "bad", Provider: ProviderChatGPT}, {ID: "b", Provider: ProviderChatGPT}}
	f := NewFleet(NewBroker(), Config{Browsers: specs, ChatGPTRotateEvery: 10})
	f.setWarmError("bad", errors.New("ChatGPT session requires sign-in in the browser"))
	for i := 0; i < 21; i++ {
		want := "a"
		if i >= 10 && i < 20 {
			want = "b"
		}
		got, err := f.ResolveChatGPT("")
		if err != nil || got != want {
			t.Fatalf("request %d: %s %v; want %s", i, got, err, want)
		}
	}
	id, err := f.ResolveChatGPT("b")
	if err != nil || id != "b" {
		t.Fatal("pinned conversations must keep their account")
	}
}

func TestBrowserQueueHonorsCancellation(t *testing.T) {
	f := &Fleet{config: Config{ChatGPTRotateEvery: 10}}
	release, err := f.acquireChatGPT(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.acquireChatGPT(ctx); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}

func TestTransientFailureDoesNotPermanentlyExcludeAccount(t *testing.T) {
	f := NewFleet(NewBroker(), Config{ChatGPTRotateEvery: 10, Browsers: []BrowserSpec{{ID: "a", Provider: ProviderChatGPT}}})
	f.setWarmError("a", context.DeadlineExceeded)
	if got, err := f.ResolveChatGPT(""); err != nil || got != "a" {
		t.Fatalf("temporary timeout must be recoverable: %s %v", got, err)
	}
}
