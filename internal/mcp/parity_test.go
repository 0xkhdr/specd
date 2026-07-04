package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestMCPParity(t *testing.T) {
	tools := CoreTools()
	if len(tools) == 0 {
		t.Fatal("expected tools")
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		seen[tool.Name] = true
		if core.ForbiddenTool(tool.Name) {
			t.Fatalf("forbidden tool exposed: %s", tool.Name)
		}
	}
	for _, command := range core.CommandNames() {
		if core.ForbiddenTool(command) {
			continue
		}
		if !seen[command] {
			t.Fatalf("command missing from MCP tools: %s", command)
		}
	}

	var out bytes.Buffer
	err := Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, tools)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response Response
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("response json: %v", err)
	}
	if response.Error != nil || response.Result == nil {
		t.Fatalf("bad response: %#v", response)
	}
}

// TestDenyList pins the MCP deny list itself (R2.1): the named human-gate and
// host-only verbs must be absent from tools/list AND refused by tools/call.
// Removing any entry from core.ForbiddenTool breaks CI here at both layers.
func TestDenyList(t *testing.T) {
	denied := []string{"approve", "init", "mcp", "brain"}

	listed := map[string]bool{}
	for _, tool := range CoreTools() {
		listed[tool.Name] = true
	}
	for _, name := range denied {
		if listed[name] {
			t.Fatalf("tools/list must exclude %q", name)
		}
		resp := Dispatch(Request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: []byte(`{"name":"` + name + `"}`),
		}, CoreTools())
		if resp.Error == nil || resp.Error.Code != -32001 {
			t.Fatalf("tools/call %q: want policy error -32001, got %#v", name, resp.Error)
		}
	}
}

func TestBrainToolsGatedByConfig(t *testing.T) {
	if got := BrainTools(core.Config{}); len(got) != 0 {
		t.Fatalf("brain tools should be disabled: %#v", got)
	}
	if got := BrainTools(core.Config{Orchestration: core.OrchestrationConfig{Enabled: true}}); len(got) != 1 {
		t.Fatalf("brain tools should be enabled: %#v", got)
	}
}
