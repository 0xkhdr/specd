package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/runner"
)

// ScrubbedEnv builds a minimal allowlisted environment for child processes
// (verify commands and custom gates), dropping inherited secrets from the parent
// shell while keeping the SPECD_* namespace the harness relies on. It is the
// single source of the env-scrub policy shared by verify and the custom-gate
// runner.
func ScrubbedEnv() []string {
	allow := []string{"PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"}
	var out []string
	for _, k := range allow {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SPECD_") {
			out = append(out, kv)
		}
	}
	return out
}

// CustomGateTaskRef is the read-only view of a task handed to a custom gate.
type CustomGateTaskRef struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Role   string `json:"role"`
	Wave   int    `json:"wave"`
}

// CustomGateInput is the JSON document written to a custom gate's stdin. It is a
// read-only snapshot of the spec; custom gates inspect it and emit findings.
// They never receive write access — the contract is data-in / findings-out.
type CustomGateInput struct {
	Spec   string              `json:"spec"`
	Root   string              `json:"root"`
	Status string              `json:"status"`
	Tasks  []CustomGateTaskRef `json:"tasks"`
}

// CustomGateFinding is one problem reported by a custom gate.
type CustomGateFinding struct {
	Location string `json:"location"`
	Message  string `json:"message"`
}

// CustomGateOutput is the JSON document a custom gate writes to stdout. Findings
// in Violations fail the gate; findings in Warnings are advisory. The runner
// applies the configured warn/error severity on top of this split.
type CustomGateOutput struct {
	Violations []CustomGateFinding `json:"violations"`
	Warnings   []CustomGateFinding `json:"warnings"`
}

// BuildCustomGateInput projects a spec's state into the read-only gate input.
func BuildCustomGateInput(root string, state *State) CustomGateInput {
	in := CustomGateInput{Spec: state.Spec, Root: root, Status: string(state.Status)}
	for _, t := range state.Tasks {
		in.Tasks = append(in.Tasks, CustomGateTaskRef{ID: t.ID, Status: string(t.Status), Role: t.Role, Wave: t.Wave})
	}
	return in
}

// RunCustomGate executes an external custom-gate command, passing input as JSON
// on stdin and parsing the gate's stdout as a CustomGateOutput. It mirrors the
// verify execution guarantees: a scrubbed environment, a bounded timeout, and a
// NUL-byte rejection. It loads no Go plugins and makes no network calls — the
// gate is an ordinary subprocess. Invalid JSON on stdout, a non-zero exit, or a
// timeout are all errors (the caller decides how a gate error maps to severity).
//
// sandbox selects the isolation backend (A5 R2): the empty string or "none" runs
// the command directly on the host (historical, byte-identical behaviour); any
// other value routes the command through the shared verify sandbox runner for
// parity. The env scrub holds in BOTH modes — sandboxed runs receive the same
// ScrubbedEnv() — and an unavailable backend fails the gate closed, consistent
// with verify.
func RunCustomGate(parent context.Context, root, shell, command string, input CustomGateInput, timeout time.Duration, sandbox string) (CustomGateOutput, error) {
	if strings.ContainsRune(command, 0) {
		return CustomGateOutput{}, GateError("custom gate command contains a NUL byte — refusing to run")
	}
	if shell == "" {
		shell = "sh"
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return CustomGateOutput{}, err
	}

	switch strings.TrimSpace(sandbox) {
	case "", "none":
		return runCustomGateHost(parent, root, shell, command, payload, timeout)
	default:
		return runCustomGateSandboxed(parent, root, shell, command, payload, timeout, sandbox)
	}
}

// runCustomGateHost is the default path: the gate command runs directly on the
// host with a scrubbed env and bounded timeout (unchanged pre-A5 behaviour).
func runCustomGateHost(parent context.Context, root, shell, command string, payload []byte, timeout time.Duration) (CustomGateOutput, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-c", command)
	cmd.Dir = root
	cmd.Env = ScrubbedEnv()
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate timed out after %s", timeout))
	}
	if runErr != nil {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate exited non-zero: %v (stderr: %s)", runErr, strings.TrimSpace(stderr.String())))
	}
	return parseCustomGateOutput(stdout.Bytes())
}

// runCustomGateSandboxed routes the gate command through the shared verify
// sandbox runner so a cautious operator gets parity with verify's fail-closed
// isolation. The same ScrubbedEnv() is forwarded, so the env-scrub guarantee
// holds identically to the host path. An unavailable backend (e.g. bwrap absent)
// is a hard error, not a silent host fallback.
func runCustomGateSandboxed(parent context.Context, root, shell, command string, payload []byte, timeout time.Duration, sandbox string) (CustomGateOutput, error) {
	r, err := runner.SelectRunner(sandbox)
	if err != nil {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate sandbox unavailable: %v", err))
	}
	res := r.Run(parent, runner.RunSpec{
		Root:    root,
		Shell:   shell,
		Command: command,
		Env:     ScrubbedEnv(),
		Timeout: timeout,
		Stdin:   payload,
	})
	if res.TimedOut {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate timed out after %s", timeout))
	}
	if res.ExitCode != 0 {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate exited non-zero: %d (stderr: %s)", res.ExitCode, strings.TrimSpace(res.Stderr)))
	}
	return parseCustomGateOutput([]byte(res.Stdout))
}

// parseCustomGateOutput decodes a gate's stdout into CustomGateOutput, rejecting
// unknown fields so a malformed contract fails loud rather than silently passing.
func parseCustomGateOutput(stdout []byte) (CustomGateOutput, error) {
	var out CustomGateOutput
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate emitted invalid JSON: %v", err))
	}
	return out, nil
}
