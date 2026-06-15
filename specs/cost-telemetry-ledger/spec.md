# spec.md — Cost & Telemetry Ledger

**Status:** proposed
**Source:** specd-report.html §8 idea **C3** (impact: med · effort: low · moat: low)
**Date:** 2026-06-16
**Scope:** per-task telemetry captured into state; surfaced in `specd report`.

---

## 1. Objective

Record per-task wall-clock, retries, verify duration, and (optionally) token/cost
annotations into state. `specd report` then shows where time/money goes per
wave — the missing analytics layer for agent work. Teams adopt what they can
measure and justify to finance; specd already records the structure, so add the
meter.

> **Hard invariant:** deterministic, stdlib-only, append-only audit. Telemetry is
> captured from facts specd already knows (timestamps, exit codes, verify
> durations) plus **operator-supplied** token/cost annotations — specd never
> calls a pricing API or an LLM to estimate cost. Time is injected via the
> existing clock abstraction so tests stay deterministic.

## 2. Context

- `VerificationRecord` (`state.go`) already captures verify timing context
  (HEAD, output); the regression test harness has a `clock.go`
  (`internal/testharness`) — telemetry must use an injectable clock for
  determinism.
- `specd report` (`internal/cmd/report.go`, `core/report.go`) renders per-wave
  structure already.

## 3. Requirements (EARS)

- **R1 (H)** WHEN a task transitions running→complete/blocked, THE SYSTEM SHALL
  record wall-clock duration, attempt/retry count, and cumulative verify
  duration into the task's state.
- **R2 (M)** WHERE an operator/agent supplies token or cost annotations
  (`specd task ... --tokens N --cost X`), THE SYSTEM SHALL store them on the
  task record; THE SYSTEM SHALL NOT compute cost itself.
- **R3 (M)** THE SYSTEM SHALL aggregate telemetry per wave and per spec and
  expose it in `specd report` (and `SPECD_JSON=1`) as a time/retry/cost
  breakdown.
- **R4 (M)** THE SYSTEM SHALL derive all timing from an injectable clock so tests
  are deterministic (no wall-clock flakiness).
- **R5 (M)** THE telemetry SHALL be append-only/additive in `state.json` and
  JSON-backward-compatible (`omitempty`); absence ⇒ pre-telemetry spec still
  loads.
- **R6 (L)** WHERE telemetry is absent for a task, report rendering SHALL show
  "—" rather than zero or an error.

## 4. Design / approach

1. **Record fields** — add `Telemetry` to `TaskState` (`state.go`):
   `{ DurationMS, Retries, VerifyMS, Tokens *int, Cost *string }`, all
   `omitempty`.
2. **Capture** — on status transitions in `internal/cmd/task.go`/`verify.go`,
   compute durations from the injectable clock; increment retries on re-runs.
3. **Annotations** — `--tokens`/`--cost` flags on `specd task` write the
   operator-supplied values (never computed).
4. **Aggregate + render** — extend `core.ReportData` with per-wave/per-spec
   roll-ups; render in text/HTML/JSON.

## 5. Non-goals

- No pricing API, no LLM cost estimation — cost is operator-supplied.
- No real-time metering daemon (that overlaps `specd watch`, C1).
- No change to gate semantics.

## 6. Acceptance criteria

- Completing tasks records duration/retries/verify-duration; re-running a verify
  increments the retry count.
- `--tokens`/`--cost` annotations are stored verbatim; specd computes no cost.
- `specd report` (+ JSON) shows per-wave/per-spec time/retry/cost breakdown;
  missing telemetry renders "—".
- Timing uses the injectable clock (deterministic tests); old `state.json` still
  loads; `make ci` green; stdlib-only.
