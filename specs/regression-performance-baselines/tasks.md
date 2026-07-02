# S11 Tasks — Performance Baseline Regression

Requirement coverage: R13. Dependencies: S1–S10.

## Wave 1 — Baseline (after core specs green)

- [ ] Read current recorded baselines. File: `docs/agent-harness-baselines.md`.
- [ ] Run `make bench` and capture Init/Probe/Detection + DAG numbers with
  `-benchmem`.
- **Validation:** `make bench`

## Wave 2 — Compare & record (depends on Wave 1)

- [ ] Diff measured vs. recorded baselines; flag any >10% regression.
- [ ] Update `docs/agent-harness-baselines.md` with refreshed numbers + date.
- **Validation:** `make bench` (numbers within ±10% of recorded, else documented)

## Wave 3 — Guard scheduling throughput (depends on Wave 2)

- [ ] Ensure `internal/core/dag_bench_test.go` still runs and is comparable.
- **Validation:** `go test ./internal/core/... -run '^$' -bench Dag -benchmem`

## Rollout & cleanup

- [ ] Note in `TESTING.md` that bench is informational, not a CI gate.
- **Rollback:** revert baseline doc edits.
- **Completion evidence:** refreshed baseline doc + documented ±10% comparison.
