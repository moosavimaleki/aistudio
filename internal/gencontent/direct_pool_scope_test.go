package gencontent

import "testing"

func TestDirectPoolDoesNotReuseBrowserOrOtherSessionRoots(t *testing.T) {
	pool := &chatConversationPool{}
	t.Setenv("CHATGPT_DIRECT_NATIVE", "")
	t.Setenv("CHATGPT_MODE", "browser")
	browser := pool.poolKey("model", "profile")
	t.Setenv("CHATGPT_MODE", "direct")
	t.Setenv("CHATGPT_SESSION_FILE", "/session-a.json")
	first := pool.poolKey("model", "profile")
	t.Setenv("CHATGPT_SESSION_FILE", "/session-b.json")
	second := pool.poolKey("model", "profile")
	if browser == first || first == second || browser == second {
		t.Fatal("browser and different native credential sources must have separate roots")
	}
}
