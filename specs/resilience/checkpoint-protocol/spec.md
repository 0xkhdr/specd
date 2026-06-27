# Spec — Checkpoint Protocol (R1 + R4)

**Priority:** P0 · **Wave:** 1 · **Gaps:** R1 (no proactive token-limit checkpointing),
R4 (no worker hibernate/thaw protocol).

## Introduction

A Pinky worker is ephemeral: it claims a lease, works, verifies, reports, releases. There
is no way for a worker to say *"I am 70% done, my context window is full, save my progress
and let a fresh worker continue from here."* When the host hits its context limit mid-task,
the lease expires and the task is retried **from scratch** by a worker with no memory of the
partial implementation — wasting all consumed tokens and progress.

This spec adds a **checkpoint** primitive: a worker (or the host, on behalf of all workers)
serializes partial progress to disk and releases its lease cleanly. The Brain detects the
checkpoint on its next sense step and emits a new `resume-from-checkpoint` decision that
dispatches a fresh worker primed with the prior worker's progress, working notes, changed
files, and context manifest — *continue, do not restart*.

## Current-state grounding

- Orchestration model, actions, and validators: `internal/core/orchestration.go`
  (`OrchestrationAction` consts, `OrchestrationDecision`, `validOrchestrationAction`).
- Mission assembly: `internal/core/pinky.go` (`BuildMissionContextManifest`, mission
  struct with `ContextManifest contextpkg.MissionContextManifest`) and
  `internal/core/pinky_brief.go` (`renderContextManifest`).
- Sense/decide split: `SenseOrchestration` builds the snapshot; `DecideOrchestration`
  is a pure function of `(snapshot, policy)`. New state must enter via Sense.
- Lease release: `ACPStore.ClaimLease` / release in `internal/core/acp_lease.go`;
  worker subcommands in `internal/cmd/pinky.go` (`claim|brief|heartbeat|progress|
  block|query|inbox|report|release`).
- Runtime paths: `internal/core/runtime_paths.go` (`ACPRuntimePaths.SessionDir`,
  `WorkerDir`, `EventsDir`). Add a `CheckpointsDir(sessionID)` helper.
- Terminal report shape to mirror: `PinkyTerminalReport` in
  `internal/core/pinky_report.go` (already carries `ChangedFiles`, `GitHead`, `Summary`).

## Requirements

### Requirement 1 — CheckpointRecord data model
**User story:** As the Brain, I want a persisted record of a worker's partial progress, so
that a future worker can resume from the exact point of interruption.

**Acceptance criteria:**
1. THE SYSTEM SHALL define `CheckpointRecord` in `internal/core/orchestration.go` with
   fields: `CheckpointID`, `SessionID`, `WorkerID`, `Spec`, `TaskID`, `Attempt`,
   `CreatedAt`, `ProgressPercent`, `Summary`, `ChangedFiles`, `GitHead`,
   `ContextManifest` (omitempty), `WorkingNotes` (omitempty), `PartialVerify` (pointer,
   omitempty).
2. THE SYSTEM SHALL provide `ValidateCheckpointRecord` enforcing: valid session/worker IDs
   (reuse `validateACPOpaqueID` / `validateACPRuntimeSegment`), `acpTaskIDRE` task ID,
   `Attempt >= 1`, `ProgressPercent` in `[0,100]`, parseable `CreatedAt`.
3. THE SYSTEM SHALL serialize checkpoints via the existing canonical-JSON path so byte
   output is deterministic and stable across runs.

### Requirement 2 — `specd pinky checkpoint` command
**User story:** As a worker sensing token pressure, I want one command to save my progress
and release my lease, so that no work is lost when my context fills.

**Acceptance criteria:**
1. WHEN a worker runs `specd pinky checkpoint --session <id> --worker <id> --spec <slug>
   --task <id> --attempt <n> --progress <0-100> --summary "..."` with optional
   `--changed-files`, `--working-notes`, `--git-head`, THE SYSTEM SHALL persist a
   `CheckpointRecord` to `.specd/runtime/sessions/<session>/checkpoints/<task>-<attempt>.json`.
2. WHEN the checkpoint is persisted THE SYSTEM SHALL release the caller's active lease for
   that task (same effect as `pinky release`) and emit a `checkpoint` ACP event.
3. IF the worker holds no active lease for `<task>/<attempt>` THEN THE SYSTEM SHALL fail with
   a non-zero exit and an explanatory message, persisting nothing.
4. WHERE `--json` is passed THE SYSTEM SHALL print the persisted record as canonical JSON.

### Requirement 3 — `specd brain checkpoint` (force-all)
**User story:** As the host nearing its own context limit, I want to checkpoint every active
worker at once, so that I can `/clear` safely.

**Acceptance criteria:**
1. WHEN the host runs `specd brain checkpoint --session <id> --reason "host-context-limit"`
   THE SYSTEM SHALL request a checkpoint for every active lease in the session.
2. THE SYSTEM SHALL record the `--reason` on each emitted checkpoint event.
3. IF no workers are active THEN THE SYSTEM SHALL exit 0 and report that nothing was
   checkpointed.

### Requirement 4 — `resume-from-checkpoint` decision
**User story:** As the Brain, I want to prefer resuming a checkpointed task over dispatching
fresh, so that progress is continued rather than discarded.

**Acceptance criteria:**
1. THE SYSTEM SHALL add `OrchestrationResume OrchestrationAction = "resume-from-checkpoint"`
   and register it in `validOrchestrationAction`.
2. WHILE a `CheckpointRecord` exists for a runnable task that has no active lease THE SYSTEM
   SHALL emit `resume-from-checkpoint` for that task instead of `dispatch`.
3. THE SYSTEM SHALL surface the checkpoint into the snapshot inside `SenseOrchestration`
   (not by reading disk inside `DecideOrchestration`), keeping `DecideOrchestration` a pure
   function of `(snapshot, policy)`.
4. THE SYSTEM SHALL validate the new action through `ValidateOrchestrationDecision` with no
   `Artifact` requirement (it is an execution decision, not an authoring one).

### Requirement 5 — Resume mission brief
**User story:** As a resuming worker, I want the prior worker's progress in my mission brief,
so that I continue from 70% rather than rebuild from 0%.

**Acceptance criteria:**
1. WHEN the decision is `resume-from-checkpoint` THE SYSTEM SHALL include the checkpoint's
   `ContextManifest`, `WorkingNotes`, `ChangedFiles`, `GitHead`, and `ProgressPercent` in the
   mission brief produced by `BuildPinkyMission` / `renderContextManifest`.
2. THE SYSTEM SHALL prefix the brief with an explicit resume instruction: *"You are resuming
   task <T> from a checkpoint at <P>%. Continue from here; do not restart. Working notes: …"*
3. WHERE no checkpoint applies THE SYSTEM SHALL render the brief exactly as today (byte-stable
   for the non-resume path).

### Requirement 6 — Lifecycle & cleanup
**User story:** As an operator, I want stale checkpoints not to resurrect completed work.

**Acceptance criteria:**
1. WHEN a task reaches `complete` (verified) THE SYSTEM SHALL delete or archive its
   checkpoint so it is not re-resumed.
2. IF a checkpoint's `Attempt` is older than the task's current attempt THEN THE SYSTEM SHALL
   ignore it when sensing.
3. THE SYSTEM SHALL gate the whole feature behind `orchestration.resilience.checkpointEnabled`
   (default `false`); when disabled, behavior is byte-identical to today.

## Design

### Data & storage
- `CheckpointRecord` lives in `orchestration.go` beside `OrchestrationSession`. Add it to the
  `normalizeOrchestrationValue` switch for canonical JSON + validation.
- New path helper `ACPRuntimePaths.CheckpointDir(sessionID)` →
  `sessions/<id>/checkpoints`, and `CheckpointPath(sessionID, task, attempt)` →
  `checkpoints/<task>-<attempt>.json`. Reuse `sessionJoin`.
- A new core function `RecordCheckpoint(root, rec, cfg)` mirrors `RecordPinkyProgress`:
  validate → write canonically → release lease → append ACP `checkpoint` event.

### Sense/decide
- `SenseOrchestration` scans the checkpoints dir, attaches the newest valid record per task
  to the snapshot via a new `Checkpoints []OrchestrationCheckpointRef` field on
  `OrchestrationSnapshot` (omitempty → byte-stable when empty).
- `DecideOrchestration` precedence: a runnable task with a matching checkpoint ref and no
  active lease → `resume-from-checkpoint`; else existing `dispatch` logic. Place the check
  immediately before the fresh-dispatch branch.

### Mission
- `BuildPinkyMission` accepts the checkpoint ref (or reads it from the decision's `TaskID`)
  and threads notes/manifest into the mission. `renderContextManifest` gains a resume header
  block, emitted only when resume data is present.

### Host integration (reference only — documented, not enforced here)
- Host worker loop calls `pinky checkpoint` before `/clear` or on a token warning, then exits
  gracefully. Documented in AGENTS.md (Wave-4 of source plan).

## Out of scope
- Rate-limit suspend/resume (see `rate-limit-lease`).
- Snapshot delta optimization of context (see `context-snapshot`); this spec stores the full
  manifest inline, which `context-snapshot` later optimizes.

## Risks
- **Resurrection bug:** a checkpoint that outlives task completion re-dispatches finished
  work. Mitigated by Requirement 6 (attempt-guard + cleanup).
- **Determinism leak:** reading checkpoint files inside `DecideOrchestration` would break
  purity. Mitigated by sensing in `SenseOrchestration` only.
