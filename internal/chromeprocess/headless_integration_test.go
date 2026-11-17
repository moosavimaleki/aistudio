package chromeprocess

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Opt-in test uses only a synthetic extension and a local page, never cookies
// or the private extension images. No display server or network is needed.
func TestHeadlessExtensionAndInput(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE")
	if binary == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE to run a real headless browser")
	}
	root := t.TempDir()
	source := filepath.Join(root, "extension")
	if err := os.MkdirAll(filepath.Join(source, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.json": `{"manifest_version":3,"name":"Headless test","version":"1.0","content_scripts":[{"matches":["http://127.0.0.1/*"],"js":["test.js"]}]}`,
		"test.js":       `document.documentElement.dataset.extensionLoaded = "yes";`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(source, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CHROME_EXECUTABLE", binary)
	t.Setenv("CHROME_RUNTIME_DIR", root)
	t.Setenv("EXTENSION_SOURCE_DIR", source)
	t.Setenv("CHROME_HEADLESS", "true")
	t.Setenv("DISPLAY", "")
	t.Setenv("LAB_PROXY_URL", "")
	config, err := filepath.Abs("../../config/upstream.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AISTUDIO_UPSTREAM_CONFIG", config)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	p := NewProcess("synthetic-headless", port, 0)
	defer p.Stop()
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<input id="text"><button onclick="document.body.dataset.clicked='yes'">Send</button>`)
	}))
	defer server.Close()
	allocator, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), p.CDPURL())
	defer cancelAllocator()
	ctx, cancelTab := chromedp.NewContext(allocator)
	defer cancelTab()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var extension, value, clicked string
	err = chromedp.Run(ctx,
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`html[data-extension-loaded="yes"]`, chromedp.ByQuery),
		chromedp.SendKeys("#text", "headless input", chromedp.ByQuery),
		chromedp.Click("button", chromedp.ByQuery),
		chromedp.Evaluate(`document.documentElement.dataset.extensionLoaded`, &extension),
		chromedp.Value("#text", &value, chromedp.ByQuery),
		chromedp.Evaluate(`document.body.dataset.clicked`, &clicked),
	)
	if err != nil {
		t.Fatal(err)
	}
	if extension != "yes" || value != "headless input" || clicked != "yes" {
		t.Fatalf("extension/input/click failed: %q %q %q", extension, value, clicked)
	}
}
