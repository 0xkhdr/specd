package mcp

import "github.com/0xkhdr/specd/internal/core"

// toolPrefix namespaces specd commands as MCP tools so a host sees specd_status,
// specd_verify, etc. — recognisable and collision-free in a shared tool list.
const toolPrefix = "specd_"

// metaCommands are core.Commands entries that are NOT exposed as MCP tools:
// help/version are handled before dispatch, and mcp is the server itself
// (exposing it as a tool would let a host recursively spawn servers).
var metaCommands = map[string]bool{"help": true, "version": true, "mcp": true}

// readOnlyCommands never mutate spec state (spec R4). Every other exposed
// command is annotated readOnlyHint:false so a host knows it may change state.
var readOnlyCommands = map[string]bool{
	"status": true, "waves": true, "context": true, "check": true,
	"next": true, "dispatch": true, "report": true,
	"serve": true, "watch": true, "validate": true, "replay": true, "diff": true,
}

// destructiveCommands mutate the install itself rather than spec state; they are
// additionally flagged so a host can warn before invoking them.
var destructiveCommands = map[string]bool{"uninstall": true, "update": true}

type toolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
}

type schemaProp struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Items       *schemaProp `json:"items,omitempty"`
}

type jsonSchema struct {
	Type                 string                `json:"type"`
	Properties           map[string]schemaProp `json:"properties"`
	AdditionalProperties bool                  `json:"additionalProperties"`
}

type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema jsonSchema      `json:"inputSchema"`
	Annotations toolAnnotations `json:"annotations"`

	// intent marks a semantic, intent-level tool (GAP-5) that wraps the
	// deterministic primitives with sane defaults rather than mirroring one
	// command 1:1. Such tools model named arguments instead of a positional
	// "args" array and are translated to an argv before dispatch.
	intent bool
}

// commandToTool maps one command's help metadata to an MCP tool definition.
// Positionals are modelled as an ordered "args" array (the help schema embeds
// them in the usage string rather than naming them individually); each flag
// becomes a typed property.
func commandToTool(c core.CommandMeta) toolDef {
	props := map[string]schemaProp{
		"args": {
			Type:        "array",
			Description: "Positional arguments, in order. Usage: " + c.Synopsis,
			Items:       &schemaProp{Type: "string"},
		},
	}
	for _, f := range c.Flags {
		t := "string"
		if f.Type == "boolean" {
			t = "boolean"
		}
		props[f.Name] = schemaProp{Type: t, Description: f.Description}
	}
	return toolDef{
		Name:        toolPrefix + c.Command,
		Description: c.Description,
		InputSchema: jsonSchema{Type: "object", Properties: props, AdditionalProperties: false},
		Annotations: toolAnnotations{
			ReadOnlyHint:    readOnlyCommands[c.Command],
			DestructiveHint: destructiveCommands[c.Command],
		},
	}
}

// buildTools generates the MCP tool list: one command-mirror tool per non-meta
// core.Commands entry (raw passthrough, stable help-display order) followed by
// the intent-level orchestration tools (GAP-5). A new command surfaces as a
// passthrough tool with no separate registration; intent tools give a model a
// single high-level affordance over the same deterministic primitives.
func buildTools() []toolDef {
	tools := make([]toolDef, 0, len(core.Commands)+len(intentTools))
	for _, c := range core.Commands {
		if metaCommands[c.Command] {
			continue
		}
		tools = append(tools, commandToTool(c))
	}
	for _, it := range intentTools {
		tools = append(tools, it.def())
	}
	return tools
}
