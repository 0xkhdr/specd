# Spec — Configuration and Mode Sentinel

**Priority:** P0 · **Wave:** 2 · **Domain:** binding configuration adherence.

## Introduction

The fusion analysis names configuration disrespect as a core cause of agent drift. Agents must honor `roles.subagentMode`, `orchestration.enabled`, per-spec `executionMode`, gate severities, and verify sandbox settings. The current code loads config defaults robustly, exposes mode commands, and gates Brain/Pinky by spec mode, but malformed config can silently fall back to defaults and there is no compact policy document for agents to consult.

This spec adds a read-only sentinel that validates and summarizes the binding configuration and mode policy before major operations.

## Current-state grounding

- `internal/core/specfiles.go` defines `Config`, `LoadConfig`, defaults, orchestration, gates, roles, verify sandbox, and MCP settings.
- `LoadConfig` returns defaults on missing or invalid config, which is convenient but hides malformed config from agents.
- `internal/cmd/mode.go` exposes per-spec execution mode and recommendation.
- Brain/Pinky commands enforce orchestrated spec mode and orchestration capability in command/core paths.
- `docs/agent-integration.md` explains capability vs selection, but agents still need a machine-readable sentinel.

## Requirements

### Requirement 1 — Strict config validation path
**User story:** As an agent, I want to know when config is malformed instead of silently using defaults.

**Acceptance criteria:**
1. THE SYSTEM SHALL add a strict config loader returning validation diagnostics while preserving existing `LoadConfig` behavior for backward compatibility.
2. Invalid JSON, invalid enum values, and out-of-range integers SHALL be reported with clear field paths.
3. Missing config SHALL be reported as defaulted, not invalid.

### Requirement 2 — Policy summary command
**User story:** As an agent, I want one policy summary before choosing inline/base/orchestrated behavior.

**Acceptance criteria:**
1. `specd fusion policy [<slug>] --json` SHALL emit effective config policy and, when slug is supplied, the spec's effective execution mode.
2. The output SHALL include `subagentMode`, `orchestrationEnabled`, `approvalPolicy`, `workerMode`, `maxWorkers`, `maxRetries`, `timeoutSeconds`, `verifySandbox`, `gateSeverities`, `mcpExposure`, and `configDigest`.
3. With a slug, output SHALL include `specMode`, `modeOrigin`, `brainAllowed`, `baseLoopAllowed`, and recommended next command family.

### Requirement 3 — Configuration drift detection
**User story:** As an agent, I want to know if config changed after bootstrap.

**Acceptance criteria:**
1. `specd fusion policy --expect-config-digest <sha256> --json` SHALL compare the current digest to the expected digest.
2. On mismatch THE SYSTEM SHALL exit 1 and recommend rerunning `specd fusion bootstrap --json`.
3. On match THE SYSTEM SHALL exit 0.

### Requirement 4 — Mode-mixing diagnostics
**User story:** As an agent, I want explicit warnings before mixing Base and Orchestrated loops.

**Acceptance criteria:**
1. For Base specs, policy output SHALL set `brainAllowed=false` and recommend `context/next/verify/task`.
2. For Orchestrated specs with project orchestration enabled, policy output SHALL set `baseLoopAllowed=false` and recommend `brain run` or MCP `brain_orchestrate`.
3. For Orchestrated specs without project capability, policy output SHALL report a policy violation and recommend `specd mode <slug> --set base` or enabling orchestration.

### Requirement 5 — Doctor integration
**User story:** As a user, I want `doctor` to catch policy-breaking config.

**Acceptance criteria:**
1. `specd doctor --json` SHALL include strict config diagnostics.
2. `specd doctor` text mode SHALL show a concise config policy check.
3. `--fix` SHALL NOT rewrite invalid custom values except existing safe scaffold repair behavior.

## Design

- Add `LoadConfigStrict(root) (Config, []Diagnostic)` to `internal/core/specfiles.go` or a new `config_validate.go`.
- Keep `LoadConfig` unchanged for call sites that intentionally default.
- Add `fusion policy` as a second subcommand in `internal/cmd/fusion.go` sharing the bootstrap policy summary structs.
- Reuse `state.EffectiveMode()` and mode origin helpers for spec mode.
- Add config digest calculation once in core fusion helpers.
- Extend doctor checks with read-only strict config validation.

## Out of scope

- Automatic migration of arbitrary invalid config.
- Changing default `roles.subagentMode` or `orchestration.enabled`.
- Forcing orchestration when project capability is true; users still opt in per spec.

## Risks

- **Backward compatibility:** Keep permissive `LoadConfig`; strict validation is opt-in via fusion/doctor.
- **Policy overreach:** The sentinel reports and recommends; command handlers remain the enforcers.
