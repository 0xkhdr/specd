package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: []byte(`{"name":"approve","arguments":{"args":["demo"]}}`)}, CoreTools(), func(string, []string, map[string]string, *core.AuthorityV1, time.Time) (string, error) {
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

// TestActorOperationEnforcement pins R1.2/R1.3/R1.4 on the MCP transport: a
// caller-supplied actor class is provenance, not attestation, so an operator
// claim over stdio still gets the human handoff — and the claim never reaches
// the dispatcher as a flag.
func TestActorOperationEnforcement(t *testing.T) {
	t.Run("operatorclaimoverstdiostillhandsoff", func(t *testing.T) {
		called := false
		resp := Dispatch(Request{JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: []byte(`{"name":"approve","arguments":{"actor":"operator","args":["demo"]}}`)},
			CoreTools(), func(string, []string, map[string]string, *core.AuthorityV1, time.Time) (string, error) {
				called = true
				return "unexpected", nil
			})
		if called {
			t.Fatal("a spoofed operator claim executed a human-only operation")
		}
		if resp.Error == nil || resp.Error.Code != MCPHandoffRequiredCode {
			t.Fatalf("handoff error = %#v", resp.Error)
		}
		var handoff Handoff
		raw, _ := json.Marshal(resp.Error.Data)
		if err := json.Unmarshal(raw, &handoff); err != nil {
			t.Fatal(err)
		}
		if handoff.Actor != "human" || handoff.Command != "specd approve demo" {
			t.Fatalf("handoff = %+v", handoff)
		}
		if handoff.ObservedActor != core.ActorClassUnknown {
			t.Fatalf("observed actor = %q, want unknown: stdio attests nothing", handoff.ObservedActor)
		}
		if handoff.Assurance != core.AssuranceAdvisory {
			t.Fatalf("assurance = %q, want advisory", handoff.Assurance)
		}
	})

	t.Run("actorclaimneverreachesdispatcher", func(t *testing.T) {
		var gotFlags map[string]string
		var gotArgs []string
		resp := Dispatch(Request{JSONRPC: "2.0", ID: 4, Method: "tools/call",
			Params: []byte(`{"name":"status","arguments":{"actor":"operator","args":["demo"]}}`)},
			CoreTools(), func(_ string, args []string, flags map[string]string, _ *core.AuthorityV1, _ time.Time) (string, error) {
				gotArgs, gotFlags = args, flags
				return "ok", nil
			})
		if resp.Error != nil {
			t.Fatalf("status error = %#v", resp.Error)
		}
		if _, ok := gotFlags["actor"]; ok {
			t.Fatalf("actor claim forwarded as a flag: %v", gotFlags)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "demo" {
			t.Fatalf("args = %v, want the positional operands unchanged", gotArgs)
		}
	})
}
