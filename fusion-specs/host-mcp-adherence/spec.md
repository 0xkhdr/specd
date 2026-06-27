# Spec — Host and MCP Adherence Protocol

**Priority:** P1 · **Wave:** 3 · **Domain:** coding-agent host integration.

## Introduction

The fusion analysis requires hosts to follow the same adherence rules whether they drive specd through shell commands, MCP raw tools, or MCP intent tools. The current MCP layer already exposes raw command mirrors, intent tools (`brain_orchestrate`, `brain_status`, etc.), essential/phase exposure, and server instructions. This spec tightens the host protocol around bootstrap, policy checks, subagent delegation, command schema discovery, and Brain/Pinky execution.

## Current-state grounding

- `internal/mcp/server.go` provides server instructions and dispatches tools through existing command handlers.
- `internal/mcp/tools.go` supports `mcp.expose`, `essentialTools`, `includeMeta`, `includeOrchestration`, phase tools, and role filtering.
- `internal/mcp/intent.go` provides intent-level Brain tools.
- `internal/core/embed_templates/AGENTS.md` describes roles, subagentMode, Base vs Orchestrated, and Brain/Pinky basics.
- `docs/agent-integration.md` documents host adapter pattern, MCP, context engineering, and orchestration.

## Requirements

### Requirement 1 — MCP startup instructions include fusion bootstrap
**User story:** As an MCP host, I want server instructions to tell the model exactly how to start a specd session.

**Acceptance criteria:**
1. MCP initialize/server instructions SHALL instruct hosts to call `specd_fusion` or the raw fusion command equivalent when available.
2. Instructions SHALL state fallback: call `specd_status`, `specd_context`, and `specd_help --json` / command schema before acting.
3. Instructions SHALL remain concise enough for MCP initialization payloads.

### Requirement 2 — Fusion tool exposure
**User story:** As an MCP host, I want bootstrap and policy as tools, not only shell commands.

**Acceptance criteria:**
1. The raw `fusion` command SHALL be exposed as `specd_fusion` when non-meta tools are listed.
2. In `mcp.expose="essential"`, bootstrap/policy availability SHALL be considered for inclusion or documented fallback.
3. The tool SHALL be read-only annotated.

### Requirement 3 — Delegate-mode host protocol
**User story:** As a host with subagents, I need exact behavior when `roles.subagentMode=delegate`.

**Acceptance criteria:**
1. Docs and AGENTS template SHALL state that delegate mode requires spawning role-bound subagents for implementation work when the host supports it.
2. For hosts without subagent capability, the protocol SHALL require an explicit inline fallback warning in the agent response.
3. Brain/Pinky missions SHALL remain the preferred path for orchestrated specs; base delegate mode SHALL use `specd dispatch --json` packets.

### Requirement 4 — Orchestration adherence playbook
**User story:** As a host driving Brain/Pinky, I want a state-machine playbook for every Brain decision.

**Acceptance criteria:**
1. Docs SHALL list required handling for `dispatch`, `wait`, `awaiting-approval`, `escalate`, `policy-violation`, and `complete-session` decisions.
2. Pinky lifecycle SHALL be documented as `claim -> heartbeat/progress -> verify -> report/block -> release`.
3. Verification references SHALL be required for terminal reports; host telemetry SHALL be documented as metadata only.

### Requirement 5 — Phase-compatible MCP exposure tests
**User story:** As a maintainer, I want tests proving the tool surface does not invite phase-incompatible actions.

**Acceptance criteria:**
1. Tests SHALL assert planning phase exposure excludes `next`, `verify`, `task`, `brain`, and `pinky` unless explicitly allowed by orchestration policy.
2. Tests SHALL assert executing exposure includes `next`, `dispatch`, `verify`, and `task`.
3. Tests SHALL assert orchestration tools disappear when `includeOrchestration=false` and appear when enabled.

## Design

- Update MCP server instructions in `internal/mcp/server.go` to mention fusion startup compactly.
- Let raw command mirroring expose `fusion` once the fusion command exists; mark it read-only in `internal/mcp/tools.go`.
- Revisit default essential tools after fusion lands; prefer `specd_fusion` if it reduces total startup calls without bloating day-to-day tool lists.
- Expand docs and templates rather than adding enforcement that host runtimes cannot satisfy.
- Strengthen tests around phase exposure and orchestration gating.

## Out of scope

- Implementing host-native subagent APIs.
- Forcing a model to obey MCP instructions.
- Networked orchestration transports beyond the existing file-backed ACP.

## Risks

- **Instruction bloat:** Keep server instruction additions short and push detail into docs/AGENTS.
- **Host variability:** Delegate mode docs must define fallback behavior for hosts without subagents.
