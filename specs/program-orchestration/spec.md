# Spec: Multi-Spec Program Orchestration

Status: proposed — awaiting human review
Scope: deterministic scheduling across the existing `program.json` DAG.

## 1. Outcome

Extend the current program graph, wave, frontier, cycle, orphan, and critical-path logic with bounded orchestration sessions. Do not create a second parser or program state machine.

## 2. Requirements

- R6.1 Reuse `core.LoadProgram`, `BuildProgram`, `RunnableFrontier`, `DetectCycle`, and `CriticalPath`.
- R6.2 Add a pure program decision function that selects runnable specs in stable wave/slug order.
- R6.3 Enforce `maxConcurrentSpecs` and each child spec's worker limits.
- R6.4 Acquire a child orchestration lease before starting a Spec Brain; skip or escalate on conflicting ownership.
- R6.5 Recompute the frontier after every child terminal event or authoritative state revision.
- R6.6 Fail closed on cycles, orphan dependencies, corrupt state, incompatible session policy, or child escalation.
- R6.7 Support configurable failure policy: `fail-fast` in v1; `continue-independent` is reserved for a later version.
- R6.8 Add `specd brain start --program`, program-aware status, pause, resume, and cancel.
- R6.9 Keep child sessions independently replayable while linking them to one parent program session.
- R6.10 Produce deterministic progress counts by wave/spec status and expose the critical path.
- R6.11 Never mark program completion from ACP claims; derive it from authoritative complete spec states.
- R6.12 Preserve explicit approval policy per child spec; program mode cannot widen it.

## 3. Scheduling

```text
load graph -> validate -> sense child leases/states -> stable frontier
-> start up to capacity -> wait for event/revision -> recompute
-> complete only when every spec state is complete
```

No recursive goroutine tree is required. One scheduler loop owns the parent session and advances child Brain sessions through bounded `step` operations.

## 4. Invariants

- V1 Existing `specd program` output remains the graph source of truth.
- V2 A spec starts only after all declared dependencies are complete.
- V3 At most one parent session owns a child spec lease.
- V4 Stable input state produces stable dispatch order and JSON output.
- V5 Parent pause/cancel prevents new child dispatch and propagates cooperative directives.
- V6 A blocked child cannot be mistaken for a completed dependency.

## 5. Acceptance

- Tests cover parallel frontier scheduling, capacity, cycles, orphans, child conflicts, pause/resume, fail-fast escalation, restart recovery, and all-complete termination.
- Cross-process stress proves child leases prevent duplicate orchestration.
- `make ci` passes.
