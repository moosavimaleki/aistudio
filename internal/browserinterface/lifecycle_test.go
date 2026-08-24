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

func TestNativeLifecycleInputsPromptInsidePage(t *testing.T) {
	if !strings.Contains(clickRunScript, `HTMLTextAreaElement.prototype`) ||
		!strings.Contains(clickRunScript, `new InputEvent("input"`) {
		t.Fatal("native lifecycle still depends on cross-window keyboard focus")
	}
}
