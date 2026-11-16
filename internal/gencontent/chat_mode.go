package gencontent

import (
	"fmt"
	"os"
)

// chatMode selects native ChatGPT HTTP independently of the browser rendering
// setting. Empty preserves existing model-based routing for older deployments.
func chatMode() (string, error) {
	mode := os.Getenv("CHATGPT_MODE")
	switch mode {
	case "", "browser", "headless", "direct":
	default:
		return "", fmt.Errorf("CHATGPT_MODE must be browser, headless or direct")
	}
	if mode == "" && (os.Getenv("CHATGPT_DIRECT_NATIVE") == "true" || os.Getenv("CHATGPT_DIRECT_NATIVE") == "1") {
		return "direct", nil
	}
	return mode, nil
}
