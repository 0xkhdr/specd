# Regression & Gap Analysis — specd Level-Up Program

> Generated: 2026-06-23 · Branch: `level-up` · Baseline: `c2cc4af`
> Source of truth audited: `specs/progress.md` + per-spec `spec.md` acceptance criteria.
> Method: measured the live tree (coverage, LOC, test inventory, build) against
> each wave's exit gate — not just the checkbox state.

---

## TL;DR

| Wave | progress.md says | Reality | Verdict |
|---|---|---|---|
| **W1** P0 — autonomous core | ⬜ | worker ✅, but brain.go coverage + driver-level recovery tests **missing** | 🔴 **blocked on one missing seam** |
| **W2** P1 — observability + ratchet | 🟡 | observability ✅; floors raised to *measured-1*, not program targets | 🟡 partial |
| **W3** P2 — hardening | ⬜ | LOC ✅, windows ✅, linters configured ✅; **lint never proven green** | 🟡 unverified |
| **W4** P3 — harness features | ✅ | code present, tests present | ✅ matches |

**Whole suite builds clean and passes** (`go build ./...` ok, `go test ./...` exit 0).
**Stdlib-only invariant intact.** The program is *not* production-grade yet: **5 of 8
program exit criteria are unmet**, and they nearly all cascade from a single root cause.

---

## Root cause (the one thing blocking W1)

`internal/cmd/brain.go:404` hardcodes the concrete runner:

```go
runner := worker.ShellRunner{}   // brainRunWorker
```

There is **no injectable `worker.Runner` seam at the cmd/driver layer.** Spec 01
extracted the `Runner` interface into `internal/worker` (good — worker pkg is at
90.8%), but the CLI driver still instantiates `ShellRunner` directly. Consequences:

1. The entire dispatch/run path (`brainRun`, `brainRunProgram`, `brainRunWorker`,
   `brainRunProgramWorker`, `brainRunSession`, `brainRunBootstrap`, `brainStep`,
   `brainDirective`, `workerExitCode`) is **untestable without spawning real `sh`** →
   stuck at **0% coverage** → brain.go can't reach 80% (W1.7).
2. Spec 02 explicitly requires "the `Runner` seam (Spec 01) with a recording fake
   runner" to assert no double-dispatch across kill+resume. With no seam, the
   **driver-level** kill test (W1.8) and the **kill+resume** no-double-dispatch
   test (W1.9) cannot be written. The existing tests only exercise the *engine*
   (`StepOrchestration`), not the live driver.

Fix this seam first; it unblocks W1.7, W1.8, and W1.9 together.

---

## Measured state

### Coverage (live, `go test ./... -coverprofile`)

| Package | Measured (func-avg) | Floor (`coverage-check.sh`) | Program target | Gate |
|---|---|---|---|---|
| `internal/worker` | **95.0%** (stmt 90.8%) | 90 | ≥90 | ✅ |
| `internal/cmd` | 67.9% | 61 | — | floor ok |
| `internal/cmd/brain.go` | **41.8%** (11 funcs @ 0%) | — | **≥80** | 🔴 |
| `internal/core` | 75.3% | 73 | **≥90** (toward 95) | 🔴 |
| overall | **72.8%** | — | **≥85** | 🔴 |
| `internal/mcp` | — | 87 | — | floor ok |
| `internal/testharness` | — | 80 | — | floor ok |

`brain.go` zero-coverage functions:
`brainStep, brainDirective, brainRun, brainRunProgram, brainRunProgramWorker,
brainRunSession, brainRunBootstrap, bootstrapHint, brainRunWorker, brainRunPolicy,
workerExitCode` — i.e. the whole live driver path.

### File size (W3 LOC gate)

No non-test file exceeds 700 LOC ✅. Largest: `internal/cmd/brain.go` **629**,
`internal/core/specfiles.go` 617, `internal/cmd/init.go` 595.

> ⚠️ **Spec 01 acceptance gap:** spec 01 §4 requires `cmd/brain.go` drop **below
> ~350 LOC** (from 692). It is still **629**. The *package* `internal/worker` was
> extracted, but brain.go itself wasn't slimmed to target. W3's 700-LOC gate passes;
> spec 01's own 350-LOC criterion does not.

### Recovery test inventory

Present (engine-level, `internal/core/orchestration_recovery_test.go`):
`TestOrchestrationRecoveryConvergesAcrossRestart`,
`TestOrchestrationRecoveryReclaimsExpiredLeases`,
`TestOrchestrationRetryReentersAfterReclaim`.

Present (cmd-level, `internal/cmd/brain_resume_test.go`):
`TestBrainResumeHappyPathNoDoubleDispatch` (asserts resume-after-*pause* writes no
new dispatch events), `TestBrainResumeUnknownSession`, `TestBrainResumeAlreadyCompleteSessionNoop`.

**Missing vs spec 02 §3.1:**
- No **driver-level kill-mid-wave** test. Existing "no double-dispatch" test covers
  the cooperative *pause→resume* path, not a *crash* (the spec's actual failure mode:
  process dies holding an in-flight lease).
- No **recording fake `worker.Runner`** that simulates death on first dispatch and
  asserts each `TaskID` dispatched at most once across the kill+resume boundary.
- No **idempotent-resume** assertion (resume twice → identical reconciled state)
  at the driver layer.
- `make stress-brain-recovery` target exists ✅ (Makefile:59) — W1.10 done.

### Lint (W3.2)

`.golangci.yml` enables all six new linters (`errorlint, gosec, bodyclose,
gocritic, unconvert, misspell`) plus the original five ✅ (W3.1 done).
**But `golangci-lint` is not installed in this environment** — "first run triaged,
CI green" (W3.2) **cannot be confirmed locally.** Only **one** `//nolint` directive
exists in the tree (`internal/worker/shell_runner.go`, the expected gosec subprocess
suppression). With 402 `fmt.Errorf` sites the `errorlint` clean-state is unverified.

---

## Per-wave gap detail

### W1 — Autonomous core (P0) 🔴
- W1.7 `internal/worker ≥90%` ✅ / `cmd/brain.go ≥80%` 🔴 (41.8%).
- W1.8 driver-level kill→reclaim test 🔴 (only engine-level exists).
- W1.9 no double-dispatch across kill+resume 🔴 (only pause/resume variant exists).
- W1.10 stress job ✅.
- Spec 01 §4: brain.go ≤350 LOC 🔴 (629).
- **Exit gate W1: NOT green.**

### W2 — Observability + ratchet (P1) 🟡
- W2.1–W2.5 observability ✅ (`internal/obs/log.go`, per-session log, structured
  events, byte-stability test, `brain why/status` timeline).
- W2.6 floors raised to *measured-minus-1* (core 73 / cmd 61 / worker 90 / mcp 87 /
  harness 80) ✅ as written — but these are **far below** the program's 85/90/80
  targets. The ratchet exists; it hasn't been ratcheted up.
- W2.7 doc + no-lowering rule ✅.
- **Exit gate W2:** logs stderr-only + stdout byte-stable ✅; floors raised locally,
  CI confirmation still pending.

### W3 — Hardening (P2) 🟡
- W3.1 linters added ✅. W3.2 triage/CI-green **unverified** (tool not installed).
- W3.3/W3.4 orchestration split ✅ (no file >700 LOC; tests green).
- W3.5/W3.6 Windows runner ✅ (`internal/worker/runner_windows.go` + tests).
- **Exit gate W3:** LOC ✅, Windows ✅, lint-green ❓.

### W4 — Harness features (P3) ✅
- W4.1 resume command + tests ✅ (`brain_resume_test.go`).
- W4.2 host adapters ✅ (`internal/integration/*`).
- W4.3 metrics export ✅ (`internal/core/report_metrics.go` + test).
- W4.4 cost brake ✅ (`internal/core/cost_brake.go` + test).
- **Exit gate W4: green.**

---

## Program exit criteria scorecard (plan §4)

| # | Criterion | Status |
|---|---|---|
| 1 | `cmd/brain.go` ≥80%; `internal/worker` ≥90% | 🔴 brain 41.8% / worker ✅ |
| 2 | Overall ≥85%, `internal/core` ≥90%, floors raised | 🔴 overall 72.8 / core 75.3 |
| 3 | Deadline/kill/pipe-drain/crash-recovery tests + CI stress job | 🟡 worker-level ✅, driver kill-recovery 🔴 |
| 4 | slog stderr + per-session log + stdout byte-unchanged | ✅ |
| 5 | `.golangci.yml` errorlint+gosec+bodyclose; CI green | 🟡 configured, green unverified |
| 6 | No non-test file >700 LOC | ✅ |
| 7 | Windows works-or-fails-fast | ✅ |
| 8 | Stdlib-only runtime invariant | ✅ |

**3 ✅ · 2 🟡 · 3 🔴.**

---

## Action plan (ordered by leverage)

### P0 — unblock W1 (do first, in this order)

1. **Add an injectable `worker.Runner` seam at the cmd layer.**
   - Give the brain driver a runner field/param (default `worker.ShellRunner{}`)
     instead of hardcoding it in `brainRunWorker`/`brainRunProgramWorker`
     (`internal/cmd/brain.go:404`).
   - Keep the default field-identical behavior; tests inject a fake.
   - *Unblocks tasks 2–4 below.*

2. **Write the driver-level recovery tests (W1.8 + W1.9).**
   - Add a `recordingRunner` fake: on first dispatch of a target task, record the
     mission, persist the lease as in-flight, then return a sentinel that stops the
     drive *without* releasing (simulates death).
   - Reload session from a fresh driver; run `brain run`/`brain step` to completion.
   - Assert: (a) the stale lease is reclaimed (released or re-leased, never dangling);
     (b) each `TaskID` dispatched **at most once** across kill+resume; (c) resume run
     twice → identical state (idempotent).
   - Seed all randomness; verify under `-race` and `-count=2`.

3. **Cover the brain driver path → brain.go ≥80% (W1.7).**
   - With the fake runner, exercise `brainRun`, `brainRunProgram`, `brainRunSession`,
     `brainRunBootstrap`, `brainStep`, `brainDirective`, `workerExitCode`,
     `brainRunPolicy`. These are the 0% functions.

4. **Slim `cmd/brain.go` below ~350 LOC (spec 01 §4).**
   - Move remaining mechanism (mission-build, exit-code mapping, bootstrap hint)
     behind the worker seam / helpers; CLI keeps routing + policy + flag parsing.

### P1 — close W2/W3 verification

5. **Install + run `golangci-lint` and triage (W3.2).**
   - `golangci-lint run ./...`; fix real findings; add narrow commented
     `//nolint:<linter> // reason` only where intentional (gosec on the `sh -c`
     worker path, tied to `SECURITY.md`). Confirm `bodyclose` on `update.go` /
     `watch_sse.go` and `errorlint` `%w`/`errors.Is` correctness.

6. **Ratchet coverage toward program targets.**
   - Raise overall 72.8→85, core 75.3→90, then bump the floors in
     `scripts/coverage-check.sh` to lock the gains (never lower).

### P2 — confirm gates in CI

7. Run `make ci` end-to-end (lint + test + test-order + cover-check + perf-gate +
   all four stress jobs incl. `stress-brain-recovery`) and capture green output as
   the W1/W2/W3 exit-gate evidence; then flip the wave statuses in `progress.md`.

---

## Quick reference — commands

```bash
# coverage by package
go test ./... -coverprofile=cov.out && go tool cover -func=cov.out | tail -1
go tool cover -func=cov.out | grep 'cmd/brain.go'      # the 0% funcs

# lint (install first)
golangci-lint run ./...

# recovery stress
make stress-brain-recovery

# full gate
make ci
```
