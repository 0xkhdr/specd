package mcp_test

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// captureCLI runs cmd.Dispatch for one command under SPECD_JSON=1, capturing
// stdout, and returns the parsed JSON object — the reference result a real CLI
// invocation would print. This is the ground truth an MCP tool call must match.
func captureCLI(t *testing.T, command string, argv ...string) map[string]any {
	t.Helper()
	out := captureCLIOutcome(t, command, argv...)
	if out.Code != core.ExitOK {
		t.Fatalf("CLI %s exit = %d, want 0\nstdout: %s\nstderr: %s", command, out.Code, out.Stdout, out.Stderr)
	}
	if out.Structured == nil {
		t.Fatalf("CLI %s output not JSON: %q", command, out.Stdout)
	}
	return out.Structured
}

type cliOutcome struct {
	Code       int
	Stdout     string
	Stderr     string
	Structured map[string]any
}

func captureCLIOutcome(t *testing.T, command string, argv ...string) cliOutcome {
	t.Helper()
	prev, had := os.LookupEnv("SPECD_JSON")
	os.Setenv("SPECD_JSON", "1")
	defer func() {
		if had {
			os.Setenv("SPECD_JSON", prev)
		} else {
			os.Unsetenv("SPECD_JSON")
		}
	}()

	// The MCP path forces structured output by appending --json to the argv
	// (mirrored here) in addition to SPECD_JSON; commands gate JSON on the flag.
	argv = append(append([]string{}, argv...), "--json")
	stdout, stderr, code := captureStreams(func() int {
		rc, known := cmd.Dispatch(command, cli.ParseArgs(argv))
		if !known {
			t.Fatalf("CLI command %q not known to dispatch", command)
		}
		return rc
	})
	out := cliOutcome{Code: code, Stdout: stdout, Stderr: stderr}
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		var m map[string]any
		if json.Unmarshal([]byte(trimmed), &m) == nil {
			out.Structured = m
		}
	}
	return out
}

func captureStreams(fn func() int) (stdout, stderr string, code int) {
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	outCh, errCh := readPipe(rOut), readPipe(rErr)
	code = fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = origOut, origErr
	return <-outCh, <-errCh, code
}

func readPipe(r *os.File) <-chan string {
	ch := make(chan string, 1)
	go func() {
		raw, _ := io.ReadAll(r)
		_ = r.Close()
		ch <- string(raw)
	}()
	return ch
}

// mcpStructured drives a single tools/call through the MCP stdio loop and
// returns the tool result's structuredContent object.
func mcpStructured(t *testing.T, tool string, args ...string) map[string]any {
	t.Helper()
	r := mcpToolResult(t, tool, map[string]any{"args": args})
	sc, ok := r["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call %s missing structuredContent: %v", tool, r)
	}
	return sc
}

func mcpToolResult(t *testing.T, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	argsJSON, _ := json.Marshal(arguments)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool +
		`","arguments":` + string(argsJSON) + `}}`
	resps := driveIntegration(t, &th.Harness{}, req)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	r, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call %s returned no result: %v", tool, resps[0])
	}
	return r
}

// TestCLIMCPParity asserts that for a representative set of read-only tools the
// MCP tool call returns the same structured result the equivalent CLI command
// prints (R2.2). Both paths re-enter cmd.Dispatch under SPECD_JSON, so any
// divergence is a real bug, not a fixture artifact. The CLI reference is
// captured BEFORE the MCP drive because both swap process-global os.Stdout.
func TestCLIMCPParity(t *testing.T) {
	h := th.New(t)
	h.Spec("parity").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	cases := []struct {
		tool    string
		command string
		args    []string
	}{
		{"specd_status", "status", []string{"parity"}},
		{"specd_waves", "waves", []string{"parity"}},
		{"specd_check", "check", []string{"parity"}},
		{"specd_next", "next", []string{"parity"}},
	}

	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			want := captureCLI(t, c.command, c.args...)
			got := mcpStructured(t, c.tool, c.args...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("MCP %s != CLI %s (divergence is a bug)\n  mcp: %v\n  cli: %v",
					c.tool, c.command, got, want)
			}
		})
	}
}

func TestOrchestrationCLIMCPParity(t *testing.T) {
	const sessionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cases := []struct {
		name      string
		setup     func(*th.Harness)
		command   string
		cliArgs   []string
		tool      string
		arguments map[string]any
	}{
		{
			name:      "brain status structured payload",
			setup:     func(h *th.Harness) { seedOrchestrationParitySession(t, h, "brain-status", sessionID) },
			command:   "brain",
			cliArgs:   []string{"status", "--session", sessionID},
			tool:      "specd_brain",
			arguments: map[string]any{"args": []string{"status"}, "session": sessionID},
		},
		{
			name:      "brain pause mutation",
			setup:     func(h *th.Harness) { seedOrchestrationParitySession(t, h, "brain-pause", sessionID) },
			command:   "brain",
			cliArgs:   []string{"pause", "--session", sessionID},
			tool:      "specd_brain",
			arguments: map[string]any{"args": []string{"pause"}, "session": sessionID},
		},
		{
			name: "brain resume mutation",
			setup: func(h *th.Harness) {
				seedOrchestrationParitySession(t, h, "brain-resume", sessionID)
				if _, err := core.PauseOrchestration(h.Root, sessionID); err != nil {
					t.Fatalf("pause seeded session: %v", err)
				}
			},
			command:   "brain",
			cliArgs:   []string{"resume", "--session", sessionID},
			tool:      "specd_brain",
			arguments: map[string]any{"args": []string{"resume"}, "session": sessionID},
		},
		{
			name:      "brain cancellation mutation",
			setup:     func(h *th.Harness) { seedOrchestrationParitySession(t, h, "brain-cancel", sessionID) },
			command:   "brain",
			cliArgs:   []string{"cancel", "--session", sessionID},
			tool:      "specd_brain",
			arguments: map[string]any{"args": []string{"cancel"}, "session": sessionID},
		},
		{
			name: "approval mutation",
			setup: func(h *th.Harness) {
				h.Spec("approval").
					Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
					Status(core.StatusRequirements).
					Build()
			},
			command:   "approve",
			cliArgs:   []string{"approval"},
			tool:      "specd_approve",
			arguments: map[string]any{"args": []string{"approval"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCLIMCPToolParity(t, tc.setup, tc.command, tc.cliArgs, tc.tool, tc.arguments)
		})
	}
}

func TestOrchestrationCLIMCPParityErrors(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(*th.Harness)
		command   string
		cliArgs   []string
		tool      string
		arguments map[string]any
	}{
		{
			name:      "invalid session ID error",
			setup:     func(*th.Harness) {},
			command:   "brain",
			cliArgs:   []string{"status", "--session", "not-valid"},
			tool:      "specd_brain",
			arguments: map[string]any{"args": []string{"status"}, "session": "not-valid"},
		},
		{
			name: "evidence gate failure",
			setup: func(h *th.Harness) {
				h.Spec("evidence-failure").
					Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
					FullDesign().
					AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
					Status(core.StatusExecuting).
					Build()
			},
			command:   "task",
			cliArgs:   []string{"evidence-failure", "T1", "--status", "complete", "--evidence", "claimed"},
			tool:      "specd_task",
			arguments: map[string]any{"args": []string{"evidence-failure", "T1"}, "status": "complete", "evidence": "claimed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCLIMCPToolParity(t, tc.setup, tc.command, tc.cliArgs, tc.tool, tc.arguments)
		})
	}
}

func seedOrchestrationParitySession(t *testing.T, h *th.Harness, slug, sessionID string) {
	t.Helper()
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
	policy, err := core.NewOrchestrationPolicy(core.LoadConfig(h.Root).Orchestration)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	if _, err := core.StartOrchestrationSession(h.Root, slug, sessionID, "parity-test", policy); err != nil {
		t.Fatalf("start orchestration session: %v", err)
	}
}

func assertCLIMCPToolParity(t *testing.T, setup func(*th.Harness), command string, cliArgs []string, tool string, arguments map[string]any) {
	t.Helper()
	cliHarness := th.New(t)
	setup(cliHarness)
	want := captureCLIOutcome(t, command, cliArgs...)

	mcpHarness := th.New(t)
	setup(mcpHarness)
	got := mcpToolResult(t, tool, arguments)

	wantIsError := want.Code != core.ExitOK
	if got["isError"] != wantIsError {
		t.Fatalf("MCP isError = %v, want %v (CLI exit %d); result=%v", got["isError"], wantIsError, want.Code, got)
	}
	if want.Structured != nil {
		sc, ok := got["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("MCP %s missing structuredContent: %v", tool, got)
		}
		if !reflect.DeepEqual(sc, want.Structured) {
			t.Fatalf("MCP %s structuredContent != CLI %s\n  mcp: %v\n  cli: %v", tool, command, sc, want.Structured)
		}
		return
	}
	if !wantIsError {
		t.Fatalf("CLI %s succeeded without JSON stdout; stdout=%q stderr=%q", command, want.Stdout, want.Stderr)
	}
	content, ok := got["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("MCP error result missing content: %v", got)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	wantText := strings.TrimSpace(want.Stderr)
	if wantText == "" {
		wantText = strings.TrimSpace(want.Stdout)
	}
	if wantText != "" && !strings.Contains(text, wantText) {
		t.Fatalf("MCP error text does not include CLI diagnostic\n  mcp: %q\n  cli: %q", text, wantText)
	}
}

// driveIntegration is a local helper that does NOT call t.Parallel: capture()
// swaps process-global os.Stdout/os.Stderr so integration steps must be serial.
func driveIntegration(t *testing.T, h *th.Harness, requests ...string) []map[string]any {
	t.Helper()
	_ = h // harness keeps the cwd fixture alive for the duration
	input := strings.Join(requests, "\n") + "\n"
	var out strings.Builder
	if err := mcp.Serve(strings.NewReader(input), &out, cmd.Dispatch); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var resps []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("response not JSON: %q: %v", line, err)
		}
		resps = append(resps, m)
	}
	return resps
}

// TestMCPWireContract exercises the four-step sequence required by R6:
//  1. initialize  → negotiated protocol version and tools capability
//  2. tools/list  → specd_status present with readOnlyHint annotation
//  3. tools/call specd_status → result carries structuredContent
//  4. unknown tool call → -32602 without connection teardown (recovery call follows)
//
// The test is intentionally serial (no t.Parallel) because capture() swaps
// process-global os.Stdout/os.Stderr.
func TestMCPWireContract(t *testing.T) {
	h := th.New(t)
	// Seed a real spec so specd_status has structured data to return.
	h.Spec("widget").
		Req("Auth", "As a user I want auth", "THE SYSTEM SHALL authenticate.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	resps := driveIntegration(t, h,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"integration-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["widget"]}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"specd_unknown_does_not_exist","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"ping"}`,
	)

	// notifications/initialized yields no response, so 5 requests → 5 responses.
	if len(resps) != 5 {
		t.Fatalf("got %d responses, want 5 (notification skipped): %v", len(resps), resps)
	}

	// Step 1: initialize — protocol version and tools capability (R6.1).
	t.Run("initialize advertises protocol version and tools capability", func(t *testing.T) {
		init, ok := resps[0]["result"].(map[string]any)
		if !ok {
			t.Fatalf("initialize result not an object: %v", resps[0])
		}
		if got := init["protocolVersion"]; got != "2025-11-25" {
			t.Errorf("protocolVersion = %q, want %q", got, "2025-11-25")
		}
		caps, ok := init["capabilities"].(map[string]any)
		if !ok {
			t.Fatalf("capabilities not an object: %v", init)
		}
		tools, ok := caps["tools"].(map[string]any)
		if !ok {
			t.Fatalf("capabilities.tools not an object: %v", caps)
		}
		if tools["listChanged"] != false {
			t.Errorf("listChanged = %v, want false", tools["listChanged"])
		}
	})

	// Step 2: tools/list — specd_status present with readOnlyHint:true (R6.2).
	t.Run("tools/list contains specd_status with annotations", func(t *testing.T) {
		list, ok := resps[1]["result"].(map[string]any)
		if !ok {
			t.Fatalf("tools/list result not an object: %v", resps[1])
		}
		rawTools, ok := list["tools"].([]any)
		if !ok {
			t.Fatalf("tools not an array: %v", list)
		}
		var statusTool map[string]any
		for _, raw := range rawTools {
			tool, _ := raw.(map[string]any)
			if tool["name"] == "specd_status" {
				statusTool = tool
				break
			}
		}
		if statusTool == nil {
			t.Fatalf("specd_status not found in tools/list")
		}
		ann, ok := statusTool["annotations"].(map[string]any)
		if !ok {
			t.Fatalf("specd_status missing annotations: %v", statusTool)
		}
		if ann["readOnlyHint"] != true {
			t.Errorf("specd_status readOnlyHint = %v, want true", ann["readOnlyHint"])
		}
	})

	// Step 3: tools/call specd_status — result has structuredContent (R6.3).
	t.Run("tools/call specd_status returns structuredContent", func(t *testing.T) {
		r, ok := resps[2]["result"].(map[string]any)
		if !ok {
			t.Fatalf("tools/call result not an object: %v", resps[2])
		}
		if r["isError"] != false {
			content, _ := r["content"].([]any)
			var text string
			if len(content) > 0 {
				text, _ = content[0].(map[string]any)["text"].(string)
			}
			t.Fatalf("specd_status call isError = true: %s", text)
		}
		if _, ok := r["structuredContent"]; !ok {
			t.Errorf("specd_status result missing structuredContent: %v", r)
		}
	})

	// Step 4: unknown tool → -32602, server still alive to answer ping (R6.4).
	t.Run("unknown tool returns -32602 without tearing down connection", func(t *testing.T) {
		e, ok := resps[3]["error"].(map[string]any)
		if !ok {
			t.Fatalf("unknown tool should return error, got result: %v", resps[3])
		}
		if code := e["code"].(float64); code != -32602 {
			t.Errorf("error code = %v, want -32602", code)
		}
		// Recovery: ping after the bad call must succeed, proving the loop survived.
		if _, ok := resps[4]["result"]; !ok {
			t.Errorf("server did not answer ping after unknown-tool error: %v", resps[4])
		}
	})
}
