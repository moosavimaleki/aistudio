package browserinterface

import (
	"strings"
	"testing"
)

func TestNativeLifecycleWaitsForARIAEnabledRunButton(t *testing.T) {
	if !strings.Contains(clickRunScript, `button.getAttribute("aria-disabled") !== "true"`) {
		t.Fatal("native lifecycle can click Run while AI Studio still marks it aria-disabled")
	}
}

func TestNativeLifecycleFindsRunButtonWithoutSubmitType(t *testing.T) {
	if strings.Contains(clickRunScript, `button[type="submit"]`) {
		t.Fatal("AI Studio Run button has no submit type")
	}
}

func TestNativeLifecycleInputsPromptInsidePage(t *testing.T) {
	if !strings.Contains(clickRunScript, `HTMLTextAreaElement.prototype`) ||
		!strings.Contains(clickRunScript, `new InputEvent("input"`) {
		t.Fatal("native lifecycle still depends on cross-window keyboard focus")
	}
}

func TestNativeLifecycleSelectsAvailableWarmupModel(t *testing.T) {
	if !strings.Contains(selectWarmupModelScript, `Gemini 3.5 Flash Lite`) ||
		strings.Contains(selectWarmupModelScript, `Upgrade to unlock`) {
		t.Fatal("warm-up must select a usable model even when the default card has no upgrade label")
	}
}
