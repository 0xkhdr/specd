package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

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
//  1. initialize  → protocol version "2024-11-05" and tools capability
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
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
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
		if got := init["protocolVersion"]; got != "2024-11-05" {
			t.Errorf("protocolVersion = %q, want %q", got, "2024-11-05")
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
