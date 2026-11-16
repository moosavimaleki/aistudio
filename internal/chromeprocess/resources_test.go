package chromeprocess

import (
	"slices"
	"testing"
)

func TestResourceModePreservesPageCapabilities(t *testing.T) {
	flags := resourceFlags("")
	if !slices.Contains(flags, "--renderer-process-limit=2") {
		t.Fatal("resource mode must bound spare renderer allocation")
	}
	for _, forbidden := range []string{"--single-process", "--disable-javascript", "--disable-extensions", "--disable-web-security", "--disable-site-isolation-trials"} {
		if slices.Contains(flags, forbidden) {
			t.Fatalf("resource mode changed page capabilities: %s", forbidden)
		}
	}
	if len(resourceFlags("0")) != 0 || len(resourceFlags("false")) != 0 {
		t.Fatal("resource mode must be reversible for comparison")
	}
}

func TestHeadlessDisplayFlags(t *testing.T) {
	for _, value := range []string{"true", "TRUE", "1"} {
		flags := displayFlags(value)
		if !slices.Contains(flags, "--headless=new") || !slices.Contains(flags, "--window-size=1280,900") {
			t.Fatalf("unified headless viewport missing for %q: %v", value, flags)
		}
	}
	for _, value := range []string{"", "false", "0"} {
		if len(displayFlags(value)) != 0 {
			t.Fatalf("headed mode must remain available for VNC: %q", value)
		}
	}
}
