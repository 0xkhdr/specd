# Design — Regression: Core Engine (DAG, gates, state, runner, telemetry)

## Overview
We treat the existing `internal/core` test suite as the baseline and close gaps to a full
behavioral contract. Approach: characterize each public surface (dag, frontier, gates,
runner, lock, ears, exit, telemetry) with golden/property tests so regressions surface as
failing assertions, not as silent behavior drift. We prefer table-driven tests and a shared
fixture builder so a single graph mutation is cheap to assert across many criteria.

## Architecture
```
requirements.md (EARS) --> tests map by ID --> internal/core/*_test.go
        |                                            |
        v                                            v
   dag/frontier -- gates/phases -- runner/sandbox -- lock -- telemetry
        \---------------- state.json (CAS, schema v4) <------/
```
Each requirement maps to one or more existing test files (dag_test.go, frontier_test.go,
gates_test.go, runner_sandbox_test.go, lock_test.go, ears_test.go, concurrency_test.go)
plus new coverage where a criterion is unmapped.

## Components and interfaces
- **dag.go / frontier.go** — graph build, cycle detect, wave assignment. Contract: pure,
  deterministic ordering given identical input.
- **gates.go / phases.go / customgate.go** — gate pipeline, phase transitions. Contract:
  no advance with open gate; custom gates run in declared order.
- **runner.go / runner_sandbox.go** — verify execution. Contract: exit code + stderr
  preserved; sandbox honored.
- **lock.go / backend.go** — CAS state writes. Contract: serialized, schema-valid post-write.
- **ears.go / exit.go** — validation + exit-code taxonomy.

## Data models
state.json schemaVersion 4: {phase, gate, revision (monotone), tasks{}, blockers[],
telemetry annotations}. Tests assert schema validity after each write.

## Error handling
Cycle -> explicit cycle report, no frontier. Missing evidence -> rejected flip with reason.
Runner failure -> verbatim exit/stderr. Stale lock -> recover, no corruption.

## Verification strategy
- Unit: dag/frontier/ears/exit pure-function tables (R1, R4).
- Integration: gate pipeline + phase advance + task flip (R2, R3).
- System: concurrency_test under contention; sandbox honored under each mode (R5, R6).
- Coverage gate: `go test ./internal/core/... -cover` must not drop below current baseline.

## Risks and open questions
- Sandbox modes are platform-dependent; redis/postgres backends need services (deferred to
  regression-backends-state). Open: is wave assignment stable across map iteration? Tests
  must sort to assert determinism.
