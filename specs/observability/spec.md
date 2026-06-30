# Spec — Observability (S5)

## Introduction

The analysis plan assumed `internal/obs/` lacked structured logging and
recommended adding `log/slog`. Live evidence (see `../discrepancies.md` D7)
shows `internal/obs/log.go` (289 lines) already implements structured
`log/slog`-based logging with a tee handler. This spec drops the logging
work entirely and narrows to the genuine gap: there is no metrics emission
(counters/timers/histograms) and no tracing anywhere in the repository. Per
`go.mod`'s documented stdlib-only constraint and gate G2 (no performance
regression before this work begins, validated by S2), any addition here must
stay stdlib-only and be opt-in/zero-cost when disabled — specd is a CLI tool
invoked thousands of times in CI and scripted contexts, not a long-running
service, so observability overhead on the hot path (every invocation) must be
negligible.

## Requirement 1 — Command-duration and operation metrics

**User story:** As a maintainer investigating whether S2's frontier
optimization actually helped in practice (not just in synthetic benchmarks), I
want duration metrics for command execution, DAG computation, and verify
execution, so I can correlate real-world performance with the benchmark
claims.

**Acceptance criteria:**
1. THE SYSTEM SHALL record a duration measurement for: total command
   execution (wrapping `main.go`'s dispatch), `Observe()`/frontier
   computation (`internal/core/frontier.go`), and verify command execution
   (`internal/runner/runner.go`).
2. THE SYSTEM SHALL emit these measurements via the existing
   `internal/obs/log.go` structured logger (as structured log fields, e.g.
   `slog.Duration("dag_compute_ms", ...)`) by default — THE SYSTEM SHALL NOT
   require a separate metrics backend/exporter dependency, preserving the
   stdlib-only constraint.
3. WHERE `SPECD_METRICS_ENDPOINT` is set (a new, documented opt-in
   environment variable) THE SYSTEM SHALL additionally expose these
   measurements via a minimal stdlib `net/http` endpoint in a
   Prometheus-text-exposition-compatible format (hand-written, not the
   `client_golang` library, to preserve stdlib-only) — THE SYSTEM SHALL NOT
   start this endpoint, or pay any associated overhead, when the variable is
   unset.
4. THE SYSTEM SHALL measure the overhead of Acceptance Criterion 1's logging
   on a representative command (`specd check`) and confirm it adds no more
   than 1ms to total execution time on the existing benchmark fixtures (the
   `SPECD_JSON=1` output contract and exit codes must remain byte-identical
   — observability is additive only).

## Requirement 2 — Optional compile-time tracing hooks

**User story:** As a developer debugging a slow Brain/Pinky orchestration run
locally, I want optional tracing spans around frontier dispatch and worker
orchestration, so I can see where time actually goes without instrumenting
the code by hand each time.

**Acceptance criteria:**
1. THE SYSTEM SHALL provide tracing hooks around frontier dispatch
   (`internal/core/frontier.go` `Observe()`) and worker orchestration
   (`internal/worker/`), gated behind a build tag (e.g. `specd_trace`) so the
   default build carries zero tracing code/dependency.
2. WHEN built without the `specd_trace` tag THE SYSTEM SHALL have identical
   binary behavior and size impact to the pre-S5 binary (verified by
   `TestDefaultLinksNoDriver`-style guard, extended to cover tracing).
3. WHEN built with the `specd_trace` tag THE SYSTEM SHALL emit spans
   consumable by a standard tool — favor a minimal stdlib-compatible trace
   format (e.g. Chrome trace JSON, which `net/http/pprof`-adjacent tooling
   already supports) over adding an OpenTelemetry SDK dependency, UNLESS the
   builder records a specific justification for the dependency that the
   reviewer wave accepts (a decision gate, not a default).

## Requirement 3 — No regression to deterministic output contract

**User story:** As a CI pipeline parsing `SPECD_JSON=1` output, I want
observability additions to never appear in or alter that output, so existing
automation doesn't break.

**Acceptance criteria:**
1. THE SYSTEM SHALL NOT add any field to `SPECD_JSON=1` output as part of
   this spec — metrics/tracing are out-of-band (log fields, a separate HTTP
   endpoint, or build-tag-gated spans), never mixed into command stdout.
2. THE SYSTEM SHALL pass the existing `make perf-gate` deterministic-output
   test suite unmodified.

## Design

### Overview
Metrics ride on the existing `slog`-based logger as structured fields by
default (zero new dependency, zero new opt-in surface to document), with an
optional pull-based HTTP endpoint behind an env var for users who want
Prometheus-style scraping. Tracing is fully opt-in via build tag, so the
default release binary is unaffected.

### Architecture
`internal/obs/` gains two new files: `metrics.go` (duration recording +
optional HTTP exposition) and `trace_stub.go`/`trace_enabled.go` (build-tag
pair for tracing, following Go's standard `//go:build` stub pattern already
likely used elsewhere for the redis/postgres backend gating per discrepancy
D5's confirmation of build-tag-gated optional backends).

### Components and interfaces
- `internal/obs/metrics.go` (new) — `RecordDuration(name string, d
  time.Duration)` helper wrapping `slog`; optional HTTP handler when
  `SPECD_METRICS_ENDPOINT` is set.
- `internal/obs/trace_stub.go` (new, default build) — no-op tracing API.
- `internal/obs/trace_enabled.go` (new, `specd_trace` build tag) — real
  span recording.
- `main.go` — wrap top-level dispatch with `RecordDuration`.
- `internal/core/frontier.go` — wrap `Observe()`.
- `internal/runner/runner.go` — wrap verify execution.

### Data models
No `state.json` change. `SPECD_METRICS_ENDPOINT` is a new documented env
var (per analysis-plan §6's "Data/Configuration" intent), not a config-file
schema change.

### Error handling
Metrics/tracing failures (e.g., HTTP endpoint bind failure) SHALL log a
warning and continue — observability must never cause a command to fail
that would otherwise succeed.

### Verification strategy
- Overhead: benchmark `specd check` before/after Requirement 1, confirm
  <1ms added (Requirement 1.4).
- Build-tag isolation: a test confirming the default build has no
  tracing-related symbols/imports (binary size or `go list -deps` check).
- `make perf-gate` unmodified pass (Requirement 3.2).

### Risks and open questions
- Risk: even structured-log-based metrics add allocation/syscall overhead on
  every invocation, which matters for a CLI tool run thousands of times in
  CI. Requirement 1.4's <1ms bar is the explicit guard against this.
- Open question: is `SPECD_METRICS_ENDPOINT` actually wanted, or is
  out-of-band structured-log metrics sufficient for this project's scale
  (a CLI tool, not a long-running service)? Builder should treat
  Requirement 1.3 as the lower-priority half of Requirement 1 and confirm
  with the user before building the HTTP exposition path if scope needs to
  shrink under time pressure — Requirement 1.1/1.2 (log-based metrics) is
  the must-have; 1.3 (HTTP endpoint) is a stretch.
