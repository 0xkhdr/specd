package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestForbiddenToolsDeniedOnCall(t *testing.T) {
	for _, name := range []string{"approve", "init", "mcp", "brain", "task"} {
		resp := Dispatch(Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  []byte(`{"name":"` + name + `"}`),
		}, CoreTools(), nil)
		if resp.Error == nil {
			t.Fatalf("%s: expected policy error", name)
		}
		wantCode := -32001
		if name == "approve" {
			wantCode = MCPHandoffRequiredCode
		}
		if resp.Error.Code != wantCode {
			t.Fatalf("%s: expected policy error code, got %d", name, resp.Error.Code)
		}
	}
}

func TestTypedHumanHandoffHasNoSideEffect(t *testing.T) {
	called := false
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: []byte(`{"name":"approve","arguments":{"args":["demo","design"]}}`)}, CoreTools(), func(string, []string, map[string]string) (string, error) {
		called = true
		return "unexpected", nil
	})
	if called {
		t.Fatal("human-only MCP refusal executed handler")
	}
	if resp.Error == nil || resp.Error.Code != MCPHandoffRequiredCode {
		t.Fatalf("handoff error = %#v", resp.Error)
	}
	var handoff Handoff
	raw, _ := json.Marshal(resp.Error.Data)
	if err := json.Unmarshal(raw, &handoff); err != nil {
		t.Fatal(err)
	}
	if handoff.Code != "MCP_HANDOFF_REQUIRED" || handoff.Actor != "human" || handoff.Command != "specd approve demo design" {
		t.Fatalf("handoff = %+v", handoff)
	}
	if !strings.Contains(resp.Error.Message, "MCP_HANDOFF_REQUIRED") {
		t.Fatalf("message = %q", resp.Error.Message)
	}
}
