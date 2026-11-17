package browserinterface

import "testing"

func TestLazyStartupKeepsOnlyProviderDefaultsRunning(t *testing.T) {
	fleet := &Fleet{config: Config{LazyStart: true, DefaultID: "default", ChatGPTDefaultID: "chatgpt"}}
	for _, id := range []string{"default", "chatgpt", "browser2", "chatgpt2"} {
		want := id == "default" || id == "chatgpt"
		if got := fleet.startsAtBoot(BrowserSpec{ID: id}); got != want {
			t.Fatalf("%s: start=%t, want %t", id, got, want)
		}
	}
	fleet.config.LazyStart = false
	if !fleet.startsAtBoot(BrowserSpec{ID: "chatgpt2"}) {
		t.Fatal("eager mode must still open all profiles")
	}
}

func TestLazyProfileRemainsAddressableAndVisible(t *testing.T) {
	specs := []BrowserSpec{{ID: "default"}, {ID: "chatgpt", Provider: ProviderChatGPT}, {ID: "chatgpt2", Provider: ProviderChatGPT}}
	fleet := NewFleet(NewBroker(), Config{LazyStart: true, DefaultID: "default", ChatGPTDefaultID: "chatgpt", Browsers: specs})
	id, err := fleet.ResolveChatGPT("chatgpt2")
	if err != nil || id != "chatgpt2" {
		t.Fatalf("unused profile must remain selectable: id=%s error=%v", id, err)
	}
	for _, status := range fleet.Status() {
		if status["sessionState"] != "IDLE" {
			t.Fatalf("unstarted profile reported as broken: %#v", status)
		}
	}
	fleet.sessions["chatgpt"].started.Store(true)
	id, err = fleet.ResolveChatGPT("")
	if err != nil || id != "chatgpt2" {
		t.Fatalf("automatic fallback must wake an unused account: id=%s error=%v", id, err)
	}
}
