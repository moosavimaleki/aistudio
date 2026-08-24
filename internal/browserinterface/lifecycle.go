package browserinterface

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
)

func (s *ChromeSession) primeNativeGenerate(ctx context.Context) error {
	result := make(chan error, 1)
	s.primeMu.Lock()
	s.primeResult = result
	s.primeMu.Unlock()
	defer func() {
		s.primeMu.Lock()
		s.primeResult = nil
		s.primeMu.Unlock()
		_ = chromedp.Run(s.ctx, fetch.Disable())
	}()

	pattern := &fetch.RequestPattern{
		URLPattern:   "*MakerSuiteService/GenerateContent*",
		RequestStage: fetch.RequestStageRequest,
	}
	if err := chromedp.Run(s.ctx, fetch.Enable().WithPatterns([]*fetch.RequestPattern{pattern})); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(clickConsentScript, nil),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(clickRunScript, nil),
	); err != nil {
		return fmt.Errorf("start native AI Studio lifecycle: %w", err)
	}

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("native AI Studio lifecycle did not produce GenerateContent: %w", ctx.Err())
	}
}

func (s *ChromeSession) captureNativeGenerate(event *fetch.EventRequestPaused) {
	if !strings.Contains(event.Request.URL, "MakerSuiteService/GenerateContent") {
		return
	}
	s.captureHeaders(event.Request.Headers)

	go func() {
		executor := chromedp.FromContext(s.ctx)
		if executor == nil || executor.Target == nil {
			return
		}
		commandContext := cdp.WithExecutor(s.ctx, executor.Target)
		continueErr := fetch.ContinueRequest(event.RequestID).Do(commandContext)
		s.primeMu.Lock()
		result := s.primeResult
		s.primeMu.Unlock()
		if result != nil {
			select {
			case result <- continueErr:
			default:
			}
		}
	}()
}

const promptSelector = `[aria-label="Enter a prompt"],[placeholder="Enter a prompt"]`

const clickConsentScript = `(() => {
  const agree = [...document.querySelectorAll("button")]
    .find(button => button.textContent?.trim() === "Agree");
  agree?.click();
})()`

const clickRunScript = `(async () => {
  const waitFor = async (find, timeoutMs) => {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const value = find();
      if (value) return value;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    throw new Error("AI Studio prompt controls did not become ready");
  };
  const prompt = await waitFor(() => document.querySelector('` + promptSelector + `'), 30000);
  const setValue = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value").set;
  setValue.call(prompt, "آزمون آماده‌سازی داخلی");
  prompt.dispatchEvent(new InputEvent("input", {
    bubbles: true,
    inputType: "insertText",
    data: "آزمون آماده‌سازی داخلی",
  }));
  const run = await waitFor(() => [...document.querySelectorAll('button[type="submit"]')]
    .find(button => button.textContent?.includes("Run") &&
      !button.disabled && button.getAttribute("aria-disabled") !== "true"), 30000);
  run.click();
})()`
