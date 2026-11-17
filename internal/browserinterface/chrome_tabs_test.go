package browserinterface

import (
	"testing"

	"github.com/chromedp/cdproto/target"
)

func TestStartupTabCleanupPreservesControlledAndAccountTabs(t *testing.T) {
	for _, test := range []struct {
		id, kind, url string
		close         bool
	}{
		{"controlled", "page", "about:blank", false},
		{"startup", "page", "about:blank", true},
		{"account", "page", "https://chatgpt.com/", false},
		{"studio", "page", "https://aistudio.google.com/", false},
		{"extension", "service_worker", "about:blank", false},
	} {
		tab := &target.Info{TargetID: target.ID(test.id), Type: test.kind, URL: test.url}
		if got := unusedStartupTab(tab, "controlled"); got != test.close {
			t.Fatalf("tab %s: close=%t, want %t", test.id, got, test.close)
		}
	}
}
