# Spec — Command Schema Guardrails

**Priority:** P0 · **Wave:** 1 · **Domain:** zero-error command discovery.

## Introduction

The fusion analysis requires agents to stop guessing specd syntax. The current command registry is already a single source for help and MCP mirroring, but it does not expose explicit positional arguments, flag enums, phase compatibility, mode compatibility, or `specd help <command> --json`. This leaves hosts parsing usage strings or relying on training memory.

This spec enriches `CommandMeta` into a true machine contract while preserving existing text help and backward-compatible JSON fields.

## Current-state grounding

- `internal/core/commands.go` defines `CommandMeta`, `FlagMeta`, and `ExitCodeMeta`.
- `internal/core/help.go` renders all commands as JSON only through `RenderHelpJSON()`.
- `RenderCommandHelp()` supports text for one command, but no one-command JSON.
- `internal/mcp/tools.go` infers positionals from synopsis/usage and exposes flags as generic string/boolean properties.
- Phase filtering exists for MCP tool exposure (`internal/mcp/tools.go`) but is not part of CLI help schema.

## Requirements

### Requirement 1 — Explicit positionals
**User story:** As a command-building agent, I want named positional args, so I never parse prose usage strings.

**Acceptance criteria:**
1. `CommandMeta` SHALL include `positionals: [{name, required, repeatable, description}]`.
2. Existing JSON keys SHALL remain present and compatible.
3. All commands with positional arguments SHALL populate this field.

### Requirement 2 — Flag enums and constraints
**User story:** As an agent, I want allowed values, so I do not send invalid flags.

**Acceptance criteria:**
1. `FlagMeta` SHALL support optional `enum`, `required`, and `default` fields.
2. Flags with closed values (`mode --set`, `report --format`, `verify --sandbox`, orchestration policies, etc.) SHALL populate `enum`.
3. Boolean flags SHALL remain typed as `boolean`.

### Requirement 3 — Per-command JSON help
**User story:** As an agent, I want to query one command schema before invoking it.

**Acceptance criteria:**
1. `specd help <command> --json` SHALL print exactly one `CommandMeta` object.
2. Unknown commands SHALL exit 2 with the existing error behavior.
3. `specd help --json` SHALL continue printing the full array.

### Requirement 4 — Phase and mode compatibility metadata
**User story:** As an agent, I want the schema to tell me when a command is inappropriate for the current phase or execution mode.

**Acceptance criteria:**
1. `CommandMeta` SHALL include optional `phaseCompatibility` with allowed statuses/phases.
2. `CommandMeta` SHALL include optional `modeCompatibility` with allowed execution modes (`base`, `orchestrated`, `any`) and orchestration-capability requirements.
3. Planning-only, execution-only, verification, complete/reflection, and orchestration commands SHALL be annotated.
4. Metadata SHALL be advisory; existing command enforcement remains in command handlers and gates.

### Requirement 5 — MCP schema consumes enriched metadata
**User story:** As an MCP host, I want tool schemas to reflect the richer command contract.

**Acceptance criteria:**
1. MCP command mirror tools SHALL use `positionals` names in descriptions and schema where possible.
2. Flag enum values SHALL appear in JSON Schema `enum`.
3. Existing MCP golden tests SHALL be updated intentionally.

## Design

- Extend metadata structs in `internal/core/commands.go` with `omitempty` fields to avoid noisy zero values.
- Populate metadata incrementally but completely for the current command set.
- Add `RenderCommandHelpJSON(cmdName)` in `internal/core/help.go`.
- Update `main.go`/help dispatch to route `help <command> --json` separately from `help --json`.
- Update MCP `commandToTool` to include enum values and richer descriptions while preserving `args` array compatibility unless a later spec changes positional input shape.

## Out of scope

- Preventing a user from running commands manually.
- Replacing command handlers' own validation.
- Adding a separate `specd invoke` wrapper.

## Risks

- **Metadata drift:** Mitigate with tests that every usage positional is represented and every enum-like flag documents an enum.
- **Golden churn:** MCP schema changes are intentional; update golden files once.
