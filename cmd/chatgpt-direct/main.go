// Command chatgpt-direct serves sequential chat through the direct HTTP client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hamed/aistudio-api/internal/chatgptdirect"
	"github.com/hamed/aistudio-api/internal/directapi"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", "127.0.0.1:8788", "loopback HTTP address")
	session := flag.String("session", os.Getenv("CHATGPT_SESSION_FILE"), "chatgpt-web session snapshot")
	cookies := flag.String("cookie-dir", os.Getenv("CHATGPT_COOKIE_DIR"), "Netscape cookie directory (alternative to session)")
	script := flag.String("black-box-script", os.Getenv("CHATGPT_BLACK_BOX_SCRIPT"), "Black Box runtime script")
	proxy := flag.String("proxy", os.Getenv("AISTUDIO_PROXY_URL"), "optional HTTP/SOCKS proxy")
	model := flag.String("model", "chatgpt/gpt-5.6-thinking-extended", "direct model name")
	upstream := flag.String("upstream", "https://chatgpt.com", "upstream origin")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || (host != "127.0.0.1" && host != "localhost") {
		return fmt.Errorf("listen address must use 127.0.0.1 or localhost")
	}
	if _, err := chatgptdirect.ResolveModel(*model); err != nil {
		return err
	}
	if *session == "" && *cookies == "" {
		root := os.Getenv("XDG_STATE_HOME")
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("find home: %w", err)
			}
			root = filepath.Join(home, ".local", "state")
		}
		*session = filepath.Join(root, "chatgpt-web", "session.json")
	}
	client, err := chatgptdirect.NewNativeClient(chatgptdirect.NativeConfig{
		SessionFile: *session, CookieDir: *cookies, ProxyURL: *proxy,
		BlackBoxScript: *script, BaseURL: *upstream,
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: *listen, Handler: directapi.NewHandler(client, *model),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 310 * time.Second, IdleTimeout: 60 * time.Second,
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdown); err != nil {
				log.Printf("shutdown: %v", err)
			}
		case <-done:
		}
	}()
	log.Printf("direct API listening on http://%s", *listen)
	err = server.Serve(listener)
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
