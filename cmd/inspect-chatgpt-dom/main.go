package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	if len(os.Args) != 2 {
		panic("CDP URL is required")
	}
	response, err := http.Get(strings.TrimRight(os.Args[1], "/") + "/json/list")
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	var targets []struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		WebSocket string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(response.Body).Decode(&targets); err != nil {
		panic(err)
	}
	webSocket := ""
	targetID := ""
	for _, target := range targets {
		if strings.HasPrefix(target.URL, "https://chatgpt.com/") {
			webSocket = target.WebSocket
			targetID = target.ID
			break
		}
	}
	if webSocket == "" {
		panic("ChatGPT target is unavailable")
	}
	allocator, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), webSocket)
	defer cancelAllocator()
	ctx, cancel := chromedp.NewContext(allocator, chromedp.WithTargetID(targetID))
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, 10*time.Second)
	defer timeout()
	var images []map[string]any
	err = chromedp.Run(ctx, chromedp.Evaluate(`[
      ...document.images,
    ].map((image) => {
      const url = new URL(image.currentSrc || image.src, location.href);
      return {
        alt: image.alt,
        className: image.className,
        testId: image.dataset.testid || "",
        width: image.naturalWidth,
        height: image.naturalHeight,
        source: url.protocol + "//" + url.host + url.pathname,
        parentClass: image.parentElement?.className || "",
        messageRole: image.closest("[data-message-author-role]")?.dataset.messageAuthorRole || "",
      };
    })`, &images))
	if err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(images, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}
