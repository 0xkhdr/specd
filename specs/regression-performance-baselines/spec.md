# S11 — Performance Baseline Regression

## 1. Purpose and requirement coverage

Guarantee no performance regression against recorded baselines. Covers **R13**.

## 2. Verified current state

- Bench target: `make bench` runs
  `go test ./internal/cmd/... ./internal/mcp/... -run '^$' -bench 'Init|Probe|Detection' -benchmem`
  (Makefile:90) — informational, never a CI gate.
- Benchmarks: `internal/cmd/init_benchmark_test.go`, `internal/core/dag_bench_test.go`.
- Baselines doc: `docs/agent-harness-baselines.md` (present, 4.4K). Related:
  `docs/agent-harness-compat.md`, `docs/agent-harness-gap-analysis.md`.
- Deterministic-output (not latency) gated by `make perf-gate` (Makefile:86);
  latency is tracked, not gated.

## 3. Proposed design and end-to-end flow

Record current `make bench` numbers into `docs/agent-harness-baselines.md` as the
reference, then compare subsequent runs within ±10%. Because bench is
informational, the regression signal is a documented before/after comparison, not
a hard CI failure. DAG bench (`dag_bench_test.go`) guards scheduling throughput.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** benchmark names (`Init|Probe|Detection`) so baselines stay
  comparable; baseline doc format.
- **Dependencies:** S1–S10 (measures the finished surfaces).

## 5. Invariants, security, errors, observability, compatibility, rollback

- Baselines are informational; a regression is investigated, not auto-failed.
- **Observability:** `-benchmem` captures alloc regressions.
- **Rollback:** doc-only; revert baseline edits.

## 6. Acceptance criteria and validation commands

- `make bench` runs and emits comparable numbers.
- Results recorded in `docs/agent-harness-baselines.md`.
- No benchmark regresses >10% from the recorded baseline (documented).

## 7. Open decisions and deviations

- Deviation U6: exact baseline values must be read from
  `docs/agent-harness-baselines.md` locally and refreshed; the analysis plan did
  not fetch them. `make bench` is informational — this spec does NOT add a CI
  latency gate (matches Makefile intent).
