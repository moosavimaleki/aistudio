package browserinterface

import (
	"context"
	"fmt"
	"log"
	"time"
)

// One browser owns the UI bridge through cookie/transport extraction.
// Go requests may continue after the browser is parked. Conversation IDs
// remain pinned to their account, even when another account is selected.
func (f *Fleet) acquireChatGPT(ctx context.Context) (func(), error) {
	if f.config.ChatGPTRotateEvery == 0 {
		return func() {}, nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if f.chatGPTGate.TryLock() {
			return f.chatGPTGate.Unlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (f *Fleet) rotatingChatGPT() (string, error) {
	eligible := func(id string) bool {
		session := f.sessions[id]
		reason := f.warmErrors[id]
		return session != nil && session.spec.Provider == ProviderChatGPT && !isChatGPTSessionError(reason) && !isChatGPTChallengeError(reason)
	}
	if f.chatGPTUses < f.config.ChatGPTRotateEvery && eligible(f.chatGPTActive) {
		f.chatGPTUses++
		return f.chatGPTActive, nil
	}
	count := len(f.config.Browsers)
	for offset := 0; offset < count; offset++ {
		index := (f.chatGPTNext + offset) % count
		id := f.config.Browsers[index].ID
		if !eligible(id) {
			continue
		}
		f.chatGPTNext = (index + 1) % count
		f.chatGPTActive, f.chatGPTUses = id, 1
		return id, nil
	}
	return "", fmt.Errorf("No healthy ChatGPT browser profile is available")
}

func (f *Fleet) parkOtherChatGPT(id string) {
	if f.config.ChatGPTRotateEvery == 0 {
		return
	}
	f.chatGPTCurrent = id
	available := f.memoryAvailable
	if available == nil {
		available = browserMemoryAvailable
	}
	needed := browserMemoryReserve
	if session := f.sessions[id]; session != nil && !session.started.Load() {
		needed += browserLaunchMemory
	}
	for other, session := range f.sessions {
		if available() >= needed {
			return
		}
		if other != id && session.spec.Provider == ProviderChatGPT && session.started.Load() {
			log.Printf("parking idle ChatGPT browser %s under memory pressure", other)
			session.MarkProviderUnready()
			session.Close()
		}
	}
}
