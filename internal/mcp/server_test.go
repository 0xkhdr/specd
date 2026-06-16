package mcp_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
	if err := mcp.Serve(in, &out, cmd.Dispatch); err != nil {
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

func result(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result object: %v", resp)
	}
	return r
}

// TestToolsCall drives tools/call into the real handlers and checks the result
// payload: read-only status returns structured JSON, verify runs the task's
// command, and a non-zero exit maps to isError (R3).
func TestToolsCall(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	t.Run("read-only status returns structured JSON", func(t *testing.T) {
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

	t.Run("verify runs the task command", func(t *testing.T) {
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

	t.Run("non-zero exit maps to isError", func(t *testing.T) {
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

	t.Run("unknown tool is invalid params", func(t *testing.T) {
		resps := drive(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"specd_bogus","arguments":{}}}`)
		e, ok := resps[0]["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected error, got %v", resps[0])
		}
		if e["code"].(float64) != -32602 {
			t.Errorf("code = %v, want -32602", e["code"])
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
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
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

	// tools/list — parity with non-meta command count (R2).
	wantTools := 0
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
