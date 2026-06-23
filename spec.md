# Spec — Close the Level-Up Production Gate

> Branch: `level-up` · Baseline regression: `regression_result.md` (2026-06-23)
> Scope: turn the 3 🔴 / 2 🟡 program exit criteria green. Stdlib-only runtime
> invariant must hold throughout. No coverage floor may be lowered.

---

## 1. Problem

The level-up program audit (`regression_result.md`) found **5 of 8 program exit
criteria unmet**, nearly all cascading from **one root cause**:

`internal/cmd/brain.go:404` (and the program adapter at `:323`) hardcode the
concrete runner:

```go
runner := worker.ShellRunner{}   // brainRunWorker
```

There is **no injectable `worker.Runner` seam at the cmd/driver layer.** Spec 01
extracted the `Runner` interface into `internal/worker` (worker pkg at 90.8%
stmt), but the CLI driver still builds `ShellRunner` directly. As a result the
whole live driver path is untestable without spawning real `sh`:

- `brainRun, brainRunProgram, brainRunProgramWorker, brainRunSession,
  brainRunBootstrap, bootstrapHint, brainRunWorker, brainRunPolicy,
  workerExitCode, brainStep, brainDirective` sit at **0% coverage**.
- `cmd/brain.go` is stuck at **41.8%** vs the **≥80%** gate (W1.7).
- Spec 02's driver-level kill (W1.8) and kill+resume no-double-dispatch (W1.9)
  tests **cannot be written** — they require a recording fake runner injected at
  the driver.

Secondary gaps (independent of the seam):
- `cmd/brain.go` is **629 LOC**; spec 01 §4 requires **≤~350**.
- Overall coverage **72.8%** (target ≥85), `internal/core` **75.3%** (target ≥90).
- `golangci-lint` not installed → W3.2 "first run triaged, CI green" unverified.
- Coverage floors set to *measured-minus-1*, far below program targets; ratchet
  never ratcheted up.

## 2. Goals

1. **G1 — Runner seam at cmd layer.** Driver path takes an injectable
   `worker.Runner` (default `worker.ShellRunner{}`, byte-identical behavior).
2. **G2 — Driver recovery tests.** Recording fake runner proves: stale lease
   reclaimed, every `TaskID` dispatched at-most-once across kill+resume, resume
   idempotent. Pass under `-race -count=2`, seeded randomness.
3. **G3 — `cmd/brain.go` ≥80% coverage.** Cover the 0% driver functions via fake.
4. **G4 — `cmd/brain.go` ≤~350 LOC.** Move mechanism behind seam/helpers; CLI
   keeps routing + policy + flag parsing.
5. **G5 — Lint green.** Install `golangci-lint`, run all 11 linters, triage,
   narrow `//nolint` only where intentional.
6. **G6 — Ratchet coverage up.** overall →85, core →90; bump floors to lock.
7. **G7 — CI gate evidence.** `make ci` green end-to-end; flip `progress.md`.

## 3. Non-goals

- No new runtime dependencies (stdlib-only invariant).
- No behavior change to the live `sh -c` worker path; the default runner stays
  `ShellRunner` and output/exit-code semantics are unchanged.
- No rewrite of the engine-level recovery tests (already present and green).

## 4. Design

### 4.1 Seam (G1)

The engine already accepts a host callback: `DriverOptions.Worker
func(DriverDispatch) error` (`internal/core/orchestration_driver.go:60`) and the
analogous `ProgramDriverOptions.Worker`. The fix is **above** that callback —
inside `cmd`, where the callback is built.

- Add a `worker.Runner` parameter (or a small `brainDeps`/driver struct field)
  threaded into `brainRunWorker` / `brainRunProgramWorker` instead of the
  hardcoded `worker.ShellRunner{}`.
- Production callers pass `worker.ShellRunner{}` (default). Tests pass a fake.
- Keep mission construction (`worker.Mission{...}`) and `obs.LogEvent` calls
  intact; only the runner instance becomes injected.

### 4.2 Recording fake runner (G2)

A test-only `recordingRunner` implementing `worker.Runner`:
- Records each `Mission.TaskID` it is asked to `Run`.
- On first dispatch of a designated target task: persists the lease as in-flight,
  then returns a sentinel error that stops the drive **without releasing**
  (simulates process death holding a lease).
- On resume (fresh driver, reloaded session): runs to completion normally.

Assertions:
1. Stale lease reclaimed — released or re-leased, never dangling.
2. Each `TaskID` dispatched **at most once** across the kill+resume boundary.
3. Resume run twice → identical reconciled state (idempotent).

Test hygiene: seed all randomness; run under `-race` and `-count=2`.

### 4.3 Slimming (G4)

Target `cmd/brain.go` ≤~350 LOC by moving mechanism out of the CLI file:
- mission-build, exit-code mapping (`workerExitCode`), bootstrap hint — into a
  helper file or behind the worker seam. CLI retains command routing, policy
  defaults, and flag parsing.

### 4.4 Lint (G5)

`.golangci.yml` already enables all 11 linters (W3.1 done). Install the tool,
`golangci-lint run ./...`, fix real findings. Expected intentional suppression:
gosec on the `sh -c` worker path (`internal/worker/shell_runner.go`, tied to
`SECURITY.md`). Verify `bodyclose` on `update.go` / `watch_sse.go` and
`errorlint` `%w` / `errors.Is` correctness across the 402 `fmt.Errorf` sites.

### 4.5 Ratchet (G6)

After tests land and coverage rises, bump floors in
`scripts/coverage-check.sh` to the new measured values (lock gains, never lower).
Targets: overall ≥85, `internal/core` ≥90, `cmd/brain.go` ≥80, worker ≥90.

## 5. Acceptance criteria

| # | Criterion | Maps to |
|---|---|---|
| A1 | Driver accepts injectable `worker.Runner`; default is `ShellRunner`; live behavior unchanged | G1, exit #1 |
| A2 | Driver kill→reclaim test green; no-double-dispatch across kill+resume; idempotent resume; `-race -count=2` | G2, exit #3 |
| A3 | `cmd/brain.go` ≥80% coverage; the 11 named funcs no longer 0% | G3, exit #1 |
| A4 | `cmd/brain.go` ≤~350 LOC | G4, spec01 §4 |
| A5 | `golangci-lint run ./...` exits 0; only intentional commented `//nolint` | G5, exit #5 |
| A6 | overall ≥85%, `internal/core` ≥90%; floors raised to match | G6, exit #2 |
| A7 | `make ci` green end-to-end; `progress.md` waves flipped with evidence | G7 |
| A8 | Stdlib-only runtime invariant intact (no new non-test deps) | exit #8 |

## 6. Risks

- **Seam churn touches the live path.** Mitigate: default runner unchanged;
  assert byte-stable stdout (W2 invariant) before/after.
- **Lint surfaces many findings** across 402 `fmt.Errorf` sites. Mitigate: triage
  by linter, batch `errorlint` `%w` fixes, suppress only the documented gosec case.
- **Coverage target may need extra core tests** beyond the driver path. Mitigate:
  measure after driver tests land, then target the largest remaining 0% funcs.
