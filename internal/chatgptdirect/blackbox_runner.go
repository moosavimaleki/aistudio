package chatgptdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// blackBoxRunner executes SDK calculations in a separate Node.js process.
// Its JSON interface keeps the Go client independent of runtime internals.
type blackBoxRunner struct {
	scriptPath string
	nodePath   string
	timeout    time.Duration
}

func newBlackBoxRunner(scriptPath string) *blackBoxRunner {
	return &blackBoxRunner{
		scriptPath: scriptPath,
		nodePath:   "node",
		timeout:    30 * time.Second,
	}
}

// initialize returns the opaque context used for request preparation.
func (r *blackBoxRunner) initialize(ctx context.Context) (string, error) {
	out, err := r.run(ctx, []string{"--emit-initial"}, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Initial string `json:"initial"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("black box initialization returned invalid json")
	}
	if result.Initial == "" {
		return "", fmt.Errorf("black box initialization returned no context")
	}
	return result.Initial, nil
}

// evaluate binds the server requirements to the original initialization context.
func (r *blackBoxRunner) evaluate(ctx context.Context, requirements, initial string) (workToken, runtimeToken string, err error) {
	input := map[string]any{
		"requirements": json.RawMessage(requirements),
		"initial":      initial,
	}
	out, err := r.run(ctx, []string{"--requirements-stdin", "--emit-evidence-json"}, input)
	if err != nil {
		return "", "", err
	}
	var result struct {
		Work    string `json:"work_token"`
		Runtime string `json:"runtime_token"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", "", fmt.Errorf("black box evaluation returned invalid json")
	}
	return result.Work, result.Runtime, nil
}

// run transfers request data over stdin and reads the JSON result from stdout.
func (r *blackBoxRunner) run(ctx context.Context, args []string, input any) ([]byte, error) {
	cmdCtx := ctx
	if r.timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(cmdCtx, r.nodePath, append([]string{r.scriptPath}, args...)...)
	cmd.Env = blackBoxEnvironment(os.Environ())
	// The Node 24 runtime used in Docker honors the same HTTP proxy as Go
	// when fetching the SDK. Prepared evidence still travels only over stdin.
	if proxy := os.Getenv("AISTUDIO_PROXY_URL"); proxy != "" {
		cmd.Env = append(cmd.Env, "NODE_USE_ENV_PROXY=1", "HTTPS_PROXY="+proxy, "HTTP_PROXY="+proxy)
	}
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("marshal black box runner input: %w", err)
		}
		cmd.Stdin = bytes.NewReader(payload)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Runtime diagnostics may contain request data and must not enter API errors.
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if cmdCtx.Err() != nil {
			return nil, fmt.Errorf("black box runtime: %w", cmdCtx.Err())
		}
		return nil, fmt.Errorf("black box runner failed: %w", err)
	}
	return stdout.Bytes(), nil
}
