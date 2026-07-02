# S3 — DAG & Scheduling Regression

## 1. Purpose and requirement coverage

Guarantee the task DAG computes a correct runnable frontier with cycle/orphan
detection and stable wave ordering. Covers **R3**.

## 2. Verified current state

- DAG core: `internal/core/dag.go` — `NextRunnable` (`dag.go:193`),
  `RunnableFrontier` (`dag.go:255`). Bench in `dag_bench_test.go`, unit in
  `dag_test.go`.
- Frontier derivation from state: `DagTasksFromState` (`internal/core/render.go:51`).
  Frontier helpers in `internal/core/frontier.go`, tested by `frontier_test.go`.
- Gate DAG validation: `GateDAG` (`internal/core/gates.go:112`), tested in
  `gates_test.go`.
- Wave surface consumed by `internal/cmd/dispatch.go` (`runDispatch`) and
  `internal/cmd/waves.go`.

## 3. Proposed design and end-to-end flow

Regression tests build task graphs and assert: frontier = tasks whose deps are
all complete; cycles are rejected by `GateDAG` with a violation; orphan deps
(reference to missing task id) are flagged; wave numbers are deterministic and
monotonic; an all-complete graph yields empty frontier with a "complete" reason.
Feed the same graph twice to assert stable ordering.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** `DagTask` shape, `NextResult.Kind` reason strings, frontier
  ordering, `GateDAG` violation categories.
- **Dependencies:** S2 (state is the frontier input via `DagTasksFromState`).

## 5. Invariants, security, errors, observability, compatibility, rollback

- No cycles or orphans in a valid plan; violations are surfaced, never silently
  dropped.
- Frontier ordering is deterministic (INV3 downstream of exit codes).
- **Rollback:** additive tests.

## 6. Acceptance criteria and validation commands

- `go test ./internal/core/... -run 'DAG|Frontier|Wave|GateDAG' -race -count=1`
  passes.
- `go test ./internal/core/... -count=2` stable.
- Cycle and orphan graphs each produce a `GateDAG` violation.

## 7. Open decisions and deviations

- Plan references `gates.go:GateDAG` and `dag.go` — both verified. No deviation.
- Open: whether wave assignment is stored in state or recomputed; tests treat it
  as recomputed by `DagTasksFromState`.
