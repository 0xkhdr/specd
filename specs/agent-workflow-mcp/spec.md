# Agent Workflow and MCP Contract Spec

## Purpose
Close gaps between documented specd workflow, CLI command palette, and MCP tool surface so agents receive accurate, executable instructions in every host.

## Source Gaps
- GAP-ANALYSIS.md domain 1: agent-native workflow knowledge.
- Agent guide describes a 5-step loop but current CLI and MCP palette expose only part of it.
- MCP-connected agents receive incomplete or misleading command availability.
- Deferred or unsupported verbs must fail or explain deterministically, never look usable.

## Goals
- Make `AGENTS.md`, `docs/agent-integration.md`, `docs/mcp-guide.md`, MCP tools, and `core.Commands` describe one truth.
- Expose all safe agent workflow primitives through MCP where supported.
- Refuse unsupported MCP operations with structured errors and rationale.
- Add palette digest tests so docs, embedded scaffold, and MCP cannot drift silently.

## Non-Goals
- Do not add LLM logic to gates, DAG, reports, or MCP decision paths.
- Do not make MCP mutate task state without existing CLI-equivalent validation.
- Do not add runtime dependencies.

## Required Knowledge
- Command registry: `internal/core/commands.go`, `internal/cmd/registry.go`.
- MCP server: `internal/mcp/`.
- Embedded scaffold: `internal/core/scaffold.go`, `embed_templates/`.
- Integration tests: `internal/integration/`.
- Docs sync gate: `scripts/docs-lint.sh`.

## Functional Contract
- `core.Commands` remains canonical for command name, usage, phase, flags, and support status.
- MCP command palette is generated from canonical command metadata, not hand-written lists.
- MCP refusal set is explicit. Unsupported verbs return a structured error code and message that names the reason.
- Docs list the same workflow steps and refusal rationale as the binary.
- Scaffolded `AGENTS.md` includes current command names and never references non-existent flags.

## Acceptance Criteria
- `specd mcp` exposes supported workflow tools for status, next/frontier, context, check, approve, verify, task state, decision, memory, help, and version as applicable.
- `docs/mcp-guide.md` names unsupported commands and explains human-in-the-loop or host-safety reason.
- `internal/integration` has a palette digest/conformance test comparing docs, scaffold, and `core.Commands`.
- Unknown verbs still fail closed with exit code 2.
- Deferred verbs print deterministic deferral notice and exit 0 only when declared deferred.

## Invariants
- No command metadata duplicated in MCP by hand.
- No MCP tool bypasses existing CLI validation or evidence gates.
- No scaffold text advertises inert flags.
- Docs and cheatsheet stay byte-synced where existing lint requires it.

## Verification
- `go test ./internal/core ./internal/mcp ./internal/integration -count=1`
- `go test ./... -race -count=1`
- `./scripts/docs-lint.sh`

