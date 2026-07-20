package mcp

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestAssuranceLevelOnHandshake pins R3.1/R3.2 on the machine surface a driver
// actually reads: the initialize result states the assurance level, and a host
// that declared no sandbox is told advisory rather than left to assume it is
// fully governed.
func TestAssuranceLevelOnHandshake(t *testing.T) {
	for _, tc := range []struct {
		name string
		host core.HostCapabilities
		want core.AssuranceLevel
	}{
		{"no capabilities declared", core.HostCapabilities{}, core.AssuranceAdvisory},
		{"everything but sandbox", core.HostCapabilities{ContextLoading: true, Telemetry: true, Eval: true, A2A: true}, core.AssuranceAdvisory},
		{"sandbox declared", core.HostCapabilities{Sandbox: true}, core.AssuranceSandboxed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params, err := json.Marshal(map[string]any{"driver_capabilities": tc.host})
			if err != nil {
				t.Fatal(err)
			}
			resp := Dispatch(Request{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: params}, CoreTools(), nil)
			if resp.Error != nil {
				t.Fatalf("initialize error: %+v", resp.Error)
			}
			result, ok := resp.Result.(map[string]any)
			if !ok {
				t.Fatalf("result type = %T, want map", resp.Result)
			}
			got, ok := result["assurance"]
			if !ok {
				t.Fatal("initialize result carries no assurance level")
			}
			if got != tc.want {
				t.Errorf("assurance = %v, want %q", got, tc.want)
			}
		})
	}

	// An omitted params block is the same absence of a declaration, so it must
	// land on the fail-safe rather than the default zero of some other field.
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}, CoreTools(), nil)
	if result, ok := resp.Result.(map[string]any); !ok || result["assurance"] != core.AssuranceAdvisory {
		t.Fatalf("initialize without params: assurance = %v, want advisory", resp.Result)
	}
}
