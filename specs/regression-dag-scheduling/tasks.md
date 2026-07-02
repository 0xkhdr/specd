# S3 Tasks — DAG & Scheduling Regression

Requirement coverage: R3. Dependencies: S2.

## Wave 1 — Baseline (after S2 green)

- [ ] Inventory current DAG assertions in `dag_test.go`, `frontier_test.go`,
  `gates_test.go`; note missing cases (cycle, orphan, empty-frontier reason).
- **Validation:** `go test ./internal/core/... -run 'DAG|Frontier' -race -count=1`

## Wave 2 — Core regression tests (depends on Wave 1)

- [x] Frontier correctness: deps-complete → runnable; deps-incomplete → excluded.
  File: `internal/core/frontier_test.go` (extend).
- [x] Cycle rejection: build a cyclic graph, assert `GateDAG` violation. File:
  `internal/core/gates_test.go` (extend).
- [x] Orphan detection: dep referencing a missing task id → violation. File:
  `internal/core/dag_test.go` (extend).
- [x] Empty-frontier reason: all-complete graph → empty frontier + "complete"
  reason from `NextRunnable`. File: `internal/core/dag_test.go`.
- **Validation:** `go test ./internal/core/... -run 'DAG|Frontier|GateDAG' -race -count=1` ✅

## Wave 3 — Wave ordering & determinism (depends on Wave 2)

- [ ] Assert wave numbers are monotonic and stable across two builds of the same
  graph (via `DagTasksFromState`). File: `internal/core/dag_test.go`.
- [ ] Run `-count=2` for ordering stability.
- **Validation:** `go test ./internal/core/... -count=2`

## Rollout & cleanup

- [ ] Confirm no `dag_bench_test.go` regression (`make bench` informational).
- **Rollback:** revert test extensions.
- **Completion evidence:** green DAG/frontier/gate tests under `-count=2`.
