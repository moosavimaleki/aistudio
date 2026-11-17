package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type chatGPTPageState struct {
	Ready         bool   `json:"ready"`
	Challenge     bool   `json:"challenge"`
	LoginRequired bool   `json:"loginRequired"`
	Title         string `json:"title"`
}

const chatGPTComposerSelector = `#prompt-textarea, [data-composer-markdown][contenteditable=true]`

const chatGPTPageProbe = `({
  ready: Boolean(document.querySelector('#prompt-textarea, [data-composer-markdown][contenteditable=true]')),
  challenge: document.title.includes("Just a moment") ||
    Boolean(document.querySelector('iframe[src*="challenges.cloudflare.com"]')),
  loginRequired: [...document.querySelectorAll('dialog, [role="dialog"]')].some(
    (dialog) => /welcome back|choose an account|session has expired|log in to another account/i.test(dialog.textContent || "")
  ),
  title: document.title,
})`

func (state chatGPTPageState) usable() bool {
	return state.Ready && !state.Challenge && !state.LoginRequired
}

func (state chatGPTPageState) blocked() bool {
	return state.Challenge || state.LoginRequired
}

func (s *ChromeSession) waitForChatGPTComposer(ctx context.Context) error {
	var last chatGPTPageState
	for {
		if err := chromedp.Run(ctx, chromedp.Evaluate(chatGPTPageProbe, &last)); err == nil {
			if last.LoginRequired {
				return fmt.Errorf("ChatGPT session requires sign-in in the browser")
			}
			if last.usable() {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if last.Challenge {
				return fmt.Errorf("ChatGPT browser challenge is waiting for native completion: %s", last.Title)
			}
			return fmt.Errorf("ChatGPT composer did not become ready: %w", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (s *ChromeSession) refreshChatGPTReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	var state chatGPTPageState
	if err := chromedp.Run(ctx, chromedp.Evaluate(chatGPTPageProbe, &state)); err != nil {
		return false
	}
	if state.blocked() {
		s.MarkProviderUnready()
		return false
	}
	if !state.usable() {
		return false
	}
	if s.spec.ChatGPTCookieFile != "" {
		cookies, err := s.readCookiesForURL("https://chatgpt.com/")
		if err != nil || len(cookies) == 0 {
			return false
		}
	}
	s.markProviderReady()
	return true
}

func isChatGPTChallengeError(message string) bool {
	return strings.Contains(strings.ToLower(message), "browser challenge")
}

func isChatGPTSessionError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "session cookies are expired or rejected") ||
		strings.Contains(message, "session requires sign-in")
}
