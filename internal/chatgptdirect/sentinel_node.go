package chatgptdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// sentinelNodeRunner invokes the official ChatGPT Sentinel SDK through a Node.js
// sidecar. The SDK runs in a minimal browser environment (no Chromium); Node
// supplies crypto, TextEncoder, and a fake window/document. This replaces the
// in-Go FNV-1a proof-of-work solver and adds the turnstile VM evidence that the
// SDK evaluates internally. The sidecar script ships at
// /app/sentinel/sentinel-node-probe.js in the container.
type sentinelNodeRunner struct {
	scriptPath string
	nodePath   string
	timeout    time.Duration
}

func newSentinelNodeRunner(scriptPath string) *sentinelNodeRunner {
	return &sentinelNodeRunner{
		scriptPath: scriptPath,
		nodePath:   "node",
		timeout:    30 * time.Second,
	}
}

// initialProof runs the sidecar with --emit-initial and returns the SDK's
// initial proof string (the value sent as the prepare request body "p").
func (r *sentinelNodeRunner) initialProof(ctx context.Context) (string, error) {
	out, err := r.run(ctx, []string{"--emit-initial"}, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Initial string `json:"initial"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("decode sentinel initial: %w", err)
	}
	if result.Initial == "" {
		return "", fmt.Errorf("sentinel sidecar returned no initial proof")
	}
	return result.Initial, nil
}

// enforcementEvidence runs the sidecar with the requirements + the original
// preparation proof and returns the SDK's proof-of-work + turnstile evidence.
// The SDK binds the evidence to the original preparation proof; the caller must
// pass the same initial proof that was sent to /prepare.
func (r *sentinelNodeRunner) enforcementEvidence(ctx context.Context, requirements, initial string) (proof, turnstile string, err error) {
	input := map[string]any{
		"requirements": json.RawMessage(requirements),
		"initial":      initial,
	}
	out, err := r.run(ctx, []string{"--requirements-stdin", "--emit-enforcement-json"}, input)
	if err != nil {
		return "", "", err
	}
	var result struct {
		Proof     string `json:"proof"`
		Turnstile string `json:"turnstile"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", "", fmt.Errorf("decode sentinel enforcement: %w", err)
	}
	return result.Proof, result.Turnstile, nil
}

// run executes the Node sidecar with the given arguments, optionally feeding
// input as JSON on stdin, and returns the stdout bytes.
func (r *sentinelNodeRunner) run(ctx context.Context, args []string, input any) ([]byte, error) {
	cmdCtx := ctx
	if r.timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(cmdCtx, r.nodePath, append([]string{r.scriptPath}, args...)...)
	cmd.Env = os.Environ()
	// The Node 24 runtime used in Docker honors the same HTTP proxy as Go
	// when fetching the SDK. Prepared evidence still travels only over stdin.
	if proxy := os.Getenv("AISTUDIO_PROXY_URL"); proxy != "" {
		cmd.Env = append(cmd.Env, "NODE_USE_ENV_PROXY=1", "HTTPS_PROXY="+proxy, "HTTP_PROXY="+proxy)
	}
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("marshal sentinel sidecar input: %w", err)
		}
		cmd.Stdin = bytes.NewReader(payload)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := stderr.String()
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("sentinel sidecar failed: %w: %s", err, truncate(detail, 256))
	}
	return stdout.Bytes(), nil
}
