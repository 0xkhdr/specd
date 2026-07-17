package mcp

import "github.com/0xkhdr/specd/internal/core"

type Tool struct {
	Name              string               `json:"name"`
	Command           string               `json:"command"`
	Actor             core.OperationActor  `json:"actor"`
	Effect            core.OperationEffect `json:"effect"`
	Mutable           bool                 `json:"mutable"`
	AuthorityRequired bool                 `json:"authority_required"`
	TaskRequired      bool                 `json:"task_required"`
	Description       string               `json:"description"`
	InputSchema       map[string]any       `json:"inputSchema"`
}

// CoreTools derives the MCP tool palette entirely from core.Command metadata —
// descriptions and input schemas are generated, never hand-authored, so there
// is one source of truth and no drift (spec 03 R5, C.8). Flag enums map to JSON
// Schema `enum` and declared defaults to `default`.
func CoreTools() []Tool {
	tools := make([]Tool, 0, len(core.Operations))
	for _, operation := range core.Operations {
		if core.ForbiddenTool(operation.Command) {
			continue
		}
		command, ok := core.CommandByName(operation.Command)
		if !ok {
			continue
		}
		tools = append(tools, Tool{
			Name: operation.ID, Command: operation.Command, Actor: operation.Actor,
			Effect: operation.Effect, Mutable: operation.Effect != core.EffectRead,
			AuthorityRequired: operation.AuthorityRequired, TaskRequired: operation.TaskRequired,
			Description: command.Description, InputSchema: inputSchema(command, operation),
		})
	}
	return tools
}

// inputSchema builds a JSON Schema object for a command's flags. Each flag is a
// property; enums and defaults flow through from the command metadata.
func inputSchema(command core.Command, operation core.Operation) map[string]any {
	properties := make(map[string]any, len(command.Flags)+1)
	// Positional operands (subcommand, spec slug, task id, …) travel under the
	// reserved "args" key as an ordered array; splitArguments maps it back to
	// the dispatcher's positional slice. The description carries the command's
	// exact usage signature so required operands are documented per-tool and
	// never drift from dispatch (spec 03 R5, C.8).
	properties["args"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": argsDescription(command, operation),
	}
	if operation.TaskRequired {
		properties["authority"] = map[string]any{
			"type":        "object",
			"description": "Digest-pinned AuthorityV1 packet required for production task operations.",
		}
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

// argsDescription documents the ordered positional operands for a tool by
// projecting the command's exact usage signature (the single source of truth
// in core.Command) into the "args" schema. Missing or wrong operands then fail
// against dispatch's own arg validation, and the agent sees the required shape
// up front — e.g. handshake surfaces `usage: handshake bootstrap <spec>`.
func argsDescription(command core.Command, operation core.Operation) string {
	description := "Positional arguments, in order. usage: " + command.Usage
	if len(operation.Examples) > 0 {
		description += " — example: " + operation.Examples[0]
	}
	return description
}

// jsonType maps a flag's declared type to a JSON Schema scalar type. A flag
// that takes no value (or declares "bool") is a boolean switch.
func jsonType(flag core.Flag) string {
	if flag.Type == "string" || flag.TakesValue {
		return "string"
	}
	return "boolean"
}
