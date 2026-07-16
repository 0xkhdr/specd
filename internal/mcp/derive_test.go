package mcp

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestToolSchemaDerivedFromMetadata pins spec 03 R5: every non-forbidden operation
// appears as a tool whose description is the command's own description (no
// hand-authored strings) and whose input schema is generated from flag
// metadata, mapping enums to JSON Schema `enum` and defaults to `default`.
func TestToolSchemaDerivedFromMetadata(t *testing.T) {
	tools := map[string]Tool{}
	for _, tool := range CoreTools() {
		tools[tool.Name] = tool
	}
	for _, operation := range core.Operations {
		if core.ForbiddenTool(operation.Command) {
			continue
		}
		cmd, ok := core.CommandByName(operation.Command)
		if !ok {
			t.Errorf("%s: unknown command %s", operation.ID, operation.Command)
			continue
		}
		tool, ok := tools[operation.ID]
		if !ok {
			t.Errorf("%s: missing from MCP tools", operation.ID)
			continue
		}
		if tool.Description != cmd.Description {
			t.Errorf("%s: tool description %q not derived from metadata %q", operation.ID, tool.Description, cmd.Description)
		}
		props, _ := tool.InputSchema["properties"].(map[string]any)
		for _, flag := range cmd.Flags {
			prop, ok := props[flag.Name].(map[string]any)
			if !ok {
				t.Errorf("%s: flag --%s absent from input schema", operation.ID, flag.Name)
				continue
			}
			if len(flag.Enum) > 0 {
				enum, ok := prop["enum"].([]string)
				if !ok || len(enum) != len(flag.Enum) {
					t.Errorf("%s: flag --%s enum not propagated to schema", operation.ID, flag.Name)
				}
			}
			if flag.Default != "" && prop["default"] != flag.Default {
				t.Errorf("%s: flag --%s default not propagated to schema", operation.ID, flag.Name)
			}
		}
	}
}
