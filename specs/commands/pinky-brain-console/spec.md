# Spec — `/pinky-brain` Orchestration Console

**Priority:** P0 · **Wave:** 3 · **Domain:** Brain/Pinky orchestration UX.

## Introduction

Users need `/pinky-brain` as one console to inspect, enable, start, run, step, pause, resume, cancel, compact, and inspect Brain/Pinky orchestration. The command must respect specd’s no-LLM boundary and never forge worker proof. It should use native `specd brain` and `specd pinky` commands for runtime state.

## Current-state grounding

- Native Brain/Pinky command families exist for sessions and workers.
- Orchestration is opt-in through init/config and is POSIX-only for orchestration paths.
- Action plan suggests config edits for enable/disable, but best practice is native command delegation where available; direct config mutation must be atomic and schema-aware if no native command exists.

## Requirements

### Requirement 1 — Capability and status view
**Acceptance criteria:**
1. `/pinky-brain status` SHALL locate `.specd/config.*` and report orchestration enabled/disabled when parseable.
2. It SHALL call `specd brain resume --list --json` when available to show active/resumable sessions.
3. If native Brain commands are unavailable, it SHALL report unsupported with actionable guidance.
4. On native Windows without WSL/POSIX shell, orchestration actions SHALL fail fast with a clear WSL/POSIX message; read-only status may still work.

### Requirement 2 — Enable/disable configuration
**Acceptance criteria:**
1. `/pinky-brain enable` SHALL collect approval policy, workers, retries, timeout, and cost limit.
2. It SHALL prefer native `specd init --orchestration ... --repair` or another supported native config command if available.
3. If direct config write is necessary, it SHALL update only orchestration keys, preserve all unrelated config, write atomically, and keep JSON/YAML format.
4. `/pinky-brain disable` SHALL disable future orchestration without deleting active session files and SHALL warn existing sessions may need cancel.

### Requirement 3 — Session lifecycle actions
**Acceptance criteria:**
1. `/pinky-brain start <slug>` SHALL call `specd brain start` with configured/default policy, workers, retries, and timeout.
2. `/pinky-brain run <slug>` SHALL call `specd brain run` and optionally pass a worker command.
3. `/pinky-brain step <slug> <session>` SHALL call `specd brain step` with configured/default limits.
4. `pause`, `resume`, `cancel`, and `compact` SHALL delegate to native `specd brain` commands and propagate exit codes.

### Requirement 4 — Worker visibility
**Acceptance criteria:**
1. `/pinky-brain workers` SHALL show worker/session visibility from native commands or session files read-only.
2. It SHALL NOT create, alter, or forge Pinky claims/reports.
3. It SHALL recommend native `specd pinky` commands for claim/heartbeat/report flows.

### Requirement 5 — Safety, tests, docs
**Acceptance criteria:**
1. Tests SHALL cover disabled config, enabled config, missing config, native command unavailable, POSIX guard, and session action argv.
2. Direct config writer, if implemented, SHALL have atomic-write tests and format-preservation tests.
3. Docs SHALL explain approval policies, worker limits, timeout units, and proof/evidence boundaries.

## Design

- Add `/pinky-brain` dispatcher to shared workflow wrapper.
- Implement config reader with JSON/YAML awareness matching current repo config direction; use native specd where possible to avoid duplicate schema logic.
- Keep `enable/disable` explicit; never enable orchestration silently.
- Read active sessions via `specd brain resume --list --json`; treat failures as no data plus warning, not as proof no sessions exist.

## Out of scope

- Implementing Brain/Pinky internals.
- Creating worker agents.
- Generating verification evidence.
- Running untrusted `verify:` commands automatically beyond native orchestration behavior.

## Risks

- **Config schema drift:** Prefer native commands; direct writes isolated and tested.
- **Windows confusion:** Clear POSIX/WSL guard for orchestration actions.
- **Fake evidence:** Never call `pinky report` except through explicit user commands outside this wrapper.
