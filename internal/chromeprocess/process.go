package chromeprocess

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

type Process struct {
	BrowserID   string
	Port, Index int
	command     *exec.Cmd
}

func NewProcess(browserID string, port, index int) *Process {
	return &Process{BrowserID: browserID, Port: port, Index: index}
}
func (p *Process) CDPURL() string { return fmt.Sprintf("http://127.0.0.1:%d", p.Port) }
func (p *Process) Running() bool  { return p.command != nil && p.command.Process != nil }
func (p *Process) Start() error {
	if p.Running() {
		return nil
	}
	runtime := defaultValue(os.Getenv("CHROME_RUNTIME_DIR"), "/app/browser-profiles")
	profile := filepath.Join(runtime, "profiles", p.BrowserID)
	source := defaultValue(os.Getenv("EXTENSION_SOURCE_DIR"), "/app/extension")
	extension, err := extensionDirectory(source, runtime, p.BrowserID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(profile, 0755); err != nil {
		return err
	}
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(profile, name))
	}
	if err := copyExtension(source, extension, p.BrowserID); err != nil {
		return err
	}
	args := []string{"--no-sandbox", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-search-engine-choice-screen", "--remote-debugging-address=127.0.0.1", fmt.Sprintf("--remote-debugging-port=%d", p.Port), "--user-data-dir=" + profile, "--disable-extensions-except=" + extension, "--load-extension=" + extension, "about:blank"}
	if proxy := os.Getenv("LAB_PROXY_URL"); proxy != "" {
		args = append(args, "--proxy-server="+proxy, "--proxy-bypass-list=127.0.0.1;localhost;<-loopback>")
	}
	p.command = exec.Command(defaultValue(os.Getenv("CHROME_EXECUTABLE"), "/usr/bin/google-chrome"), args...)
	if err := p.command.Start(); err != nil {
		return err
	}
	return p.waitReady()
}

func extensionDirectory(source, runtime, browserID string) (string, error) {
	data, err := os.ReadFile(filepath.Join(source, "manifest.json"))
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("read extension manifest: %w", err)
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("extension manifest has no version")
	}
	return filepath.Join(runtime, "extensions", browserID, manifest.Version), nil
}
func (p *Process) Stop() {
	if !p.Running() {
		return
	}
	_ = p.command.Process.Kill()
	_, _ = p.command.Process.Wait()
	p.command = nil
}
func (p *Process) waitReady() error {
	for attempt := 0; attempt < 100; attempt++ {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", p.Port), 200*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Chrome %s CDP endpoint did not become ready", p.BrowserID)
}
func copyExtension(source, target, browserID string) error {
	_ = os.RemoveAll(target)
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
	if err != nil {
		return err
	}
	upstream, err := aistudio.LoadUpstream()
	if err != nil {
		return err
	}
	config, _ := json.Marshal(map[string]string{"browserId": browserID, "factoryOrigin": defaultValue(os.Getenv("FACTORY_ORIGIN"), "http://127.0.0.1:3345"), "pageMatch": upstream.AIStudio["origin"] + "/*", "chatgptPageMatch": "https://chatgpt.com/*"})
	return os.WriteFile(filepath.Join(target, "config", "runtime-config.js"), []byte("globalThis.AISTUDIO_BRIDGE_CONFIG = "+string(config)+";\n"), 0644)
}
func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var _ = aistudio.RuntimeConfig{}
