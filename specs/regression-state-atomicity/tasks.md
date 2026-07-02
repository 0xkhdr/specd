# S2 Tasks — State Atomicity Regression

Requirement coverage: R2. Dependencies: none (Wave-1 root).

## Wave 1 — Baseline

- [ ] Record current stress behavior: run `make stress` and capture
  turn/success counts. Files: `scripts/stress.sh`, `scripts/stress-lib.sh`.
- [ ] Inventory existing CAS/lock assertions in `state_cas_test.go`,
  `lock_test.go`, `concurrency_test.go`; note gaps.
- **Validation:** `make stress`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] Add revision-monotonicity assertion across N sequential `SaveState`
  calls. File: `internal/core/state_cas_test.go` (extend).
- [ ] Add stale-revision-write-rejected (CAS miss) assertion. File:
  `internal/core/state_cas_test.go`.
- [ ] Add torn-write guard: interrupt a save and assert prior state intact
  (extend `internal/core/write_failure_cov_test.go`).
- [ ] Add in-process concurrent-writer test with `-race` asserting
  `turn == successes`. File: `internal/core/concurrency_test.go` (extend).
- **Validation:** `go test ./internal/core/... -run 'CAS|Lock|Concurrency' -race -count=1`

## Wave 3 — Cross-process & determinism (depends on Wave 2)

- [ ] Ensure `stress.sh` asserts no torn JSON on the final `state.json` (extend
  script assertion).
- [ ] Run `-count=2` to catch iteration-order dependence.
- **Validation:** `make stress && go test ./internal/core/... -count=2`

## Rollout & cleanup

- [ ] Confirm `internal/core` coverage still ≥80% (`make cover-check`).
- **Rollback:** revert test extensions; scripts unchanged in behavior.
- **Completion evidence:** green stress + race tests.
