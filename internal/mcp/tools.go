package mcp

import (
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

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

// metaRiskCommands are spec-pack-author / install-maintenance tools hidden from
// the default MCP surface (spec R4); they reappear only under includeMeta:true.
// schema is reclassified here (spec §5.5): it stays a CLI command, but is no
// longer advertised as an MCP tool by default.
var metaRiskCommands = map[string]bool{"update": true, "uninstall": true, "schema": true}

// orchestrationCommands gate the Brain/Pinky surface (spec R5). They are hidden
// unless orchestration is included (explicitly via includeOrchestration or
// derived from orchestration.enabled). Every intent tool is `brain_*`, so the
// same gate hides all of intentTools.
var orchestrationCommands = map[string]bool{"brain": true, "pinky": true}

// defaultEssentialTools is the built-in expose:"essential" set used when
// mcp.essentialTools is empty (spec R3a): the minimal day-to-day driving loop.
var defaultEssentialTools = []string{
	"status", "context", "check", "next", "verify", "task", "approve", "report",
}

// exposurePlan is the resolved, pure allow-policy derived from a *core.Config.
// buildTools consults it per tool so the loop stays a thin filter and the
// resolution logic is table-testable in isolation (spec §5.3).
type exposurePlan struct {
	// passthrough emits every non-meta tool with no gating — the backward-compat
	// path for an absent `mcp` block (spec R1).
	passthrough          bool
	essential            bool
	essentialSet         map[string]bool
	includeMeta          bool
	includeOrchestration bool
}

// resolveMCPExposure turns config into an exposurePlan (pure; no I/O except the
// R6 diagnostic, which goes to stderr — never the protocol stream). A nil or
// unconfigured config yields passthrough so tests and absent blocks match today.
func resolveMCPExposure(cfg *core.Config) exposurePlan {
	if cfg == nil || !cfg.MCP.Configured() {
		return exposurePlan{passthrough: true}
	}
	mc := cfg.MCP

	expose := mc.Expose
	switch expose {
	case "", "all":
		expose = "all"
	case "essential":
		// keep
	default:
		// R6: an unknown mode degrades to "all" plus one stderr diagnostic.
		fmt.Fprintf(os.Stderr, "specd mcp: unknown expose mode %q; treating as \"all\"\n", mc.Expose)
		expose = "all"
	}

	// R5a: an unset includeOrchestration derives from orchestration.enabled.
	includeOrch := cfg.Orchestration.Enabled
	if mc.IncludeOrchestration != nil {
		includeOrch = *mc.IncludeOrchestration
	}

	plan := exposurePlan{
		essential:            expose == "essential",
		includeMeta:          mc.IncludeMeta,
		includeOrchestration: includeOrch,
	}
	if plan.essential {
		names := mc.EssentialTools
		if len(names) == 0 {
			names = defaultEssentialTools
		}
		plan.essentialSet = make(map[string]bool, len(names))
		for _, n := range names {
			plan.essentialSet[n] = true
		}
	}
	return plan
}

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
// A nil cfg (or an absent `mcp` block) yields the full, pre-config set so
// existing hosts see byte-identical output (spec R1). Otherwise the resolved
// exposurePlan filters by expose mode, meta gating, and orchestration gating
// while preserving deterministic command-then-intent order (spec R7).
func buildTools(cfg *core.Config) []toolDef {
	plan := resolveMCPExposure(cfg)
	tools := make([]toolDef, 0, len(core.Commands)+len(intentTools))
	for _, c := range core.Commands {
		if metaCommands[c.Command] {
			continue
		}
		if !plan.passthrough {
			if metaRiskCommands[c.Command] && !plan.includeMeta {
				continue
			}
			if orchestrationCommands[c.Command] && !plan.includeOrchestration {
				continue
			}
			if plan.essential && !plan.essentialSet[c.Command] {
				continue
			}
		}
		tools = append(tools, commandToTool(c))
	}
	for _, it := range intentTools {
		if !plan.passthrough {
			// Every intent tool is `brain_*`, so the orchestration gate hides them all.
			if !plan.includeOrchestration {
				continue
			}
			if plan.essential && !plan.essentialSet[it.name] {
				continue
			}
		}
		tools = append(tools, it.def())
	}
	return tools
}
