# Tasks — Regression: Core Engine (DAG, gates, state, runner, telemetry)

## Wave 1
- [ ] T1 — Baseline coverage snapshot of internal/core
  - why: establish the floor below which regression is forbidden (R1-R6)
  - role: investigator
  - files: internal/core
  - contract: run `go test ./internal/core/... -coverprofile=coverage-core.out`, record per-file %; do NOT change source
  - acceptance: a recorded baseline table mapping every core source file to its current coverage %
  - verify: go test ./internal/core/... -coverprofile=coverage-core.out
  - depends: —
  - requirements: 1, 2, 3, 4, 5, 6

- [ ] T2 — Map each requirement criterion to an existing test
  - why: find unmapped criteria (gaps) before writing new tests
  - role: investigator
  - files: internal/core
  - contract: produce a criterion->test_func matrix; mark UNMAPPED rows; do NOT edit tests
  - acceptance: matrix covering R1.1-R6.3 with file:func or UNMAPPED
  - verify: N/A
  - depends: T1
  - requirements: 1, 2, 3, 4, 5, 6

## Wave 2
- [ ] T3 — Close DAG/frontier gaps (cycle, wave order, determinism)
  - why: R1 must be fully enforced including determinism across map iteration
  - role: builder
  - files: internal/core/dag_test.go, internal/core/frontier_test.go
  - contract: add table tests for cycle detection, incomplete-dep exclusion, sorted wave determinism; do NOT modify dag.go/frontier.go unless a real bug is found
  - acceptance: every R1 criterion has a passing assertion; tests deterministic across runs
  - verify: go test ./internal/core/ -run 'DAG|Frontier' -count=3
  - depends: T2
  - requirements: 1

- [ ] T4 — Close gate/phase/task-flip gaps
  - why: R2, R3 enforce the state machine and evidence gating
  - role: builder
  - files: internal/core/gates_test.go, internal/core/customgate_test.go
  - contract: add tests for open-gate block, missing-evidence rejection, custom-gate pipeline order, monotone revision; do NOT weaken gate defaults
  - acceptance: R2.1-R2.4 and R3.1-R3.3 each have a passing assertion
  - verify: go test ./internal/core/ -run 'Gate|Phase|Task'
  - depends: T2
  - requirements: 2, 3

- [ ] T5 — Close runner/sandbox + locking gaps
  - why: R5, R6 are the security + integrity boundary
  - role: builder
  - files: internal/core/runner_sandbox_test.go, internal/core/lock_test.go, internal/core/concurrency_test.go
  - contract: assert verbatim exit/stderr, sandbox honored per mode, contended write fails one deterministically, stale-lock recovery keeps schema validity
  - acceptance: R5.1-R5.3 and R6.1-R6.3 each have a passing assertion
  - verify: go test ./internal/core/ -run 'Runner|Sandbox|Lock|Concurren' -race
  - depends: T2
  - requirements: 5, 6

## Wave 3
- [ ] T6 — Review core regression suite for gaps and flakiness
  - why: a regression suite that flakes is worse than none
  - role: reviewer
  - files: internal/core
  - contract: review T3-T5 diffs for missing criteria, nondeterminism, and weakened assertions; flag, do not rewrite
  - acceptance: one-line-per-finding review; zero UNMAPPED criteria remain; coverage >= T1 baseline
  - verify: go test ./internal/core/... -cover -race
  - depends: T3, T4, T5
  - requirements: 1, 2, 3, 4, 5, 6
