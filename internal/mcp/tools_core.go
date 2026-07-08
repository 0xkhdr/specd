package mcp

import "github.com/0xkhdr/specd/internal/core"

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// CoreTools derives the MCP tool palette entirely from core.Command metadata —
// descriptions and input schemas are generated, never hand-authored, so there
// is one source of truth and no drift (spec 03 R5, C.8). Flag enums map to JSON
// Schema `enum` and declared defaults to `default`.
func CoreTools() []Tool {
	tools := make([]Tool, 0, len(core.Commands))
	for _, command := range core.Commands {
		if core.ForbiddenTool(command.Name) {
			continue
		}
		tools = append(tools, Tool{
			Name:        command.Name,
			Description: command.Description,
			InputSchema: inputSchema(command),
		})
	}
	return tools
}

// inputSchema builds a JSON Schema object for a command's flags. Each flag is a
// property; enums and defaults flow through from the command metadata.
func inputSchema(command core.Command) map[string]any {
	properties := make(map[string]any, len(command.Flags)+1)
	// Positional operands (spec slug, task id, …) travel under the reserved
	// "args" key as an ordered array; splitArguments maps it back to the
	// dispatcher's positional slice.
	properties["args"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Positional arguments (e.g. spec slug, task id), in order.",
	}
	for _, flag := range command.Flags {
		property := map[string]any{
			"type":        jsonType(flag),
			"description": flag.Description,
		}
		if len(flag.Enum) > 0 {
			property["enum"] = flag.Enum
		}
		if flag.Default != "" {
			property["default"] = flag.Default
		}
		properties[flag.Name] = property
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
}

// jsonType maps a flag's declared type to a JSON Schema scalar type. A flag
// that takes no value (or declares "bool") is a boolean switch.
func jsonType(flag core.Flag) string {
	if flag.Type == "string" || flag.TakesValue {
		return "string"
	}
	return "boolean"
}
