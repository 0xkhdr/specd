# Spec 02 — Tasks (Crash Recovery)

> Prereq: **Spec 01 done** (fake `worker.Runner` available for injection).

---

## Wave A — Recovery test harness

### [ ] W1.8a — Build the kill-injection fake runner
- **Files:** `internal/cmd/brain_recovery_test.go` (new)
- **Do:** Implement a `recordingRunner` (satisfies `worker.Runner`) that:
  records every `Mission` it receives (by `TaskID`); on a configured target
  `TaskID`, persists the in-flight lease then returns an `errSimulatedKill`
  sentinel that unwinds the drive loop **without** releasing the lease (models
  process death holding a lease).
- **Done when:** Runner compiles, records missions, and the sentinel halts a
  drive mid-wave with state left on disk.

### [ ] W1.8b — Lease-reclaim assertion
- **Files:** `internal/cmd/brain_recovery_test.go`
- **Do:** Set up a temp `.specd` orchestrated spec with ≥3 tasks across ≥2
  waves. Drive until the target task's lease is in-flight, then trigger the
  simulated kill. Construct a **fresh** driver, reload the session, and run
  `brain step`/`brain run` to completion with a normal recording runner. Assert
  the previously in-flight lease was reclaimed (via `LoadProgramChildLeases` /
  session status), not left permanently active.
- **Done when:** Reclaim assertion passes deterministically.

---

## Wave B — Core invariants

### [ ] W1.9a — No-double-dispatch assertion
- **Files:** `internal/cmd/brain_recovery_test.go`
- **Do:** After resume completes, assert the recording runner saw each `TaskID`
  dispatched **at most once** beyond the documented retry budget. Pin the
  expected behavior explicitly (reclaim-then-retry-once vs. skip) and assert
  that exact count. Every non-killed task completes normally.
- **Done when:** Double-dispatch invariant holds; expected dispatch counts are
  asserted exactly.

### [ ] W1.9b — Idempotent-resume assertion
- **Files:** `internal/cmd/brain_recovery_test.go`
- **Do:** Run resume twice from the post-kill state; snapshot session + leases
  after each and assert structural equality (no compounding side effects).
- **Done when:** Two resumes → identical reconciled state.

---

## Wave C — CI stress job

### [x] W1.10a — `make stress-brain-recovery` script
- **Files:** `scripts/stress-brain-recovery.sh` (new), `Makefile`
- **Do:** Mirror `scripts/stress-program.sh`. Loop the recovery scenario N
  times (seeded RNG), randomizing the wave point at which the kill fires; run
  under `-race`; fail on any double-dispatch or dangling lease. Add a
  `stress-brain-recovery` Make target.
- **Done when:** `make stress-brain-recovery` runs locally, green, deterministic
  under a fixed seed.

### [x] W1.10b — Wire into CI
- **Files:** `.github/workflows/*` (the workflow hosting the existing four
  stress jobs)
- **Do:** Add a `stress-brain-recovery` job alongside acp/orchestration/program/
  cross-process. Same Go version matrix and `-race` settings.
- **Done when:** CI runs the new job; it is green on `level-up`.

---

## Definition of done (Spec 02)
- [ ] Mid-wave kill → reclaim test green.
- [ ] No-double-dispatch + idempotent-resume tests green.
- [ ] `make stress-brain-recovery` wired into CI, deterministic, `-race`.
- [ ] Update `specs/progress.md` W1 + exit gate.
