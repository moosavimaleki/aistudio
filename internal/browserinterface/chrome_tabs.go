package browserinterface

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// Chrome opens an initial blank tab; chromedp creates a second controlled tab.
// Remove only unused startup blanks, never an account page or the controlled tab.
func closeStartupTabs(ctx context.Context) error {
	tabs, err := chromedp.Targets(ctx)
	if err != nil {
		return err
	}
	current := chromedp.FromContext(ctx)
	commandContext := cdp.WithExecutor(ctx, current.Browser)
	for _, tab := range tabs {
		if unusedStartupTab(tab, current.Target.TargetID) {
			if err := target.CloseTarget(tab.TargetID).Do(commandContext); err != nil {
				return err
			}
		}
	}
	return nil
}

func unusedStartupTab(tab *target.Info, controlled target.ID) bool {
	return tab.Type == "page" && tab.URL == "about:blank" && tab.TargetID != controlled
}
