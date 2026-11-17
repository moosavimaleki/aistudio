package chatgptdirect

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeOriginSecurity(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "runtime.cjs")
	if err := os.WriteFile(script, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		origin string
		valid  bool
	}{
		{"https://chatgpt.com", true},
		{"https://staging.example.test", true},
		{"http://127.0.0.1:1234", true},
		{"http://[::1]:1234", true},
		{"http://localhost:1234", true},
		{"http://staging.example.test", false},
		{"https://user:password@example.test", false},
		{"https://example.test/path", false},
		{"https://example.test?", false},
		{"https://example.test#fragment", false},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			_, err := NewNativeClient(NativeConfig{
				CookieDir: dir, BlackBoxScript: script, BaseURL: tc.origin,
			})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestBlackBoxEnvironmentExcludesCredentials(t *testing.T) {
	env := blackBoxEnvironment([]string{
		"PATH=/usr/bin", "HOME=/tmp", "CHATGPT_BLACK_BOX_CACHE_TTL_MS=1000",
		"CHATGPT_SESSION_FILE=/private/session.json", "API_KEY=fixture-secret",
		"AUTHORIZATION=Bearer fixture", "NODE_OPTIONS=--inspect", "LC_ALL=C",
	})
	joined := strings.Join(env, "\n")
	for _, secret := range []string{"SESSION", "API_KEY", "AUTHORIZATION", "NODE_OPTIONS"} {
		if strings.Contains(joined, secret) {
			t.Errorf("runtime received %s", secret)
		}
	}
	if len(env) != 4 {
		t.Fatalf("expected 4 runtime settings, got %d", len(env))
	}
}

func TestBlackBoxRunnerDataAndErrors(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for the subprocess contract test")
	}
	script := filepath.Join(t.TempDir(), "runtime.cjs")
	source := `
if (process.argv.includes("--fail")) {
  process.stderr.write("fixture-private-token"); process.exit(1);
}
if (process.argv.includes("--emit-initial")) {
  process.stdout.write(JSON.stringify({initial:"fixture-initial"}));
} else {
  let text = "";
  process.stdin.on("data", chunk => text += chunk);
  process.stdin.on("end", () => {
    const input = JSON.parse(text);
    if (input.initial !== "fixture-initial" || input.requirements.token !== "fixture") process.exit(2);
    process.stdout.write(JSON.stringify({work_token:"fixture-work", runtime_token:"fixture-runtime"}));
  });
}`
	if err := os.WriteFile(script, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	runner := newBlackBoxRunner(script)
	runner.nodePath = node
	initial, err := runner.initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	work, runtime, err := runner.evaluate(context.Background(), `{"token":"fixture"}`, initial)
	if err != nil || work != "fixture-work" || runtime != "fixture-runtime" {
		t.Fatalf("runtime contract failed: %v", err)
	}
	if _, err := runner.run(context.Background(), []string{"--fail"}, nil); err == nil || strings.Contains(err.Error(), "fixture-private-token") {
		t.Fatalf("runtime error must not include stderr: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runner.initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not preserved: %v", err)
	}
}
