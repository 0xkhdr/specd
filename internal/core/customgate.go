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
func RunCustomGate(parent context.Context, root, shell, command string, input CustomGateInput, timeout time.Duration) (CustomGateOutput, error) {
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

	var out CustomGateOutput
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return CustomGateOutput{}, GateError(fmt.Sprintf("custom gate emitted invalid JSON: %v", err))
	}
	return out, nil
}
