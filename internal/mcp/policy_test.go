package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestInitializeCapabilityNegotiation(t *testing.T) {
	resp := Dispatch(Request{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: []byte(`{"driver_capabilities":{"context_loading":true,"sandbox":true}}`),
	}, CoreTools(), nil)
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("initialize result = %#v", resp.Result)
	}
	report, ok := result["driverCapabilities"].(core.CapabilityReport)
	if !ok {
		t.Fatalf("capability report type = %T", result["driverCapabilities"])
	}
	if len(report.Results) != 5 || report.Results[0].Capability != "a2a" {
		t.Fatalf("capability report = %+v", report)
	}
}

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
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: []byte(`{"name":"approve","arguments":{"args":["demo"]}}`)}, CoreTools(), func(string, []string, map[string]string) (string, error) {
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
	if handoff.Code != "MCP_HANDOFF_REQUIRED" || handoff.Actor != "human" || handoff.Command != "specd approve demo" {
		t.Fatalf("handoff = %+v", handoff)
	}
	if !strings.Contains(resp.Error.Message, "MCP_HANDOFF_REQUIRED") {
		t.Fatalf("message = %q", resp.Error.Message)
	}
}
