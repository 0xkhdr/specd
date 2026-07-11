package mcp

import "testing"

// TestStatusToolExposesGuide pins spec 01 R6.1 over MCP: the machine driving
// guidance is reachable as a host tool. Because the tool palette is derived from
// command metadata, the `status` tool must surface the `guide` flag in its
// generated input schema so an agent can request guidance without a bespoke
// tool.
func TestStatusToolExposesGuide(t *testing.T) {
	var status *Tool
	for i := range CoreTools() {
		if tool := CoreTools()[i]; tool.Name == "status" {
			status = &tool
			break
		}
	}
	if status == nil {
		t.Fatal("status tool not found")
	}
	props, ok := status.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("status tool has no properties: %+v", status.InputSchema)
	}
	if _, ok := props["guide"]; !ok {
		t.Fatalf("status tool does not expose the guide flag: %v", props)
	}
}
