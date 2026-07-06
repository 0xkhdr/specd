package mcp

import (
	"strings"
	"testing"
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
