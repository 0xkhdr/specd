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

func TestBrainToolsGatedByConfig(t *testing.T) {
	if got := BrainTools(core.Config{}); len(got) != 0 {
		t.Fatalf("brain tools should be disabled: %#v", got)
	}
	if got := BrainTools(core.Config{Orchestration: core.OrchestrationConfig{Enabled: true}}); len(got) != 1 {
		t.Fatalf("brain tools should be enabled: %#v", got)
	}
}
