package mcp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// drive feeds newline-framed JSON-RPC requests through a real server and returns
// the parsed responses, in order. Notifications produce no response.
func drive(t *testing.T, requests ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := mcp.Serve(in, &out, cmd.Dispatch, nil); err != nil {
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

// seedSpec builds a gate-clean executing spec with one runnable, verifiable task.
func seedSpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()
}

func driveInDir(t *testing.T, dir string, requests ...string) []map[string]any {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return drive(t, requests...)
}

func result(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result object: %v", resp)
	}
	return r
}

func TestInitializeProtocolVersionNegotiation(t *testing.T) {
	cases := []struct {
		name   string
		params string
		want   string
	}{
		{"newest", `{"protocolVersion":"2025-11-25"}`, "2025-11-25"},
		{"previous", `{"protocolVersion":"2025-06-18"}`, "2025-06-18"},
		{"legacy", `{"protocolVersion":"2024-11-05"}`, "2024-11-05"},
		{"unsupported falls back to latest", `{"protocolVersion":"2099-01-01"}`, "2025-11-25"},
		{"missing preserves old client compatibility", `{}`, "2024-11-05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resps := drive(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":`+tc.params+`}`)
			if got := result(t, resps[0])["protocolVersion"]; got != tc.want {
				t.Errorf("protocolVersion = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMCPInstructionsAgentBudget(t *testing.T) {
	resps := drive(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)
	init := result(t, resps[0])
	instructions, ok := init["instructions"].(string)
	if !ok || instructions == "" {
		t.Fatalf("instructions missing: %v", init)
	}
	if len(instructions) > 512 {
		t.Fatalf("instructions use %d bytes, want <= 512", len(instructions))
	}
	for _, phrase := range []string{
		"specd_fusion bootstrap/policy",
		"specd_status",
		"specd_context",
		"specd_help --json/schema",
		"brain_* intents",
		"specd_brain start/step/status",
		"watch --once only",
		"host runs Pinky workers",
		"Never edit state/tasks checkboxes",
		"Approval policy-gated",
		"specd_verify evidence",
	} {
		if !strings.Contains(instructions, phrase) {
			t.Errorf("instructions missing %q: %q", phrase, instructions)
		}
	}

	agents, err := core.ReadTemplate("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadTemplate(AGENTS.md): %v", err)
	}
	for _, phrase := range []string{"specd context", "Never hand-edit `state.json`", "specd check", "specd verify", "--evidence"} {
		if !strings.Contains(agents, phrase) {
			t.Errorf("embedded AGENTS.md missing rule corresponding to instructions: %q", phrase)
		}
	}
}

func TestMCPBoundedWatchRequiresOnce(t *testing.T) {
	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_watch","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_watch","arguments":{"once":true,"sse":"127.0.0.1:0"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	if len(resps) != 3 {
		t.Fatalf("got %d responses, want 3: %v", len(resps), resps)
	}
	for i, want := range []string{"requires --once", "does not allow --sse or --webhook"} {
		e, ok := resps[i]["error"].(map[string]any)
		if !ok {
			t.Fatalf("response %d should be an MCP error: %v", i, resps[i])
		}
		if e["code"].(float64) != -32602 {
			t.Errorf("response %d code = %v, want -32602", i, e["code"])
		}
		if !strings.Contains(e["message"].(string), want) {
			t.Errorf("response %d message missing %q: %q", i, want, e["message"])
		}
	}
	if _, ok := resps[2]["result"]; !ok {
		t.Fatalf("server did not answer ping after bounded-watch errors: %v", resps[2])
	}
}

func TestMCPResourcesMissingOrCorruptRootReturnErrors(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		root := t.TempDir()
		resps := driveInDir(t, root,
			`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		)
		if len(resps) != 2 {
			t.Fatalf("got %d responses, want 2: %v", len(resps), resps)
		}
		e, ok := resps[0]["error"].(map[string]any)
		if !ok {
			t.Fatalf("missing-root response should be error: %v", resps[0])
		}
		if e["code"].(float64) != -32600 || !strings.Contains(e["message"].(string), "workspace root missing") {
			t.Fatalf("missing-root error = %v", e)
		}
		if _, ok := resps[1]["result"]; !ok {
			t.Fatalf("server did not answer ping after missing-root error: %v", resps[1])
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".specd"), []byte("not-a-dir"), 0o644); err != nil {
			t.Fatalf("write corrupt .specd: %v", err)
		}
		resps := driveInDir(t, root,
			`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"specd://steering/reasoning.md"}}`,
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		)
		if len(resps) != 2 {
			t.Fatalf("got %d responses, want 2: %v", len(resps), resps)
		}
		e, ok := resps[0]["error"].(map[string]any)
		if !ok {
			t.Fatalf("corrupt-root response should be error: %v", resps[0])
		}
		if e["code"].(float64) != -32600 || !strings.Contains(e["message"].(string), "workspace root corrupt") {
			t.Fatalf("corrupt-root error = %v", e)
		}
		if _, ok := resps[1]["result"]; !ok {
			t.Fatalf("server did not answer ping after corrupt-root error: %v", resps[1])
		}
	})
}

// TestToolsCall drives tools/call into the real handlers and checks the result
// payload: read-only status returns structured JSON, verify runs the task's
// command, and a non-zero exit maps to isError (R3).
func TestToolsCall(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	t.Run("read_only_status_returns_structured_json", func(t *testing.T) {
		resps := drive(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["auth"]}}}`)
		r := result(t, resps[0])
		if r["isError"] != false {
			t.Errorf("isError = %v, want false", r["isError"])
		}
		if _, ok := r["structuredContent"]; !ok {
			t.Errorf("status should attach structuredContent: %v", r)
		}
		content := r["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, `"spec": "auth"`) {
			t.Errorf("content text missing spec: %q", text)
		}
	})

	t.Run("verify_runs_the_task_command", func(t *testing.T) {
		resps := drive(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_verify","arguments":{"args":["auth","T1"]}}}`)
		r := result(t, resps[0])
		if r["isError"] != false {
			t.Errorf("isError = %v, want false", r["isError"])
		}
		text := r["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, "verified") {
			t.Errorf("verify text missing 'verified': %q", text)
		}
	})

	t.Run("non_zero_exit_maps_to_iserror", func(t *testing.T) {
		// Unknown spec → not-found exit → isError:true, server stays up.
		resps := drive(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"specd_check","arguments":{"args":["ghost"]}}}`)
		r := result(t, resps[0])
		if r["isError"] != true {
			t.Errorf("isError = %v, want true", r["isError"])
		}
		text := r["content"].([]any)[0].(map[string]any)["text"].(string)
		if strings.TrimSpace(text) == "" {
			t.Errorf("error result should carry diagnostic text")
		}
	})

	t.Run("lock_contention_reports_structured_status", func(t *testing.T) {
		t.Setenv("SPECD_LOCK_TIMEOUT_MS", "50")
		lockPath := core.SpecDir(h.Root, "auth") + "/.lock"
		if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d %d\n", os.Getpid(), time.Now().UnixMilli())), 0o644); err != nil {
			t.Fatalf("write lock file: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(lockPath) })
		resps := drive(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"specd_task","arguments":{"args":["auth","T1"],"status":"running"}}}`)
		r := result(t, resps[0])
		if r["isError"] != true {
			t.Fatalf("lock contention isError = %v, want true", r["isError"])
		}
		if got, _ := r["status"].(string); got != "locked" {
			t.Fatalf("lock contention status = %q, want locked", got)
		}
		text := r["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, "locked by another specd process") {
			t.Fatalf("lock contention text missing lock diagnostic: %q", text)
		}
	})

	t.Run("unknown_tool_is_invalid_params", func(t *testing.T) {
		resps := drive(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"specd_bogus","arguments":{}}}`)
		e, ok := resps[0]["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected error, got %v", resps[0])
		}
		if e["code"].(float64) != -32602 {
			t.Errorf("code = %v, want -32602", e["code"])
		}
	})

	// R1.3: malformed arguments (args must be an array) yield a structured MCP
	// error, never a crash, and the server stays up to answer the next request.
	t.Run("invalid_argument_shape_is_structured_error_server_survives", func(t *testing.T) {
		resps := drive(t,
			`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":"not-an-array"}}}`,
			`{"jsonrpc":"2.0","id":6,"method":"ping"}`,
		)
		if len(resps) != 2 {
			t.Fatalf("got %d responses, want 2: %v", len(resps), resps)
		}
		e, ok := resps[0]["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected structured error, got %v", resps[0])
		}
		if e["code"].(float64) != -32602 {
			t.Errorf("code = %v, want -32602", e["code"])
		}
		if _, ok := resps[1]["result"]; !ok {
			t.Errorf("server did not answer ping after bad-arg error: %v", resps[1])
		}
	})
}

// TestMCPEndToEnd drives a full handshake over a pipe — initialize → tools/list
// → tools/call → malformed → a recovery call — asserting JSON-RPC compliance and
// that a malformed request never kills the server (R1, R2, R3, R5).
func TestMCPEndToEnd(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // notification: no response
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["auth"]}}}`,
		`{ this is not valid json`, // malformed: error, server survives
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`,
	)

	if len(resps) != 5 {
		t.Fatalf("got %d responses, want 5 (notification yields none): %v", len(resps), resps)
	}

	// initialize
	init := result(t, resps[0])
	if init["protocolVersion"] == nil {
		t.Errorf("initialize missing protocolVersion: %v", init)
	}
	if si, ok := init["serverInfo"].(map[string]any); !ok || si["name"] != "specd" {
		t.Errorf("initialize serverInfo = %v", init["serverInfo"])
	}

	// tools/list — parity with non-meta command count (R2) plus intent tools.
	wantTools := mcp.IntentToolCount
	for _, c := range core.Commands {
		if c.Command != "help" && c.Command != "version" && c.Command != "mcp" {
			wantTools++
		}
	}
	tools := result(t, resps[1])["tools"].([]any)
	if len(tools) != wantTools {
		t.Errorf("tools/list count = %d, want %d", len(tools), wantTools)
	}

	// tools/call
	if result(t, resps[2])["isError"] != false {
		t.Errorf("status call isError = %v, want false", result(t, resps[2])["isError"])
	}

	// malformed → parse error, server still alive
	if e, ok := resps[3]["error"].(map[string]any); !ok || e["code"].(float64) != -32700 {
		t.Errorf("malformed input should yield -32700 parse error, got %v", resps[3])
	}

	// recovery call after malformed input proves the loop survived
	if result(t, resps[4]) == nil {
		t.Errorf("server did not answer after malformed input: %v", resps[4])
	}
}
