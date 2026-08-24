package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type chatGPTPageState struct {
	Ready     bool   `json:"ready"`
	Challenge bool   `json:"challenge"`
	Title     string `json:"title"`
}

const chatGPTPageProbe = `({
  ready: Boolean(document.querySelector("#prompt-textarea")),
  challenge: document.title.includes("Just a moment") ||
    Boolean(document.querySelector('iframe[src*="challenges.cloudflare.com"]')),
  title: document.title,
})`

func (s *ChromeSession) waitForChatGPTComposer(ctx context.Context) error {
	var last chatGPTPageState
	for {
		if err := chromedp.Run(ctx, chromedp.Evaluate(chatGPTPageProbe, &last)); err == nil && last.Ready {
			return nil
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
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	var state chatGPTPageState
	if err := chromedp.Run(ctx, chromedp.Evaluate(chatGPTPageProbe, &state)); err != nil || !state.Ready {
		return false
	}
	s.markProviderReady()
	return true
}

func isChatGPTChallengeError(message string) bool {
	return strings.Contains(strings.ToLower(message), "browser challenge")
}
