# Tasks — Observability (S5)

## Wave 1

- [ ] T1 — Audit internal/obs/log.go and confirm zero existing metrics/tracing
  - why: confirm the exact extension points in the existing slog-based logger before adding metrics.go, and confirm discrepancy D7's "no metrics, no tracing" finding holds for the full package, not just the sampled file
  - role: investigator
  - files: internal/obs/
  - contract: read every file in internal/obs/ in full; confirm no metrics/tracing code exists anywhere in the package (or repo-wide via grep for "otel", "prometheus", "metric" outside this review's own spec files); document the logger's existing handler/tee architecture so metrics.go can integrate cleanly rather than duplicating logger setup. Do NOT write or modify code.
  - acceptance: written confirmation of the logger architecture and a clean bill on "no existing metrics/tracing", or a corrected finding if any exists
  - verify: N/A
  - depends: —
  - requirements: 1, 2

## Wave 2

- [ ] T2 — Add log-based duration metrics (Requirement 1.1, 1.2 — must-have)
  - why: the core observability gap — no duration data for command execution, DAG computation, or verify execution exists today
  - role: builder
  - files: internal/obs/metrics.go (new), main.go, internal/core/frontier.go, internal/runner/runner.go
  - contract: add `RecordDuration(name string, d time.Duration)` in `internal/obs/metrics.go`, emitting via the existing `slog` logger from T1's findings as a structured field (e.g. `slog.Duration(name, d)`), not a new logging path. Wrap: total command dispatch in `main.go`, `Observe()` in `internal/core/frontier.go`, and verify execution in `internal/runner/runner.go`. Do NOT add any field to `SPECD_JSON=1` stdout output — this is log-only (Requirement 3.1).
  - acceptance: running any `specd` command with debug/verbose logging enabled shows duration fields for the three measured operations; `SPECD_JSON=1` output is byte-identical to pre-change output for the same command/inputs
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/obs/... ./internal/core/... ./internal/runner/... -race -count=1
  - depends: T1
  - requirements: 1, 3

- [ ] T3 — Measure and confirm overhead bound
  - why: Requirement 1.4 requires confirming <1ms overhead before this work is considered done — must not silently accept a regression
  - role: builder
  - files: internal/obs/metrics_bench_test.go (new)
  - contract: add a benchmark comparing `specd check` (or the equivalent in-process call path, not a full subprocess fork if the existing benchmark fixtures avoid that) before and after T2's instrumentation. Record the delta.
  - acceptance: written before/after comparison showing <1ms added overhead; if overhead exceeds 1ms, this task is NOT complete until T2 is optimized to meet the bound
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/obs/... -bench=. -benchmem -run=^$
  - depends: T2
  - requirements: 1

## Wave 3 (stretch — confirm scope with user before starting if time-constrained, per spec.md's open question)

- [ ] T4 — Add optional Prometheus-exposition HTTP endpoint
  - why: Requirement 1.3 — opt-in scraping support for users who want it, lower priority than T2/T3
  - role: builder
  - files: internal/obs/metrics.go
  - contract: when `SPECD_METRICS_ENDPOINT` is set, start a minimal `net/http` handler (hand-written Prometheus text exposition format, no third-party client library — stdlib-only constraint) serving the duration measurements from T2 as counters/histograms. When unset, this code path must not run or allocate anything beyond a single env-var check.
  - acceptance: with `SPECD_METRICS_ENDPOINT` unset, no HTTP listener starts and T3's overhead benchmark is unaffected; with it set, the endpoint serves valid Prometheus-text-format output
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/obs/... -race -count=1
  - depends: T3
  - requirements: 1

- [ ] T5 — Add build-tag-gated tracing hooks
  - why: Requirement 2 — optional tracing for debugging Brain/Pinky orchestration, zero cost in default builds
  - role: builder
  - files: internal/obs/trace_stub.go (new), internal/obs/trace_enabled.go (new), internal/core/frontier.go, internal/worker/
  - contract: add a no-op tracing API in `trace_stub.go` (default build) and a real span-recording implementation in `trace_enabled.go` (behind `//go:build specd_trace`). Wrap `Observe()` and worker orchestration entry points with span calls. Favor a minimal stdlib-compatible trace format (e.g. Chrome trace JSON) over an OpenTelemetry SDK dependency; if a dependency is genuinely necessary, document the specific justification for reviewer sign-off in T6 rather than adding it silently.
  - acceptance: default build (`go build`) has zero tracing-related symbols (verify via `go list -deps` or binary size comparison); `go build -tags specd_trace` produces spans consumable by the chosen trace format
  - verify: cd /var/www/html/rai/up/specd && go build -o /tmp/specd-default . && go build -tags specd_trace -o /tmp/specd-traced . && go list -deps -tags specd_trace ./... | grep -i otel || true
  - depends: T1
  - requirements: 2

## Wave 4

- [ ] T6 — Review wave: dependency and overhead sign-off
  - why: any new dependency (T5's tracing format choice) or scope cut (T4's stretch status) needs explicit reviewer sign-off, not a unilateral builder decision
  - role: reviewer
  - files: internal/obs/metrics.go, internal/obs/trace_stub.go, internal/obs/trace_enabled.go, go.mod
  - contract: confirm `go.mod` still shows zero non-stdlib dependencies (or, if T5 added one, confirm a documented justification exists and is reasonable); confirm T4 was completed or explicitly deferred with rationale recorded; confirm Requirement 3.1 (no SPECD_JSON field additions) holds across all of T2-T5's changes.
  - acceptance: written sign-off confirming stdlib-only constraint intact (or justified exception), and SPECD_JSON output contract unchanged
  - verify: cd /var/www/html/rai/up/specd && cat go.mod && make perf-gate
  - depends: T4, T5
  - requirements: 1, 2, 3

- [ ] T7 — Full verification run
  - why: gate G4 requires no performance regression >5% before documentation updates (S7) begin
  - role: verifier
  - files: N/A
  - contract: run the full test suite, perf-gate, and confirm T3's overhead measurement against the recorded S2 baseline shows no compounding regression
  - acceptance: `make test`, `make perf-gate` pass; combined S2+S5 overhead on representative commands stays within the 5% regression budget from the original S2 baseline
  - verify: cd /var/www/html/rai/up/specd && make test && make perf-gate
  - depends: T6
  - requirements: 1, 2, 3
