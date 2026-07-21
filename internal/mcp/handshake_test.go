package mcp

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestInitializeHandshake pins that the server answers the MCP `initialize`
// request. Without it a compliant client (Claude Code, Cursor) never completes
// the handshake, so even tools/list discovery is unreachable.
func TestInitializeHandshake(t *testing.T) {
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}, CoreTools(), nil)
	if resp.Error != nil {
		t.Fatalf("initialize returned error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("initialize result type = %T, want map", resp.Result)
	}
	if result["protocolVersion"] == nil {
		t.Errorf("initialize result missing protocolVersion")
	}
	if result["serverInfo"] == nil {
		t.Errorf("initialize result missing serverInfo")
	}
}

// TestNotificationGetsNoReply pins that a JSON-RPC notification (no id) is not
// answered — replying to one is a protocol violation. Serve drives this; the
// input carries notifications/initialized with no id.
func TestNotificationGetsNoReply(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out strings.Builder
	if err := Serve(in, &out, CoreTools(), nil); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("notification drew a reply: %q", out.String())
	}
}

func TestHandshakeDriverProtocolVersion(t *testing.T) {
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}, CoreTools(), nil)
	result := resp.Result.(map[string]any)
	if result["driverProtocolVersion"] != "1" {
		t.Fatalf("driver protocol version = %#v", result["driverProtocolVersion"])
	}
}

func TestRequestModeGuideConformance(t *testing.T) {
	general := RequestModeGuide(core.RequestModeResolution{Mode: core.RequestModeGeneral, Assurance: core.AssuranceAdvisory}, "")
	if strings.Contains(general, "`specd ") || !strings.HasPrefix(general, "Request mode: general") {
		t.Fatalf("general MCP guide = %q", general)
	}
	managed := RequestModeGuide(core.RequestModeResolution{Mode: core.RequestModeManaged, SelectedSpec: "demo", Assurance: core.AssuranceAdvisory}, "T1")
	if !strings.Contains(managed, "Run `specd handshake bootstrap demo --json` first") || !strings.Contains(managed, "not enforced") {
		t.Fatalf("managed MCP guide = %q", managed)
	}
}
