# S11 Tasks — Performance Baseline Regression

Requirement coverage: R13. Dependencies: S1–S10.

## Wave 1 — Baseline (after core specs green)

- [x] Read current recorded baselines. File: `docs/agent-harness-baselines.md`.
- [x] Run `make bench` and capture Init/Probe/Detection + DAG numbers with
  `-benchmem`.
- **Validation:** `make bench`

## Wave 2 — Compare & record (depends on Wave 1)

- [x] Diff measured vs. recorded baselines; flag any >10% regression.
- [x] Update `docs/agent-harness-baselines.md` with refreshed numbers + date.
- **Validation:** `make bench` (numbers within ±10% of recorded, else documented)

## Wave 3 — Guard scheduling throughput (depends on Wave 2)

- [x] Ensure `internal/core/dag_bench_test.go` still runs and is comparable.
- **Validation:** `go test ./internal/core/... -run '^$' -bench RunnableFrontier -benchmem`

## Rollout & cleanup

- [x] Note in `TESTING.md` that bench is informational, not a CI gate.
- **Rollback:** revert baseline doc edits.
- **Completion evidence:** refreshed baseline doc + documented ±10% comparison.
