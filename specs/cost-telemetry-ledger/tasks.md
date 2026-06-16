# tasks.md — Cost & Telemetry Ledger execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Clock + record recon

- [x] **T1 — Map status transitions + clock injection points** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R4
  - Report where running→complete/blocked transitions happen and how the test
    clock (`internal/testharness/clock.go`) is injected. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<transition+clock map>"`
  - **Evidence:** transitions — `RunTask` status switch
    `internal/cmd/task.go:141-184` (complete `:142-156`, blocked `:158-166`,
    running `:168-176`, pending `:178-183`); `StartedAt` set `:150`/`:171`,
    `FinishedAt` set `:149`; spec-level rollup `deriveStatus` `task.go:10-48`
    (→ Verifying/Blocked/Executing). All stamps come from `stamp := core.NowISO()`
    `task.go:139`. Clock injection — `core.Clock = time.Now` `internal/core/state.go:116`,
    `NowISO` `state.go:118-120`; tests swap it via `FakeClock`
    `internal/testharness/clock.go:19-67` — `install()` `clock.go:71-75` points
    `core.Clock` at the fake, `Epoch` `clock.go:12`, auto-advancing `Now`
    `clock.go:32-38`, `Freeze` `clock.go:56-60`. NB lock staleness deliberately
    stays on real wall clock (`state.go:113-115`, `lock.go:93`/`:110`).
    `TaskState` has **no `Telemetry` field yet** `state.go:72-85` — add it
    omitempty and capture durations via the injectable clock for back-compat.

## Wave 2 — Capture

- [x] **T2 — Add `Telemetry` to TaskState (omitempty, back-compat)** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R5
  - verify: `go test ./internal/core/ -run TestTelemetryCompat -race -count=2`
  - **Evidence:** `Telemetry{DurationMs,VerifyDurationMs,Retries,Tokens,Cost}`
    (all omitempty) + `TaskState.Telemetry *Telemetry` (omitempty) + schema mirror
    (`Telemetry` $def). Legacy tasks parse; absent telemetry omitted.
    `TestTelemetryCompat` passes. (Also fixed `reconcileTasks` to preserve
    `Telemetry` across LoadSpec, alongside Verification.)

- [x] **T3 — Capture duration/retries/verify-duration via injectable clock** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R4
  - Deterministic timing; retries increment on verify re-run.
  - verify: `go test ./internal/cmd/ -run TestTelemetryCapture -race -count=2`
  - **Evidence:** `core.DurationMsBetween` (clock-stamp diff); `verify.go`
    increments `Retries` and sets `VerifyDurationMs` per run; `task.go` complete
    sets `DurationMs` from StartedAt→FinishedAt. All via the injectable Clock →
    deterministic under FakeClock. `TestTelemetryCapture` passes `-race -count=2`.

- [x] **T4 — `--tokens`/`--cost` annotation flags (stored, not computed)** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R2
  - verify: `go test ./internal/cmd/ -run TestTelemetryAnnotate -race -count=1`
  - **Evidence:** `applyTelemetryAnnotations` stores `--tokens`/`--cost` verbatim
    into Telemetry (no flags ⇒ no telemetry created; no computation/pricing).
    Flags added to `task` metadata. `TestTelemetryAnnotate` passes.

## Wave 3 — Aggregate + render

- [x] **T5 — Per-wave/per-spec roll-up in report (+ JSON)** ✓ complete · 2026-06-16
  - role: builder · depends: T3,T4 · requirements: R3,R6
  - Missing telemetry renders "—".
  - verify: `go test ./internal/core/ -run 'TestRollupTelemetry|TestReportTelemetryRollup' -race -count=2`
  - **Evidence:** `core/telemetry.go` `RollupTelemetry` aggregates per-wave +
    per-spec (durations, retries, tokens, cost); `telemetrySection` in `report.go`
    renders the table (absent when no telemetry; missing values → "—"/"…").
    `TestRollupTelemetry` + `TestReportTelemetryRollup` pass `-race`.

- [x] **T6 — Review: no cost computation / pricing API** ✓ complete · 2026-06-16
  - role: reviewer · depends: T5 · requirements: R2
  - verify: N/A — complete with `--unverified --evidence "<no pricing call>"`
  - **Evidence:** Reviewed: tokens/cost are stored verbatim from `--tokens`/`--cost`
    annotations; nothing computes cost or calls a pricing API. The roll-up only
    sums operator-supplied numbers.

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R4 |
| 2 | T2–T4 | R1, R2, R4, R5 |
| 3 | T5–T6 | R2, R3, R6 |
