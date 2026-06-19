package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCP Prompts (spec B2). specd's phase and role guidance reaches a host through
// the native `prompts` channel rather than tool calls. Content is embedded and
// fully deterministic — no network, no LLM, no filesystem dependency — so the
// same name+arguments always render byte-identical messages (spec R6).

// errPromptNotFound is returned by prompts/get for an unknown prompt name.
const errPromptNotFound = -32002

// promptArg declares one prompt argument advertised in prompts/list (spec R7).
type promptArg struct {
	name        string
	description string
	required    bool
}

// promptMessage is one rendered message in a prompts/get result.
type promptMessage struct {
	role string
	text string
}

// promptDef is a registered prompt plus its deterministic renderer.
type promptDef struct {
	name        string
	description string
	args        []promptArg
	// render assembles the prompt's messages from embedded text and the supplied
	// arguments. It performs pure string assembly only (spec R4/R6).
	render func(args map[string]any) []promptMessage
}

// phasePrompt builds a phase prompt whose optional `slug` argument injects a
// one-line spec-context header ahead of the embedded phase guidance (spec R4).
func phasePrompt(phase, body string) func(map[string]any) []promptMessage {
	return func(args map[string]any) []promptMessage {
		text := body
		if slug := promptSlug(args); slug != "" {
			text = fmt.Sprintf("Active spec: %s — phase: %s.\n\n%s", slug, phase, body)
		}
		return []promptMessage{{role: "user", text: text}}
	}
}

// rolePrompt builds a role prompt with fixed embedded contract text.
func rolePrompt(body string) func(map[string]any) []promptMessage {
	return func(map[string]any) []promptMessage {
		return []promptMessage{{role: "user", text: body}}
	}
}

// promptSlug reads an optional string `slug` argument; non-strings are ignored so
// rendering never errors on a malformed argument (determinism over strictness).
func promptSlug(args map[string]any) string {
	if v, ok := args["slug"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// slugArg is the shared optional spec-slug argument for phase prompts.
var slugArg = []promptArg{{name: "slug", description: "Active spec slug to scope the phase guidance to."}}

// prompts is the registered prompt set in stable order: four phase prompts then
// two role prompts (spec R2). Order pins prompts/list determinism.
var prompts = []promptDef{
	{
		name:        "phase/requirements",
		description: "Guidance for the requirements phase: capture EARS-form acceptance criteria before any design.",
		args:        slugArg,
		render:      phasePrompt("requirements", requirementsPrompt),
	},
	{
		name:        "phase/design",
		description: "Guidance for the design phase: turn approved requirements into a traceable technical design.",
		args:        slugArg,
		render:      phasePrompt("design", designPrompt),
	},
	{
		name:        "phase/tasks",
		description: "Guidance for the tasks phase: decompose the design into verifiable, dependency-ordered tasks.",
		args:        slugArg,
		render:      phasePrompt("tasks", tasksPrompt),
	},
	{
		name:        "phase/execute",
		description: "Guidance for the execute phase: drive tasks to verified completion with evidence.",
		args:        slugArg,
		render:      phasePrompt("execute", executePrompt),
	},
	{
		name:        "role/builder",
		description: "The builder role contract: implement a single task within its declared file scope.",
		render:      rolePrompt(builderPrompt),
	},
	{
		name:        "role/investigator",
		description: "The investigator role contract: locate and report, never modify.",
		render:      rolePrompt(investigatorPrompt),
	},
}

// promptByName indexes prompts for O(1) lookup in prompts/get.
var promptByName = func() map[string]promptDef {
	m := make(map[string]promptDef, len(prompts))
	for _, p := range prompts {
		m[p.name] = p
	}
	return m
}()

// handlePromptsList advertises every prompt with its declared arguments (spec
// R2/R7) in registration order.
func handlePromptsList() map[string]any {
	out := make([]map[string]any, 0, len(prompts))
	for _, p := range prompts {
		args := make([]map[string]any, 0, len(p.args))
		for _, a := range p.args {
			args = append(args, map[string]any{
				"name":        a.name,
				"description": a.description,
				"required":    a.required,
			})
		}
		out = append(out, map[string]any{
			"name":        p.name,
			"description": p.description,
			"arguments":   args,
		})
	}
	return map[string]any{"prompts": out}
}

// handlePromptGet renders a known prompt's messages (spec R3); an unknown name is
// a prompt-not-found error (spec R5).
func handlePromptGet(name string, args map[string]any) (map[string]any, *rpcError) {
	p, ok := promptByName[name]
	if !ok {
		return nil, &rpcError{Code: errPromptNotFound, Message: "prompt not found: " + name}
	}
	rendered := p.render(args)
	messages := make([]map[string]any, 0, len(rendered))
	for _, m := range rendered {
		messages = append(messages, map[string]any{
			"role":    m.role,
			"content": map[string]any{"type": "text", "text": m.text},
		})
	}
	return map[string]any{"description": p.description, "messages": messages}, nil
}

// promptGetParams is the prompts/get request shape.
type promptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func parsePromptGet(raw json.RawMessage) promptGetParams {
	var p promptGetParams
	_ = json.Unmarshal(raw, &p)
	return p
}

// Embedded prompt bodies. These are the single source of truth for phase and role
// guidance over MCP; they intentionally mirror specd's workflow contract and are
// pinned by golden tests so changes are deliberate (spec §8).
const (
	requirementsPrompt = "You are in the requirements phase. Produce numbered, testable acceptance " +
		"criteria in EARS form (WHEN/WHERE/WHILE … THE SYSTEM SHALL …). Do not design or " +
		"implement yet. Each requirement must be independently verifiable. Stop and request " +
		"approval before advancing to design."

	designPrompt = "You are in the design phase. Turn the approved requirements into a technical " +
		"design: components, data shapes, and control flow. Every design element must trace back " +
		"to a requirement id. Do not write tasks or code yet. Request approval before advancing to tasks."

	tasksPrompt = "You are in the tasks phase. Decompose the approved design into small, verifiable " +
		"tasks. Each task declares its file scope, dependencies, and the requirement ids it satisfies, " +
		"and carries a concrete verify command. Order tasks by dependency. Do not implement yet."

	executePrompt = "You are in the execute phase. Drive one task at a time to verified completion: " +
		"implement only within the task's declared file scope, run its verify command, and record " +
		"evidence. Never hand-edit state.json or tasks.md checkboxes — let specd verify update state."

	builderPrompt = "Role: builder. Implement exactly one task within its declared `files:` scope. " +
		"Do not touch files outside that scope. Make the change, run the task's verify command, and " +
		"report the evidence. If the task is underspecified or blocked, report the blocker instead of guessing."

	investigatorPrompt = "Role: investigator. Locate and report only — never modify files. Answer " +
		"\"where is X\", \"what calls Y\", \"how does Z work\" with precise file:line references and a " +
		"concise summary. Surface findings; do not propose or apply fixes."
)
