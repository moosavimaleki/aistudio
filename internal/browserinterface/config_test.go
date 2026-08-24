package browserinterface

import "testing"

func TestParseBrowserIDSet(t *testing.T) {
	result, err := parseBrowserIDSet("chatgpt, chatgpt5,chatgpt")
	if err != nil || len(result) != 2 || !result["chatgpt"] || !result["chatgpt5"] {
		t.Fatalf("unexpected result: %#v err=%v", result, err)
	}
	if _, err := parseBrowserIDSet("bad id"); err == nil {
		t.Fatal("expected invalid browser ID error")
	}
}

func TestEnabledBrowserFilterRemainsActiveAfterMatch(t *testing.T) {
	enabled, err := parseBrowserIDSet("chatgpt")
	if err != nil {
		t.Fatal(err)
	}
	filter := len(enabled) > 0
	delete(enabled, "chatgpt")
	if browserIDEnabled(filter, enabled, "chatgpt2") {
		t.Fatalf("filter must remain active after the selected ID is consumed: %#v", enabled)
	}
}
