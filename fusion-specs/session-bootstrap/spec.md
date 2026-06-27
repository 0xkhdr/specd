# Spec — Fusion Session Bootstrap

**Priority:** P0 · **Wave:** 1 · **Domain:** coding-agent fusion · **Goal:** make the first agent turn specd-aware without guessing.

## Introduction

The fusion analysis identifies the highest-risk failure as session-start drift: agents begin work before loading steering, config, and command discovery. Today specd has the required primitives (`context`, `help --json`, `doctor`, `.specd/steering`, config templates), but no single project-level bootstrap oracle that tells an agent exactly what to read/run before acting.

This spec adds a deterministic **fusion bootstrap** surface. It does not make LLM calls and does not replace existing workflow commands. It packages the existing constitution, config, command schema, and health checks into one machine-readable startup contract so coding agents can initialize correctly every session.

## Current-state grounding

- `internal/cmd/context.go` provides per-spec phase briefing, not project/session bootstrap.
- `internal/core/help.go` exposes `RenderHelpJSON()` for the full command registry.
- `internal/core/specfiles.go` loads `.specd/config.json`, currently with permissive defaults.
- `internal/core/embed_templates/AGENTS.md` instructs agents to load steering but does not offer a command that verifies the load set.
- `docs/agent-integration.md` documents context engineering, MCP, and Brain/Pinky, but lacks a one-command session startup recipe.

## Requirements

### Requirement 1 — Bootstrap command
**User story:** As a coding agent, I want one startup command, so I know which constitution files, config, and command schema to load before processing the user's task.

**Acceptance criteria:**
1. WHEN a host runs `specd fusion bootstrap --json` from a specd project THE SYSTEM SHALL emit a deterministic JSON document with `version`, `root`, `load`, `commands`, `config`, `health`, `modes`, and `nextActions`.
2. THE `load` array SHALL include `.specd/steering/reasoning.md`, `.specd/steering/workflow.md`, `.specd/steering/product.md`, `.specd/steering/tech.md`, `.specd/steering/structure.md`, and `AGENTS.md`, each with mode `read-full` and a rationale.
3. THE `config` block SHALL contain the effective `roles.subagentMode`, `orchestration.enabled`, `orchestration.approvalPolicy`, verify sandbox, gate severities, and a stable digest of `.specd/config.json`.
4. THE `commands` block SHALL identify the command schema source as `specd help --json` and include the current schema digest, not inline megabytes of help by default.
5. THE command SHALL exit `3` with a clear message if no `.specd/` root is found.

### Requirement 2 — Optional full schema inclusion
**User story:** As an MCP or IDE adapter, I want the option to inline the schema once, so I can avoid an extra process call.

**Acceptance criteria:**
1. WHEN `--include-schema` is passed THE SYSTEM SHALL include the full `core.Commands` JSON under `commands.schema`.
2. WHEN omitted THE SYSTEM SHALL include only `schemaCommand`, `digest`, and `count`.
3. Output order SHALL be stable across runs for identical inputs.

### Requirement 3 — Health summary
**User story:** As an agent, I want actionable startup health, so I do not proceed against a broken scaffold.

**Acceptance criteria:**
1. THE `health` block SHALL report whether required steering files, roles, skills, config, and AGENTS markers exist.
2. THE command SHALL be read-only; it SHALL NOT repair files.
3. IF health has failures THE `nextActions` array SHALL recommend `specd doctor --fix` or `specd init --repair` as appropriate.

### Requirement 4 — Active spec summary
**User story:** As an agent resuming a repo, I want to see active specs without loading all artifacts.

**Acceptance criteria:**
1. THE `modes` block SHALL list each spec slug, status, phase, effective execution mode, mode origin, and gate.
2. THE summary SHALL NOT include full markdown artifacts or `state.json` bodies.
3. Sorting SHALL be by slug ascending.

### Requirement 5 — Documentation and template guidance
**User story:** As an agent host implementer, I want exact startup instructions.

**Acceptance criteria:**
1. `internal/core/embed_templates/AGENTS.md` SHALL tell agents to run `specd fusion bootstrap --json` or, if unavailable, manually load the same files and run `specd help --json`.
2. `docs/agent-integration.md` SHALL document the bootstrap JSON contract and fallback sequence.

## Design

- Add `internal/cmd/fusion.go` and register `fusion` in `internal/cmd/registry.go`.
- Add `CommandMeta` for `fusion` in `internal/core/commands.go` with usage `specd fusion bootstrap [--include-schema] [--json]`.
- Implement pure helpers in `internal/core/fusion.go` for file existence, config digest, command schema digest, and active spec summaries.
- Reuse existing root discovery (`FindSpecdRoot`) and existing command metadata (`core.Commands`).
- Keep output small by default; `--include-schema` is explicit.

## Out of scope

- Automatically reading files into a model context.
- Enforcing host-native subagent spawning.
- Replacing `specd context <slug>` for phase-specific loading.

## Risks

- **Command proliferation:** Mitigate by scoping `fusion` to host/agent integration and keeping business logic in core helpers.
- **Schema duplication:** Default to digests and a `schemaCommand`; inline schema only when requested.
