package browserinterface

import (
	"context"
	"testing"
	"time"
)

func TestChatGPTPageStateRejectsLoginDialog(t *testing.T) {
	state := chatGPTPageState{Ready: true, LoginRequired: true}
	if state.usable() {
		t.Fatal("composer behind a login dialog must not be ready")
	}
}

func TestChatGPTPageStateAcceptsUsableComposer(t *testing.T) {
	state := chatGPTPageState{Ready: true}
	if !state.usable() {
		t.Fatal("visible composer without blockers should be ready")
	}
}

func TestChatGPTPageStateOnlyInvalidatesExplicitBlockers(t *testing.T) {
	if (chatGPTPageState{}).blocked() {
		t.Fatal("a temporarily missing composer is not an auth failure")
	}
	if !(chatGPTPageState{Challenge: true}).blocked() {
		t.Fatal("challenge must invalidate readiness")
	}
	if !(chatGPTPageState{LoginRequired: true}).blocked() {
		t.Fatal("login dialog must invalidate readiness")
	}
}

func TestChatGPTReadyProbeWaitsForSessionRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	session := &ChromeSession{ctx: ctx}
	session.mu.Lock()
	done := make(chan bool, 1)
	go func() { done <- session.refreshChatGPTReady() }()
	select {
	case <-done:
		session.mu.Unlock()
		t.Fatal("readiness probe raced with session restart")
	case <-time.After(20 * time.Millisecond):
	}
	session.mu.Unlock()
	select {
	case ready := <-done:
		if ready {
			t.Fatal("canceled session must not be ready")
		}
	case <-time.After(time.Second):
		t.Fatal("readiness probe remained blocked after restart")
	}
}
