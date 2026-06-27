# Spec — Cross-Spec Program Recovery

**Priority:** P2 · **Wave:** 3 · **Gap:** Program-level orchestration resilience (extends R5).

## Introduction

A program-level run (`brain run --program`) orchestrates a DAG of child specs. The parent
creates a session and each child spec gets its own session. If the program is interrupted, the
existing resume continues a *single* spec session — there is no way to resume the **whole
program frontier**: which children were active, which had running workers, which were complete.

This spec persists explicit program-level recovery state and adds `brain resume --program` to
reconstruct the program DAG and restart the driver loop from the current frontier.

## Current-state grounding

- Program driver: `internal/core/orchestration_driver.go` (`DriveProgramOrchestration`,
  `ProgramDriverOpts.MaxSteps`/`MaxWaits`); program command surface in
  `internal/cmd/program.go` and `internal/cmd/brain.go` program branches
  (`PauseProgramOrchestration` / `ResumeProgramOrchestration`).
- DAG: `internal/core/dag.go`.
- Session/state persistence: `session.json` + `state.json` per spec (CAS). Program currently
  lacks a single authoritative `inflightKeys` + `childSessions` file.
- Depends on `auto-resume` (single-session discovery + idempotent resume) as its building block.

## Requirements

### Requirement 1 — Persist program-state file
**User story:** As the program driver, I want the program frontier on disk, so recovery does not
have to re-derive it from scattered child sessions.

**Acceptance criteria:**
1. THE SYSTEM SHALL persist a `program-state.json` under the parent session dir capturing:
   parent `SessionID`, `childSessions` (spec → sessionID), `inflightKeys`, each child's
   `status`, and `updatedAt`.
2. THE SYSTEM SHALL write `program-state.json` via CAS on every program driver step (consistent
   with `state.json` discipline) so a crash leaves a coherent latest frontier.
3. THE file SHALL be canonical JSON and validated on read; a corrupt file fails closed with a
   clear error rather than a partial resume.

### Requirement 2 — `brain resume --program --session <parent-id>`
**User story:** As a host, I want one command to resume an entire interrupted program.

**Acceptance criteria:**
1. WHEN a host runs `specd brain resume --program --session <parent-id>` THE SYSTEM SHALL read
   `program-state.json`, classify children into complete / running / pending, and restart the
   program driver from the current frontier.
2. THE SYSTEM SHALL NOT re-dispatch children already `complete`.
3. FOR children with a running worker, THE SYSTEM SHALL rely on lease/checkpoint recovery (from
   the P0/P1 specs) rather than restarting them from zero.
4. THE resume SHALL be idempotent: a second call does not double-advance the frontier (CAS-guarded).

### Requirement 3 — Discovery integration
**User story:** As a host, I want program sessions to show up in resume discovery.

**Acceptance criteria:**
1. THE `brain resume --list` output (from `auto-resume`) SHALL mark program-parent sessions with
   a `program: true` flag and include child counts (`complete/total`).
2. WHEN auto-resume selects a program-parent session THE SYSTEM SHALL resume it via the
   `--program` path, not the single-spec path.

## Design

- Define `ProgramState` in core (beside the orchestration models); canonical-JSON + validator;
  CAS write helper analogous to `SaveState`.
- `DriveProgramOrchestration` writes `ProgramState` each step (add the write at the existing
  per-step commit point). Path helper `ProgramStatePath(parentSessionID)`.
- `brain resume --program` reads `ProgramState`, rebuilds the child frontier using the existing
  DAG logic in `dag.go`, and calls `DriveProgramOrchestration` from that frontier.
- Discovery: `ListResumableSessions` (from `auto-resume`) detects a `program-state.json` in the
  session dir to set `program: true` and child counts.

## Coordination
- **Depends on `auto-resume`** for the discovery list and idempotent single-session resume it
  reuses for children.
- Benefits from `checkpoint-protocol` + `rate-limit-lease` for running-child recovery (Req 2.3);
  functions without them but recovers running children less gracefully.

## Out of scope
- Changing program DAG semantics or scheduling fairness — recovery only.

## Risks
- **Frontier skew:** `program-state.json` lagging child `state.json` could mis-classify a child.
  Mitigated by CAS-per-step writes and re-deriving child status from the child session on resume
  (program-state is a hint, child session is authoritative).
