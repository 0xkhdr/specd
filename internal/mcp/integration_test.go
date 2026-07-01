package mcp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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

// TestCLIMCPCallParity asserts that for a representative set of read-only tools
// the MCP tool call returns the same structured result the equivalent CLI
// command prints (R2.2). Both paths re-enter cmd.Dispatch under SPECD_JSON, so
// any divergence is a real bug, not a fixture artifact. The CLI reference is
// captured BEFORE the MCP drive because both swap process-global os.Stdout.
func TestCLIMCPCallParity(t *testing.T) {
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
	if err := mcp.Serve(strings.NewReader(input), &out, cmd.Dispatch, nil); err != nil {
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
	t.Run("initialize_advertises_protocol_version_and_tools_capability", func(t *testing.T) {
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
	t.Run("tools_list_contains_specd_status_with_annotations", func(t *testing.T) {
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
	t.Run("tools_call_specd_status_returns_structuredcontent", func(t *testing.T) {
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
	t.Run("unknown_tool_returns_32602_without_tearing_down_connection", func(t *testing.T) {
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

// TestMCPArgumentShapeValidation exercises Requirement 1 of the security-
// hardening spec (S1): an undeclared argument key or a wrong-shaped value is
// rejected with the existing -32602 envelope before any command dispatch,
// while a shape-valid request still succeeds end to end (Requirement 1.4 —
// the gate must not change behavior for already-valid requests).
func TestMCPArgumentShapeValidation(t *testing.T) {
	h := th.New(t)
	h.Spec("widget").
		Req("Auth", "As a user I want auth", "THE SYSTEM SHALL authenticate.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	resps := driveIntegration(t, h,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"integration-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		// specd_status declares "all" as a boolean flag; a string value is a shape violation.
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_status","arguments":{"all":"yes"}}}`,
		// "bogus" is not a declared argument for specd_status.
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"specd_status","arguments":{"bogus":"x"}}}`,
		// A shape-valid call must still succeed (Requirement 1.4).
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["widget"]}}}`,
	)

	if len(resps) != 4 {
		t.Fatalf("got %d responses, want 4 (initialize + 3 tools/call; notification skipped): %v", len(resps), resps)
	}
	// resps[0] is the initialize response (id 1); the three tools/call requests follow.

	t.Run("wrong_typed_flag_rejected_before_dispatch", func(t *testing.T) {
		e, ok := resps[1]["error"].(map[string]any)
		if !ok {
			t.Fatalf("type-mismatched argument should return error, got result: %v", resps[1])
		}
		if code := e["code"].(float64); code != -32602 {
			t.Errorf("error code = %v, want -32602", code)
		}
	})

	t.Run("unknown_argument_key_rejected_before_dispatch", func(t *testing.T) {
		e, ok := resps[2]["error"].(map[string]any)
		if !ok {
			t.Fatalf("unknown argument key should return error, got result: %v", resps[2])
		}
		if code := e["code"].(float64); code != -32602 {
			t.Errorf("error code = %v, want -32602", code)
		}
	})

	t.Run("shape_valid_request_still_succeeds", func(t *testing.T) {
		r, ok := resps[3]["result"].(map[string]any)
		if !ok {
			t.Fatalf("shape-valid call should succeed, got: %v", resps[3])
		}
		if r["isError"] != false {
			t.Errorf("shape-valid call isError = %v, want false: %v", r["isError"], r)
		}
	})
}

// TestResourcesListAndRead exercises the resources channel end-to-end over the
// stdio server (spec AC2–AC6): list enumerates a seeded spec's artifacts and
// steering files, read returns content with the right mime, and traversal/unknown
// URIs error without disclosure.
func TestResourcesListAndRead(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"specd://specs/auth/tasks.md"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"specd://specs/auth/state.json"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"specd://../../etc/passwd"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"specd://specs/auth/nope.md"}}`,
	)

	// list (AC2): at least the seeded spec's tasks.md + state.json show up.
	list := result(t, resps[0])["resources"].([]any)
	uris := map[string]string{}
	for _, r := range list {
		m := r.(map[string]any)
		uris[m["uri"].(string)] = m["mimeType"].(string)
	}
	if uris["specd://specs/auth/tasks.md"] != "text/markdown" {
		t.Errorf("list missing tasks.md or wrong mime: %v", uris)
	}
	if uris["specd://specs/auth/state.json"] != "application/json" {
		t.Errorf("list missing state.json or wrong mime: %v", uris)
	}

	// read markdown (AC3).
	md := result(t, resps[1])["contents"].([]any)[0].(map[string]any)
	if md["mimeType"] != "text/markdown" || strings.TrimSpace(md["text"].(string)) == "" {
		t.Errorf("tasks.md read = %v", md)
	}
	// read json (AC4).
	js := result(t, resps[2])["contents"].([]any)[0].(map[string]any)
	if js["mimeType"] != "application/json" {
		t.Errorf("state.json mime = %v, want application/json", js["mimeType"])
	}

	// traversal (AC5) and unknown (AC6) both error.
	for _, i := range []int{3, 4} {
		if _, ok := resps[i]["error"].(map[string]any); !ok {
			t.Errorf("response %d should be a resource error: %v", i, resps[i])
		}
	}
}

// TestPromptsListAndGet covers the prompts channel (spec AC2–AC6): list returns
// the 4 phase + 7 role prompts with declared arguments, get renders messages with
// slug substitution and is deterministic, and unknown names error.
func TestPromptsListAndGet(t *testing.T) {
	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"phase/design","arguments":{"slug":"auth"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"phase/design","arguments":{"slug":"auth"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/get","params":{"name":"role/craftsman"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"phase/bogus"}}`,
	)

	// list (AC2): the twelve expected prompt names with arguments declared.
	prompts := result(t, resps[0])["prompts"].([]any)
	if len(prompts) != 12 {
		t.Fatalf("prompts/list count = %d, want 12", len(prompts))
	}
	names := map[string]bool{}
	for _, p := range prompts {
		names[p.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"phase/requirements", "phase/design", "phase/tasks", "phase/execute", "role/scout", "role/researcher", "role/auditor", "role/architect", "role/tester", "role/documenter", "role/validator", "role/craftsman"} {
		if !names[want] {
			t.Errorf("prompts/list missing %s", want)
		}
	}

	// get phase/design slug=auth mentions the slug (AC3).
	msg := result(t, resps[1])["messages"].([]any)[0].(map[string]any)
	text := msg["content"].(map[string]any)["text"].(string)
	if !strings.Contains(text, "auth") {
		t.Errorf("phase/design did not substitute slug: %q", text)
	}

	// determinism (AC6): identical inputs → identical messages.
	text2 := result(t, resps[2])["messages"].([]any)[0].(map[string]any)["content"].(map[string]any)["text"].(string)
	if text != text2 {
		t.Errorf("phase/design not deterministic:\n%q\nvs\n%q", text, text2)
	}

	// role/craftsman returns a contract message (AC4).
	rb := result(t, resps[3])["messages"].([]any)
	if len(rb) == 0 {
		t.Error("role/craftsman returned no messages")
	}

	// unknown prompt errors (AC5).
	if _, ok := resps[4]["error"].(map[string]any); !ok {
		t.Errorf("unknown prompt should error: %v", resps[4])
	}
}

// TestCompositeRoundTrip covers spec AC1/AC3/AC4: a composite call produces output
// byte-identical to the atomic command it wraps.
func TestCompositeRoundTrip(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	pairs := []struct {
		name      string
		composite string
		atomic    string
	}{
		{
			"inspect=status",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_inspect","arguments":{"view":"status","slug":"auth"}}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["auth"]}}}`,
		},
		{
			"query=next",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_query","arguments":{"view":"next","slug":"auth"}}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_next","arguments":{"args":["auth"]}}}`,
		},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			resps := drive(t, p.composite, p.atomic)
			ct := result(t, resps[0])["content"].([]any)[0].(map[string]any)["text"].(string)
			at := result(t, resps[1])["content"].([]any)[0].(map[string]any)["text"].(string)
			if ct != at {
				t.Errorf("round-trip mismatch:\ncomposite=%q\natomic=%q", ct, at)
			}
		})
	}
}

// safeBuffer is a goroutine-safe sink: Serve's request handler and the phase
// watcher both write to it, while the test reads snapshots concurrently.
type safeBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func phaseConfig() *core.Config {
	cfg := core.LoadConfig(".")
	cfg.MCP.Expose = "phase"
	return &cfg
}

func setSpecStatus(t *testing.T, h *th.Harness, slug string, status core.SpecStatus) {
	t.Helper()
	path := h.SpecPath(slug, "state.json")
	raw := core.ReadOrNull(path)
	if raw == nil {
		t.Fatalf("state.json not found at %s", path)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(*raw), &m); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	enc, _ := json.Marshal(string(status))
	m["status"] = enc
	out, _ := json.MarshalIndent(m, "", "  ")
	if err := core.AtomicWrite(path, string(out)+"\n"); err != nil {
		t.Fatalf("write state.json: %v", err)
	}
}

// lastToolsList returns the tool names from the last tools/list response found in
// the captured stream, plus how many list_changed notifications it carried.
func parseStream(t *testing.T, raw string) (lastTools map[string]bool, notifications int) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("stream line not JSON: %q: %v", line, err)
		}
		if m["method"] == "notifications/tools/list_changed" {
			notifications++
			continue
		}
		res, ok := m["result"].(map[string]any)
		if !ok {
			continue
		}
		tools, ok := res["tools"].([]any)
		if !ok {
			continue
		}
		lastTools = map[string]bool{}
		for _, tv := range tools {
			td, _ := tv.(map[string]any)
			if name, _ := td["name"].(string); name != "" {
				lastTools[name] = true
			}
		}
	}
	return lastTools, notifications
}

// TestPhaseModeAdvertisesListChanged checks the capability is on under
// expose:"phase" (spec AC1) end-to-end through Serve.
func TestPhaseModeAdvertisesListChanged(t *testing.T) {
	h := th.New(t)
	seedDesignSpec(h, "auth")

	in, inW := io.Pipe()
	var out safeBuffer
	cfg := phaseConfig()
	done := make(chan struct{})
	go func() { _ = mcp.Serve(in, &out, cmd.Dispatch, cfg); close(done) }()

	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"protocolVersion"`) })
	_ = inW.Close()
	<-done

	var resp map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		_ = json.Unmarshal([]byte(line), &resp)
		if resp["id"] != nil {
			break
		}
	}
	caps := resp["result"].(map[string]any)["capabilities"].(map[string]any)
	if lc := caps["tools"].(map[string]any)["listChanged"].(bool); !lc {
		t.Error("expose:phase should advertise tools.listChanged:true")
	}
}

// TestPhaseModeNotifiesAndAdaptsList drives design→executing and asserts the host
// receives a list_changed notification and a subsequent tools/list reflects the
// executing subset (spec AC2/AC3).
func TestPhaseModeNotifiesAndAdaptsList(t *testing.T) {
	h := th.New(t)
	seedDesignSpec(h, "auth")

	in, inW := io.Pipe()
	var out safeBuffer
	cfg := phaseConfig()
	done := make(chan struct{})
	go func() { _ = mcp.Serve(in, &out, cmd.Dispatch, cfg); close(done) }()

	// Initial list under the design phase.
	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"specd_inspect"`) })

	// Transition the spec; the watcher should detect it, swap, and notify.
	setSpecStatus(t, h, "auth", core.StatusExecuting)
	waitFor(t, &out, func(s string) bool {
		return strings.Contains(s, "notifications/tools/list_changed")
	})

	// Re-fetch: the list must now be the executing subset.
	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"specd_next"`) })

	_ = inW.Close()
	<-done

	tools, notifications := parseStream(t, out.String())
	if notifications < 1 {
		t.Errorf("expected at least one list_changed notification, got %d", notifications)
	}
	if !tools["specd_next"] {
		t.Errorf("post-transition list missing executing tool specd_next: %v", tools)
	}
	if tools["specd_check"] {
		t.Errorf("post-transition list still exposes planning tool specd_check: %v", tools)
	}
}

func seedDesignSpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusDesign).
		Build()
}

func waitFor(t *testing.T, out *safeBuffer, cond func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond(out.String()) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within timeout; stream so far:\n%s", out.String())
}

// driveCfg is drive() with an explicit project config so tools/list exercises
// the configured (non-passthrough) path the Wave 4 filters live on.
func driveCfg(t *testing.T, cfg *core.Config, requests ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := mcp.Serve(in, &out, cmd.Dispatch, cfg); err != nil {
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

func listToolNames(t *testing.T, resp map[string]any) []string {
	t.Helper()
	r := result(t, resp)
	raw, ok := r["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result missing tools array: %v", r)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if n, ok := m["name"].(string); ok {
			out = append(out, n)
		}
	}
	return out
}

func writeManifest(t *testing.T, root, slug, body string) {
	t.Helper()
	path := core.SpecDir(root, slug) + "/manifest.json"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func has(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// TestManifestFilterIntegration: a configured server reading a spec's
// manifest.json restricts tools/list to its required/optional set and excludes
// forbidden tools (C1 AC1/AC2).
func TestManifestFilterIntegration(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	writeManifest(t, h.Root, "auth", `{"contextManifest":{
		"requiredTools":["specd_inspect","specd_verify"],
		"forbiddenTools":["specd_task"]}}`)

	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}
	resps := driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	got := listToolNames(t, resps[0])

	if !has(got, "specd_inspect") || !has(got, "specd_verify") {
		t.Fatalf("required tools missing: %v", got)
	}
	if has(got, "specd_task") {
		t.Fatalf("forbidden specd_task present: %v", got)
	}
	if has(got, "specd_status") {
		t.Fatalf("non-allowlisted specd_status present: %v", got)
	}
}

// TestNoManifestUnchangedIntegration: with no manifest, a configured server's
// tools/list matches the config/phase plan (C1 AC4).
func TestNoManifestUnchangedIntegration(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	withManifestWritten := func(body string) int {
		if body != "" {
			writeManifest(t, h.Root, "auth", body)
		}
		resps := driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		return len(listToolNames(t, resps[0]))
	}
	base := withManifestWritten("")
	// Adding an all-permitting (absent-field) manifest must not change the count.
	if got := withManifestWritten(`{"other":true}`); got != base {
		t.Fatalf("empty-policy manifest changed tool count: %d vs %d", got, base)
	}
}

// TestHostNegotiationMaxTools: capabilities.specd.maxTools caps tools/list, and
// the same session keeps applying it on re-fetch (C2 AC1/R5).
func TestHostNegotiationMaxTools(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	resps := driveCfg(t, cfg,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"specd":{"maxTools":5}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
	)
	for _, id := range []int{1, 2} { // resp index 1,2 are the two tools/list
		got := listToolNames(t, resps[id])
		if len(got) > 5 {
			t.Fatalf("maxTools=5 emitted %d tools: %v", len(got), got)
		}
	}
}

// TestHostNegotiationAbsentIdentical: a host that omits capabilities.specd sees
// the exact feature-off tool list (C2 AC4).
func TestHostNegotiationAbsentIdentical(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "all"}}

	plain := listToolNames(t, driveCfg(t, cfg, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)[0])
	negotiated := driveCfg(t, cfg,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	got := listToolNames(t, negotiated[1])
	if strings.Join(plain, ",") != strings.Join(got, ",") {
		t.Fatalf("absent capability changed output:\n plain=%v\n got  =%v", plain, got)
	}
}

type orchestrationMCPClient struct {
	t    *testing.T
	base string
	next int
}

func newOrchestrationMCPClient(t *testing.T, httpSSE bool) orchestrationMCPClient {
	t.Helper()
	client := orchestrationMCPClient{t: t}
	if httpSSE {
		addr := freePort(t)
		go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
		waitReady(t, addr)
		client.base = "http://" + addr
	}
	return client
}

func (c *orchestrationMCPClient) call(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	var result map[string]any
	if c.base == "" {
		result = mcpToolResult(c.t, tool, arguments)
	} else {
		result = httpResult(c.t, c.base, c.nextPath(), toolCallRequest(c.t, tool, arguments))
	}
	if result["isError"] == true {
		c.t.Fatalf("%s returned MCP error: %v", tool, result)
	}
	return result
}

func (c *orchestrationMCPClient) callError(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	var result map[string]any
	if c.base == "" {
		result = mcpToolResult(c.t, tool, arguments)
	} else {
		result = httpResult(c.t, c.base, c.nextPath(), toolCallRequest(c.t, tool, arguments))
	}
	if result["isError"] != true {
		c.t.Fatalf("%s succeeded, want tool-level error: %v", tool, result)
	}
	return result
}

func (c *orchestrationMCPClient) structured(tool string, arguments map[string]any) map[string]any {
	c.t.Helper()
	result := c.call(tool, arguments)
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		c.t.Fatalf("%s missing structuredContent: %v", tool, result)
	}
	return structured
}

func (c *orchestrationMCPClient) nextPath() string {
	c.next++
	if c.next%2 == 0 {
		return "/sse"
	}
	return "/rpc"
}

func toolCallRequest(t *testing.T, tool string, arguments map[string]any) string {
	t.Helper()
	rawArgs, err := json.Marshal(arguments)
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, rawArgs)
}

type mcpLifecycleSummary struct {
	SuccessSpecStatus  core.SpecStatus                 `json:"successSpecStatus"`
	SuccessTaskStatus  core.TaskStatus                 `json:"successTaskStatus"`
	SuccessSession     core.OrchestrationSessionStatus `json:"successSession"`
	SuccessEvents      []string                        `json:"successEvents"`
	CancelSpecStatus   core.SpecStatus                 `json:"cancelSpecStatus"`
	CancelTaskStatus   core.TaskStatus                 `json:"cancelTaskStatus"`
	CancelSession      core.OrchestrationSessionStatus `json:"cancelSession"`
	CancelEvents       []string                        `json:"cancelEvents"`
	CancelDirectiveCnt int                             `json:"cancelDirectiveCount"`
}

func TestMCPOrchestrationLifecycleStdioAndHTTPSSE(t *testing.T) {
	stdio := runMCPOrchestrationLifecycle(t, false)
	httpSSE := runMCPOrchestrationLifecycle(t, true)
	if !reflect.DeepEqual(httpSSE, stdio) {
		t.Fatalf("HTTP/SSE lifecycle summary != stdio\n http/sse: %#v\n stdio:    %#v", httpSSE, stdio)
	}
}

func runMCPOrchestrationLifecycle(t *testing.T, httpSSE bool) mcpLifecycleSummary {
	t.Helper()
	h := th.New(t)
	seedLifecycleSpec(h, "mcp-life", "test -f pass.flag")
	seedLifecycleSpec(h, "mcp-cancel", "true")
	client := newOrchestrationMCPClient(t, httpSSE)

	const lifeSession = "18181818181818181818181818181818"
	lifeStart := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("start", "mcp-life", lifeSession)))
	if lifeStart.Decision.Action != core.OrchestrationDispatch || lifeStart.Decision.Attempt != 1 || lifeStart.Event == nil {
		t.Fatalf("life start decision=%#v event=%#v, want dispatch attempt 1", lifeStart.Decision, lifeStart.Event)
	}
	client.structured("specd_brain", map[string]any{"args": []string{"status"}, "session": lifeSession})
	paused := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"pause"}, "session": lifeSession}))
	if paused.Status != core.OrchestrationSessionPaused {
		t.Fatalf("pause status=%s, want paused", paused.Status)
	}
	pausedStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if pausedStep.Decision.Action != core.OrchestrationWait || pausedStep.Event != nil {
		t.Fatalf("paused step=%#v event=%#v, want wait without event", pausedStep.Decision, pausedStep.Event)
	}
	resumed := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"resume"}, "session": lifeSession}))
	if resumed.Status != core.OrchestrationSessionRunning {
		t.Fatalf("resume status=%s, want running", resumed.Status)
	}

	lifeMission := missionFromStep(t, h.Root, lifeStart)
	claim := decodeMCPStructured[core.PinkyClaim](t, client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, lifeMission)}))
	if claim.Mission.WorkerID != lifeMission.WorkerID || claim.Mission.TaskID != lifeMission.TaskID {
		t.Fatalf("claim=%#v mission=%#v", claim, lifeMission)
	}
	client.structured("specd_pinky", leaseArgs("heartbeat", lifeMission))
	client.structured("specd_pinky", map[string]any{"args": []string{"progress"}, "session": lifeMission.SessionID, "worker": lifeMission.WorkerID, "spec": lifeMission.Spec, "task": lifeMission.TaskID, "attempt": lifeMission.Attempt, "percent": 50, "message": "halfway"})
	client.callError("specd_verify", map[string]any{"args": []string{"mcp-life", "T1"}})
	client.structured("specd_pinky", map[string]any{"args": []string{"block"}, "session": lifeMission.SessionID, "worker": lifeMission.WorkerID, "spec": lifeMission.Spec, "task": lifeMission.TaskID, "attempt": lifeMission.Attempt, "reason": "pass flag missing"})
	client.call("specd_pinky", leaseArgs("release", lifeMission))

	if err := os.WriteFile(h.Path("pass.flag"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write pass.flag: %v", err)
	}
	retryStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if retryStep.Decision.Action != core.OrchestrationDispatch || retryStep.Decision.Attempt != 2 || retryStep.Event == nil {
		t.Fatalf("retry decision=%#v event=%#v, want dispatch attempt 2", retryStep.Decision, retryStep.Event)
	}
	retryMission := missionFromStep(t, h.Root, retryStep)
	client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, retryMission)})
	client.call("specd_verify", map[string]any{"args": []string{"mcp-life", "T1"}})
	rec := verificationRecord(t, h.Root, "mcp-life", "T1")
	client.structured("specd_pinky", reportArgs(retryMission, rec, "retry passed"))
	if _, err := core.ReconcilePinkyEvidence(h.Root, terminalReport(retryMission, rec, "retry passed"), core.LoadConfig(h.Root).Orchestration); err != nil {
		t.Fatalf("reconcile MCP evidence: %v", err)
	}
	client.call("specd_pinky", leaseArgs("release", retryMission))
	completeStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-life", lifeSession)))
	if completeStep.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("complete decision=%#v, want complete-session", completeStep.Decision)
	}

	const cancelSession = "19191919191919191919191919191919"
	cancelStart := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("start", "mcp-cancel", cancelSession)))
	if cancelStart.Decision.Action != core.OrchestrationDispatch || cancelStart.Event == nil {
		t.Fatalf("cancel start decision=%#v event=%#v, want dispatch", cancelStart.Decision, cancelStart.Event)
	}
	cancelMission := missionFromStep(t, h.Root, cancelStart)
	client.structured("specd_pinky", map[string]any{"args": []string{"claim"}, "mission": writeMCPMission(t, h, cancelMission)})
	cancelled := decodeMCPStructured[core.OrchestrationSession](t, client.structured("specd_brain", map[string]any{"args": []string{"cancel"}, "session": cancelSession}))
	if cancelled.Status != core.OrchestrationSessionCancelling {
		t.Fatalf("cancel status=%s, want cancelling", cancelled.Status)
	}
	directiveStep := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-cancel", cancelSession)))
	if directiveStep.Decision.Action != core.OrchestrationCancel || directiveStep.Event == nil || directiveStep.Event.Type != core.ACPMessageDirective {
		t.Fatalf("cancel step=%#v event=%#v, want cancel directive", directiveStep.Decision, directiveStep.Event)
	}
	client.call("specd_pinky", leaseArgs("release", cancelMission))
	cancelComplete := decodeMCPStructured[core.OrchestrationStepResult](t, client.structured("specd_brain", brainArgs("step", "mcp-cancel", cancelSession)))
	if cancelComplete.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("cancel complete decision=%#v, want complete-session", cancelComplete.Decision)
	}

	return summarizeMCPLifecycle(t, h.Root, lifeSession, cancelSession)
}

func seedLifecycleSpec(h *th.Harness, slug, verify string) {
	h.T.Helper()
	h.Spec(slug).
		Req("demo", "As a user, I want demo.", "THE SYSTEM SHALL satisfy demo.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "do demo", Files: "pass.flag", Verify: verify, Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Orchestrated().
		Build()
}

func brainArgs(subcommand, slug, sessionID string) map[string]any {
	return map[string]any{
		"args":            []string{subcommand, slug},
		"session":         sessionID,
		"approval-policy": "manual",
		"max-workers":     1,
		"max-retries":     2,
		"timeout-seconds": 3600,
	}
}

func leaseArgs(subcommand string, mission core.PinkyMission) map[string]any {
	return map[string]any{
		"args":    []string{subcommand},
		"session": mission.SessionID,
		"worker":  mission.WorkerID,
		"attempt": mission.Attempt,
	}
}

func reportArgs(mission core.PinkyMission, rec *core.VerificationRecord, summary string) map[string]any {
	return map[string]any{
		"args":             []string{"report"},
		"session":          mission.SessionID,
		"worker":           mission.WorkerID,
		"spec":             mission.Spec,
		"task":             mission.TaskID,
		"attempt":          mission.Attempt,
		"verification-ref": core.VerificationRef(rec),
		"summary":          summary,
		"changed-files":    strings.Join(rec.ChangedFiles, ","),
		"git-head":         verificationGitHead(rec),
		"duration-ms":      100,
		"host-tokens":      10,
		"host-cost":        "0.00",
	}
}

func terminalReport(mission core.PinkyMission, rec *core.VerificationRecord, summary string) core.PinkyTerminalReport {
	return core.PinkyTerminalReport{
		SessionID:       mission.SessionID,
		WorkerID:        mission.WorkerID,
		Spec:            mission.Spec,
		TaskID:          mission.TaskID,
		Attempt:         mission.Attempt,
		VerificationRef: core.VerificationRef(rec),
		Summary:         summary,
		ChangedFiles:    append([]string{}, rec.ChangedFiles...),
		GitHead:         verificationGitHead(rec),
		DurationMs:      100,
		HostTokens:      10,
		HostCost:        "0.00",
	}
}

func missionFromStep(t *testing.T, root string, step core.OrchestrationStepResult) core.PinkyMission {
	t.Helper()
	workerID := fmt.Sprintf("%s-a%d", strings.ToLower(step.Decision.TaskID), step.Decision.Attempt)
	mission, err := core.BuildPinkyMission(root, step.Decision.Spec, step.Snapshot.SessionID, workerID, step.Decision.TaskID, step.Decision.Attempt, core.LoadConfig(root).Orchestration)
	if err != nil {
		t.Fatalf("BuildPinkyMission: %v", err)
	}
	return mission
}

func writeMCPMission(t *testing.T, h *th.Harness, mission core.PinkyMission) string {
	t.Helper()
	raw, err := json.MarshalIndent(mission, "", "  ")
	if err != nil {
		t.Fatalf("marshal mission: %v", err)
	}
	dir := h.Path(".specd/tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir mission tmp: %v", err)
	}
	path := filepath.Join(dir, "mission-"+mission.WorkerID+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write mission: %v", err)
	}
	return path
}

func decodeMCPStructured[T any](t *testing.T, structured map[string]any) T {
	t.Helper()
	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode structured content: %v\n%s", err, raw)
	}
	return out
}

func verificationRecord(t *testing.T, root, slug, taskID string) *core.VerificationRecord {
	t.Helper()
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		t.Fatalf("LoadSpec(%s): %v", slug, err)
	}
	rec := loaded.State.Tasks[taskID].Verification
	if rec == nil || !rec.Verified {
		t.Fatalf("verification record for %s/%s = %#v, want passing", slug, taskID, rec)
	}
	return rec
}

func verificationGitHead(rec *core.VerificationRecord) string {
	if rec == nil || rec.GitHead == nil {
		return ""
	}
	return *rec.GitHead
}

func summarizeMCPLifecycle(t *testing.T, root, lifeSession, cancelSession string) mcpLifecycleSummary {
	t.Helper()
	lifeSpec, err := core.LoadSpec(root, "mcp-life")
	if err != nil {
		t.Fatalf("load life spec: %v", err)
	}
	cancelSpec, err := core.LoadSpec(root, "mcp-cancel")
	if err != nil {
		t.Fatalf("load cancel spec: %v", err)
	}
	lifeSessionState, err := core.LoadOrchestrationSession(root, lifeSession)
	if err != nil {
		t.Fatalf("load life session: %v", err)
	}
	cancelSessionState, err := core.LoadOrchestrationSession(root, cancelSession)
	if err != nil {
		t.Fatalf("load cancel session: %v", err)
	}
	cancelEvents := eventSummary(t, root, cancelSession)
	return mcpLifecycleSummary{
		SuccessSpecStatus:  lifeSpec.State.Status,
		SuccessTaskStatus:  lifeSpec.State.Tasks["T1"].Status,
		SuccessSession:     lifeSessionState.Status,
		SuccessEvents:      eventSummary(t, root, lifeSession),
		CancelSpecStatus:   cancelSpec.State.Status,
		CancelTaskStatus:   cancelSpec.State.Tasks["T1"].Status,
		CancelSession:      cancelSessionState.Status,
		CancelEvents:       cancelEvents,
		CancelDirectiveCnt: countPrefix(cancelEvents, "directive/T1/1"),
	}
}

func eventSummary(t *testing.T, root, sessionID string) []string {
	t.Helper()
	store, err := core.NewACPStore(root)
	if err != nil {
		t.Fatalf("NewACPStore: %v", err)
	}
	events, err := store.ReadEvents(sessionID, "summary")
	if err != nil {
		t.Fatalf("ReadEvents(%s): %v", sessionID, err)
	}
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, fmt.Sprintf("%s/%s/%d", event.Type, event.Task, event.Attempt))
	}
	return out
}

func countPrefix(items []string, prefix string) int {
	count := 0
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			count++
		}
	}
	return count
}
