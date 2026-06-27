# Spec — Progress-Weighted MaxWaits (R6)

**Priority:** P2 · **Wave:** 3 · **Gap:** R6 (`MaxSteps`/`MaxWaits` stall without telemetry).

## Introduction

`DriveOrchestration` bounds consecutive non-progress waits with `MaxWaits` (default 8). A worker
that is genuinely progressing but slow — a long compile, a big test suite — produces no Brain
state change for several waits and the driver exits `DriverStalled` even though work is
happening. The host must detect this and restart, wasting effort.

This spec makes waits **progress-aware**: if any in-flight worker has reported progress within a
configurable window, that wait does not count toward the stall limit. Long-running honest work no
longer trips the stall guard.

## Current-state grounding

- Driver loop + outcomes: `internal/core/orchestration_driver.go` (`DriverOpts.MaxSteps`/
  `MaxWaits`, `DriverStalled`, the `waits++` / `waits=0` logic around lines 148–233).
- Progress reports: `PinkyProgressReport` in `internal/core/pinky_report.go` (currently has
  `Percent`, `Message` but **no timestamp**).
- Progress command: `internal/cmd/pinky.go` case `progress` → `RecordPinkyProgress`.

## Requirements

### Requirement 1 — Progress carries a timestamp
**User story:** As the driver, I want to know when a worker last reported, so I can tell slow
progress from a stall.

**Acceptance criteria:**
1. THE SYSTEM SHALL record a `LastReport` timestamp on progress (on the persisted progress
   record / worker cursor, not necessarily the in-memory `PinkyProgressReport`).
2. THE SYSTEM SHALL set `LastReport` to the server-side write time so it cannot be spoofed past
   into the future by the worker.
3. THE existing progress event/output SHALL remain byte-stable except for the added field
   (omitempty).

### Requirement 2 — Progress-weighted wait accounting
**User story:** As an operator running long tasks, I want the driver not to give up while work
is visibly progressing.

**Acceptance criteria:**
1. WHILE at least one in-flight worker has a `LastReport` within
   `orchestration.resilience.progressTimeoutSeconds` (default 300) THE SYSTEM SHALL NOT increment
   the consecutive-wait counter for that step.
2. IF no in-flight worker has reported within the window THEN THE SYSTEM SHALL increment waits as
   today.
3. THE SYSTEM SHALL still honor `MaxSteps` as a hard ceiling regardless of progress (no infinite
   loop).
4. THE SYSTEM SHALL keep the wait decision derivable from sensed state (progress timestamps enter
   via the snapshot, not via ad-hoc clock reads inside the pure decision).

### Requirement 3 — Config
**Acceptance criteria:**
1. THE SYSTEM SHALL add `orchestration.resilience.progressTimeoutSeconds` (default 300) to the
   shared `Resilience` block; absent → byte-identical config.
2. THE SYSTEM SHALL validate `0 < progressTimeoutSeconds <= 3600`.
3. WHERE the value is 0/unset THE SYSTEM SHALL behave exactly as today (no weighting).

## Design

- Add `LastReport` (RFC3339, omitempty) to the persisted progress/worker-cursor record updated by
  `RecordPinkyProgress`, stamped server-side.
- Surface the most-recent in-flight `LastReport` into `OrchestrationSnapshot` during
  `SenseOrchestration` (e.g., `MostRecentProgressAt` omitempty), so the driver's wait logic reads
  sensed state.
- In `orchestration_driver.go`, before `waits++`, compute `waitWeight`: if
  `now - MostRecentProgressAt < progressTimeoutSeconds`, weight 0 (skip increment); else 1. Reset
  `waits=0` on real progress as today. `MaxSteps` unchanged.

## Out of scope
- Changing `MaxSteps` semantics or default; only the wait/stall counter is progress-weighted.

## Risks
- **Stuck-but-chatty worker:** a worker that emits progress without advancing could defer a stall
  indefinitely. Bounded by the hard `MaxSteps` ceiling (Req 2.3).
