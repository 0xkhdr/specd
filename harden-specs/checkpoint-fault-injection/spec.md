# Spec — Checkpoint Fault-Injection Stress (A3)

**Priority:** P1 · **Wave:** 2 · **Domain:** crash-consistency / idempotency.

## Introduction

The resilience stress jobs (`stress.sh`, `stress-brain-recovery.sh`) cover
contention and retry/reclaim under clean cancellation. They do not cover a host
killed mid-`RecordCheckpoint` — specifically the window between lease-clear and
file-write. The pure-decide design (`DecideOrchestration` pure over
`(snapshot, policy)`; clock/state via `Sense*`) should make this crash-safe, but
that safety is currently argued, not proven under `SIGKILL`.

This spec adds a fault-injection stress variant that kills the host at the
critical checkpoint window and asserts no double-claim and no orphaned lease.

## Current-state grounding

- `scripts/stress.sh`, `scripts/stress-brain-recovery.sh` — existing stress jobs;
  clean cancel only.
- Resilience checkpoint/lease logic: CAS lease ops, suspend cap, idempotent
  resume; `DecideOrchestration` pure; state sensed.
- `RecordCheckpoint` performs lease-clear then file-write (the injection window).
- CI runs the two stress scripts.

## Requirements

### Requirement 1 — Crash injection at the checkpoint window
**User story:** As a maintainer, I want the host killed mid-checkpoint, so crash
behavior is exercised not assumed.

**Acceptance criteria:**
1. A stress variant SHALL `SIGKILL` (not clean cancel) the host between
   lease-clear and file-write during `RecordCheckpoint`.
2. The kill point SHALL be reproducible (injection hook/env-gated fault, fixed
   seed) so failures are debuggable.

### Requirement 2 — No double-claim after crash-resume
**User story:** As an operator, I want at most one owner of a checkpoint after a
crash, so work is not duplicated.

**Acceptance criteria:**
1. After SIGKILL + resume, no two runners SHALL hold/claim the same checkpoint.
2. The assertion SHALL run over many randomized kill timings (loop/seeded).

### Requirement 3 — No orphaned lease after crash
**User story:** As an operator, I want a crash to never strand a lease, so resume
is not blocked forever.

**Acceptance criteria:**
1. After SIGKILL, resume SHALL either reclaim or expire the lease per policy — no
   permanently-stuck lease.
2. `MaxSteps`/suspend caps SHALL still bound the post-crash run.

## Design

- Add an env-gated fault hook (e.g. `SPECD_FAULT_CHECKPOINT=after-lease-clear`)
  honored only in test/stress builds, that `os.Exit`/SIGKILLs at the window.
- New script `scripts/stress-checkpoint-fault.sh`: loop launching a worker, kill
  at the injection point across seeds, then resume and assert invariants by
  inspecting on-disk lease/checkpoint state.
- Wire into CI as a third stress job mirroring the existing two.

## Out of scope

- Fault injection for non-checkpoint write paths (separate effort).
- Filesystem-level torn-write simulation (assumes atomic rename already used).

## Risks

- **Non-determinism:** seed the kill timing; record the seed on failure.
- **CI flakiness:** keep iteration count bounded; fail fast with the seed printed.
