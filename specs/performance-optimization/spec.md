# Spec — Performance Optimization (S2)

## Introduction

The analysis plan assumed DAG/frontier computation lived in `internal/worker/`
and that `internal/spec/` needed parser-allocation work. Live evidence (see
`../discrepancies.md` D5, D6) refutes both location claims: the dependency
graph and frontier logic are in `internal/core/dag.go`/`frontier.go`, and the
task-list parser is `internal/core/tasksparser.go` (which shows no allocation
hotspot at spec-file scale). This spec retargets performance work to the one
confirmed inefficiency: `RunnableFrontier`/`NextRunnable`
(`internal/core/dag.go:162,227`) each do a full O(V+E) scan over all tasks and
their dependency edges, and are called once per `Observe()`
(`internal/core/frontier.go`) — i.e., once per task-completion event during
orchestration. Over a full run of V tasks, that is O(V·(V+E)) cumulative work
where incremental frontier maintenance would make it O(V+E) amortized.

**Out of scope (do not implement):** any change to `internal/worker/` (it has
no DAG logic — it only executes already-dispatched work); any streaming
rewrite of `internal/core/tasksparser.go` (no evidence of a hotspot at spec
scale — specs are hand-authored markdown files, not multi-MB inputs).

## Requirement 1 — Baseline before any change

**User story:** As a maintainer, I want a recorded performance baseline
before any DAG optimization lands, so a regression is caught by comparison,
not guesswork (action-prompt rule: measure before and after).

**Acceptance criteria:**
1. THE SYSTEM SHALL have a benchmark (`func Benchmark...`) exercising
   `RunnableFrontier`/`NextRunnable` (or the `Observe()` call path that
   invokes them) at three task-graph sizes: 20, 100, and 500 tasks, with a
   representative dependency density (not a degenerate linear chain or a
   fully disconnected graph — mirror the wave/depends shape seen in
   real specs).
2. THE SYSTEM SHALL record the baseline `ns/op` and `allocs/op` for each size
   in this spec's task evidence before any production code in Requirement 2
   changes.

## Requirement 2 — Incremental frontier maintenance

**User story:** As an operator running Brain/Pinky orchestration on a
large spec (100+ tasks), I want frontier recomputation to scale with the
number of newly-unblocked tasks, not the full graph size, so that
orchestration of large specs doesn't slow down quadratically as tasks
complete.

**Acceptance criteria:**
1. WHEN a task transitions to `TaskComplete` THE SYSTEM SHALL update the
   runnable frontier by examining only that task's direct dependents (tasks
   whose `Depends` includes the completed task's ID), not by rescanning the
   full task list.
2. THE SYSTEM SHALL produce byte-identical `NextResult`/frontier output
   (same task IDs, same ordering) compared to the current
   `RunnableFrontier`/`NextRunnable` implementation, for every existing test
   case and the new benchmark fixtures — this is a performance change, not a
   behavior change.
3. WHEN benchmarked against the Requirement 1 baseline AT THE SAME task-graph
   sizes THE SYSTEM SHALL show no regression and SHALL show measurable
   improvement at the 100- and 500-task sizes (>5% improvement at 500 tasks
   is the target; reject any change that regresses the 20-task case by >5%,
   since small specs are the common case and must not get slower).

## Requirement 3 — No invariant regression

**User story:** As a maintainer relying on INV1 (atomic, versioned state
mutations safe for concurrent agents), I want the frontier optimization to
preserve existing concurrency guarantees, so performance work doesn't
introduce a race.

**Acceptance criteria:**
1. THE SYSTEM SHALL pass `make test -race` and the existing orchestration
   stress targets (`make stress-orchestration`) with the new incremental
   frontier logic, with zero new race detector findings.
2. IF the incremental frontier requires new mutable state (e.g., a
   dependent-tasks index) THEN THAT STATE SHALL be derived/rebuilt
   deterministically from `state.json` on load — it SHALL NOT become a new
   persisted field (no `state.json` schema migration in this spec).

## Design

### Overview
Replace the full-rescan `RunnableFrontier`/`NextRunnable` with an in-memory
index (built once per `Observe()` call, from the loaded state, not persisted)
mapping each task ID to its direct dependents. On a completion event, only
the completed task's dependents are re-checked for runnability, rather than
every task in the graph.

### Architecture
`internal/core/frontier.go`'s `Observe()` currently calls
`DagTasksFromState(state)` then `RunnableFrontier(tasks)` — both O(V+E) per
call. The optimization keeps `DagTasksFromState` (state→DagTask conversion is
unavoidable and not the hotspot) but replaces the frontier scan: build a
`dependents map[string][]string` once (O(V+E), same as today, but only once
per `Observe()` call instead of implicitly twice via `byID` + scan in both
`RunnableFrontier` and `NextRunnable` if both are called in the same path —
T1 must confirm whether both are actually called per-observation or just one).

### Components and interfaces
- `internal/core/dag.go` — `RunnableFrontier`/`NextRunnable` signatures
  unchanged (preserve the public contract); internal scan logic replaced.
- `internal/core/frontier.go` — `Observe()` unchanged signature.
- `internal/core/dag_bench_test.go` (new) — benchmark fixtures.

### Data models
No `state.json` schema change (Requirement 3.2). The dependents index is
ephemeral, rebuilt from `DagTask.Depends` on each `Observe()` call.

### Error handling
No new error paths. Existing `NextResult` kinds (`NextTask`,
`NextAllComplete`, `NextAllBlocked`, `NextWaiting`) are unchanged.

### Verification strategy
- Benchmark: `internal/core/dag_bench_test.go`, three sizes, before/after
  comparison against Requirement 1's recorded baseline.
- Unit: existing `internal/core/dag_test.go` test cases must all still pass
  unmodified (output-identity is the correctness bar, per Requirement 2.2).
- Race: `make test -race`, `make stress-orchestration`.

### Risks and open questions
- Open question: is `RunnableFrontier` called independently of
  `NextRunnable` anywhere in production code, or only via `Observe()`? If
  both are hot, the dependents index should be shared/cached across both
  calls within one `Observe()` invocation rather than rebuilt twice. T1
  (investigator) must resolve this before T2 (builder) starts.
- Risk: an incremental index that's wrong in an edge case (e.g., a task with
  zero dependents, or a dependency cycle that shouldn't exist but isn't
  currently rejected) could silently miss a runnable task. Requirement 2.2's
  byte-identical-output bar against the full existing test suite is the
  primary defense — do not weaken or skip existing `dag_test.go` cases.
