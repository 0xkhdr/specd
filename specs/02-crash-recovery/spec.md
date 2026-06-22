# Spec 02 — Driver-Level Crash Recovery

> Wave: **W1 (P0)** · Priority: **P0** · Source: LEVEL_UP_PLAN §1.5, §2 P0.3
> Depends on: **Spec 01** (`Runner` seam enables fault injection)

## 1. Problem

Orchestration sessions persist via `Save/LoadOrchestrationSession` and
`internal/core/program_orchestration.go` has a fully-developed lease subsystem
(`AcquireProgramChildLease`, `ReleaseProgramChildLease`,
`markProgramChildLeaseEscalated`, `LoadProgramChildLeases`, lease-active checks,
per-slug file locks). The **ACP lease layer is stress-tested**.

But there is **no test that the `cmd`-layer driver survives its own death**:

> brain process dies mid-wave → resume must reclaim in-flight leases and must
> **not** double-dispatch a task that a now-dead worker already started.

This is the exact failure an autonomous fleet hits: an operator's box reboots,
a session is OOM-killed, CI cancels a job. Today that path is unverified. A
double-dispatch in production means two workers editing the same spec/task —
the worst class of orchestration bug.

## 2. Root cause

Recovery correctness lives at the seam between the persisted session/lease state
(`core`) and the live process driver (`cmd/brain.go`). The pieces exist on both
sides but nothing exercises the **join**: kill the driver, reload state, and
prove the next `step`/`run` reconciles leases instead of re-dispatching.

## 3. Solution

A deterministic, driver-level recovery test plus a CI stress job.

### 3.1 Recovery invariants to assert

1. **Lease reclaim:** after a mid-wave kill, a stale/expired in-flight lease is
   reclaimed on resume (released or re-leased to a fresh worker), never left
   dangling as permanently-active.
2. **No double-dispatch:** a task whose lease was held at kill time is **not**
   dispatched a second time within the same logical attempt. Use the `Runner`
   seam (Spec 01) with a recording fake runner to assert each `TaskID` is
   dispatched at most once across the kill+resume boundary.
3. **Idempotent resume:** running resume twice produces the same reconciled
   state (no compounding side effects).

### 3.2 Test mechanism

- Use a fake `worker.Runner` that, on first dispatch of a target task, records
  the mission then **simulates death**: persist the lease as in-flight, then
  abort the drive (return a sentinel that stops the loop without releasing).
- Reload the session from disk (fresh driver instance) and run `brain step` /
  `brain run` to completion.
- Assert against the recording runner: the killed task appears exactly once (or
  is correctly re-attempted under the documented retry policy — pin which), and
  every other task completes normally.
- Drive entirely through temp `.specd` roots; deterministic, no real `sh`.

### 3.3 CI stress job

Add `make stress-brain-recovery` mirroring the existing stress jobs
(`scripts/stress-orchestration.sh`, `stress-program.sh`, `stress-acp.sh`):
loop the recovery scenario N times under `-race`, killing at randomized
wave points, asserting the no-double-dispatch invariant every iteration. Wire
into `.github/` CI alongside the four existing stress jobs.

## 4. Acceptance criteria

- [ ] A `cmd`-layer test kills a `brain run` mid-wave and asserts lease reclaim.
- [ ] A test asserts **no task is double-dispatched** across kill+resume.
- [ ] Resume is shown idempotent (run twice → same state).
- [ ] `make stress-brain-recovery` exists, runs under `-race`, and is wired into
      CI next to the existing stress jobs.
- [ ] No flakiness across `-count=2`; job is deterministic (seeded randomness).

## 5. Non-goals

- Changing the lease format or persistence layer (already stress-tested).
- New resume *UX* command surface — that is **Spec 08** (P3). This spec proves
  the *mechanism* is correct; Spec 08 promotes it to a documented command.

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Test flakiness from real timing | Inject the kill point deterministically via the fake runner, not wall-clock sleeps |
| Hidden double-dispatch only under contention | Stress job loops with seeded random kill points + `-race` |
| Coupling test to internals | Assert through observable lease/session state + recording runner, not private fields |
