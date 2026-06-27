# Spec — Auto-Resume Hook (R5)

**Priority:** P0 · **Wave:** 1 · **Gap:** R5 (no automatic session resumption on host restart).

## Introduction

`brain run` already resumes an active session — but **someone must run it**. After a host
IDE/extension restart there is no auto-discovery of *"I was mid-orchestration on spec X."* The
user must remember the spec and re-run by hand, which breaks autonomous agent loops.

This spec adds **discovery + idempotent resume**: a way to list every resumable session as JSON,
a config contract for hosts to auto-resume on startup, and an MCP tool so agent hosts can wire
it without shelling out. Crashes become transparent: on startup the host lists running sessions
and continues the most recent automatically.

> Note: this is distinct from the existing `brain pause` / `brain resume` **lifecycle** commands
> (`internal/cmd/brain.go` cases `pause`/`resume` → `core.PauseOrchestration` /
> `core.ResumeOrchestration`), which flip session status. This spec adds **discovery** and a
> startup auto-resume contract. To avoid collision the new discovery surface is
> `brain resume --list` plus `brain resume --session <id>` semantics layered on the existing verb.

## Current-state grounding

- Existing resume/pause: `internal/cmd/brain.go` (cases `run`, `step`, `pause`, `resume`),
  `brainSessionControl`, `brainProgramSessionControl`.
- Session store + status: `OrchestrationSession` (`Status`, `UpdatedAt`, `ExpiresAt`) in
  `internal/core/orchestration.go`; status enum `OrchestrationSessionRunning`/`Paused`/… .
- Session enumeration: `ACPRuntimePaths.SessionsDir()` / `SessionDir(id)` /`SessionPath(id)`
  in `internal/core/runtime_paths.go`.
- MCP tools registry: `internal/cmd/mcp.go` (tool definitions + dispatch).
- Config: `OrchestrationCfg` in `internal/core/specfiles.go`; template
  `internal/core/embed_templates/config.json`.

## Requirements

### Requirement 1 — List resumable sessions
**User story:** As a host on startup, I want to discover every session worth resuming, so I can
continue without the user re-specifying the spec.

**Acceptance criteria:**
1. WHEN a host runs `specd brain resume --list --json` THE SYSTEM SHALL print a JSON array of
   `{sessionID, spec, status, updatedAt, pausedSince, lastDecision}` for all sessions under the
   runtime sessions dir.
2. THE SYSTEM SHALL order the array by `updatedAt` descending (most recent first).
3. THE SYSTEM SHALL include only sessions whose status is `running` or `paused`; complete,
   failed, and cancelling sessions are excluded.
4. WHERE `--max-age-minutes <n>` is given THE SYSTEM SHALL exclude sessions whose `updatedAt`
   is older than `n` minutes.
5. THE output array SHALL be empty (`[]`, exit 0) when nothing is resumable.

### Requirement 2 — Idempotent single-session resume
**User story:** As a host, I want a single idempotent command that resumes a known session, so
repeated startup calls are safe.

**Acceptance criteria:**
1. WHEN a host runs `specd brain resume --session <id> --json` THE SYSTEM SHALL reconstruct the
   policy from `session.json` and continue the driver loop for that session (equivalent to
   `brain run --session <id>`).
2. IF the session status is `complete` / `failed` THEN THE SYSTEM SHALL exit non-zero with a
   message and take no action.
3. Calling resume twice on a `running` session SHALL NOT double-dispatch or corrupt state (CAS
   on `state.json` already guards this — the command must rely on it, not bypass it).

### Requirement 3 — Auto-resume config contract
**User story:** As an operator, I want to declare auto-resume policy in config so hosts behave
consistently.

**Acceptance criteria:**
1. THE SYSTEM SHALL accept an `orchestration.resilience.autoResume` block:
   `{ enabled: bool, onHostStart: bool, maxAgeMinutes: int }`.
2. THE SYSTEM SHALL default `enabled=false`; when the block is absent the on-disk config stays
   byte-identical to today.
3. THE SYSTEM SHALL validate `maxAgeMinutes >= 0`; invalid values fail config load with a clear
   message.

### Requirement 4 — MCP `brain_resume` tool
**User story:** As an agent host using MCP, I want a `brain_resume` tool so I can list and resume
without shelling out.

**Acceptance criteria:**
1. THE SYSTEM SHALL register an MCP tool `brain_resume` with input
   `{ session?: string, json?: boolean }`.
2. WHEN `session` is omitted THE SYSTEM SHALL behave as `--list`; WHEN present it SHALL resume
   that session.
3. THE tool description SHALL instruct: *"Call on startup if a running session exists."*

### Requirement 5 — Host startup contract (documented)
**User story:** As a host adapter author, I want a precise startup recipe.

**Acceptance criteria:**
1. THE SYSTEM SHALL document the startup contract: on start run `brain resume --list`; if any
   `running` session is within `maxAgeMinutes`, auto-invoke `brain run --session <id>`; on
   multiple, resume the most-recently-updated (or present a choice).
2. THE documentation SHALL live in AGENTS.md and `docs/agent-integration.md`.

## Design

- Add a `--list` and `--max-age-minutes` flag path to the existing `resume` case in
  `brain.go`. `--list` short-circuits to an enumerator; otherwise current behavior +
  `--session` reconstruction.
- New core `ListResumableSessions(root, maxAge)` reads each `session.json` under
  `SessionsDir()`, filters by status + age, derives `lastDecision` from the session's last
  event/decision record, sorts by `UpdatedAt` desc. Pure read; no writes.
- `lastDecision` source: read the latest decision from the events dir (`EventsDir`) tail, or
  persisted last decision on the session — choose whichever the session already records to avoid
  new write paths.
- MCP tool dispatches to the same core enumerator / resume entry used by the CLI (single source
  of truth).
- Config: extend `OrchestrationCfg` with the shared `Resilience` block (coordinated with
  `checkpoint-protocol` T9 — whichever lands first defines the struct, the other adds fields).

## Out of scope
- Program-level (multi-spec) resume — see `cross-spec-recovery`.
- Actually wiring specific IDEs/extensions — only the contract + MCP tool are in scope.

## Risks
- **Verb overload:** `resume` now means both lifecycle-unpause and discovery. Mitigate by making
  `--list` an explicit, documented mode and keeping the no-flag behavior unchanged.
- **Stale auto-resume:** resuming a long-dead session wastes a step. Mitigated by
  `maxAgeMinutes` filtering.
