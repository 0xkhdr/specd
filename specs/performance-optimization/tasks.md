# Tasks — Performance Optimization (S2)

## Wave 1

- [x] T1 — Trace the Observe()/RunnableFrontier/NextRunnable call graph
  - why: confirm whether both `RunnableFrontier` and `NextRunnable` are hot per-observation (open question in spec.md Design/Risks) before designing the incremental index, and confirm there's no existing dependents-index already partially built elsewhere
  - role: investigator
  - files: internal/core/frontier.go, internal/core/dag.go, internal/core/orchestration_driver.go
  - contract: read `Observe()` in full and every call site of `RunnableFrontier`/`NextRunnable` in `internal/core/` and `internal/cmd/`; report which functions are called per task-completion event vs. once per orchestration run vs. once per CLI invocation (e.g. `specd next`). Do NOT write or modify code.
  - acceptance: a written call graph showing every caller of `RunnableFrontier`/`NextRunnable`, annotated with call frequency (per-event / per-run / per-CLI-invocation)
  - verify: N/A
  - depends: —
  - requirements: 2
  - evidence: `RunnableFrontier`/`NextRunnable` are stateless pure functions
    (input is always the full `[]DagTask`) called from: **per-CLI-invocation**
    (`internal/mcp/watcher.go`, `internal/core/gates.go`, `internal/core/render.go`,
    `internal/cmd/status.go`, `internal/cmd/context.go` — each a one-shot
    process); **per task-completion event** during a Brain orchestration run
    (`internal/core/task_complete.go:179` calls `NextRunnable` once per
    completion; `internal/core/orchestration_sense.go:124` calls
    `RunnableFrontier` once per Brain sense/decide step via
    `SenseOrchestration` → `orchestration_engine.go:38`); **per watch-daemon
    poll tick** (`internal/cmd/watch.go`'s `watchLoop`, default 1000ms
    interval, independent of actual completions — calls
    `FrontierDetector.Observe` → `RunnableFrontier` once per spec per tick;
    this is the only call site with cross-call memory, since `FrontierDetector`
    retains `last` across calls). Both functions ARE called back-to-back on
    the identical `[]DagTask` slice within one invocation in
    `internal/cmd/next.go` (1×`RunnableFrontier` + 2×`NextRunnable`),
    `internal/cmd/dispatch.go`, and `internal/cmd/program.go` (each
    1×`RunnableFrontier` + 1×`NextRunnable`). Conclusion: genuine incremental
    maintenance (examining only a changed task's dependents, Requirement 2.1)
    is only honestly achievable in `FrontierDetector.Observe` — the sole
    stateful, repeatedly-invoked caller. T3 targets that call path.

## Wave 2

- [x] T2 — Add baseline benchmarks
  - why: record pre-optimization performance per Requirement 1, before any production code changes (action-prompt rule: measure before and after)
  - role: builder
  - files: internal/core/dag_bench_test.go (new)
  - contract: add `BenchmarkRunnableFrontier20`, `BenchmarkRunnableFrontier100`, `BenchmarkRunnableFrontier500` (or table-driven equivalent with `b.Run` subtests), generating synthetic task graphs with realistic wave/depends shape (not a degenerate chain or fully disconnected set — mirror patterns visible in `internal/core/dag_test.go` fixtures). Run and record `ns/op`/`allocs/op` for each size in this task's evidence.
  - acceptance: benchmark file compiles and runs; baseline numbers recorded in task evidence for all three sizes
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$
  - depends: T1
  - requirements: 1
  - evidence: `genWaveTasks(n)` builds waves of 5 tasks, each depending on
    1-2 tasks from the immediately preceding wave (fan-in, not a chain).
    `benchmarkFullRun` exercises `FrontierDetector.Observe` across one full
    simulated orchestration run (one completion per call, in dependency
    order) — the per-task-completion-event call path T1 identified.
    Baseline (`go test ./internal/core/... -bench=BenchmarkRunnableFrontier
    -benchmem -run=^$ -count=3`, pre-T3, avg of 3 runs):
    | size | ns/op | B/op | allocs/op |
    |------|-------|------|-----------|
    | 20   | 136047  | 214210    | 845    |
    | 100  | 2233721 | 4341452   | 12609  |
    | 500  | 80728061| 133437798 | 266166 |

## Wave 3

- [x] T3 — Implement incremental frontier maintenance
  - why: replace the O(V+E)-per-call full rescan with dependent-index-based incremental updates, per Requirement 2 and T1's call-graph findings
  - role: builder
  - files: internal/core/dag.go, internal/core/frontier.go
  - contract: build a `dependents map[string][]string` index once per `Observe()` call (or shared across `RunnableFrontier`+`NextRunnable` within one call if T1 found both are hot in the same path); on a task completion, only re-evaluate runnability for that task's direct dependents rather than the full task list. Public function signatures (`RunnableFrontier`, `NextRunnable`, `Observe`) MUST NOT change. Every existing case in `internal/core/dag_test.go` MUST still pass unmodified, with byte-identical output (same task IDs, same order) — this is the correctness bar per Requirement 2.2. Do NOT add any new field to `state.json` or any persisted struct (Requirement 3.2) — the index is rebuilt from `DagTask.Depends` on each call, not persisted.
  - acceptance: `go test ./internal/core/... -run TestDag -race -count=1` passes unmodified; new benchmark numbers from T2's fixtures show no regression at 20 tasks and measurable improvement at 100/500 tasks
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -race -count=1 && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$
  - depends: T2
  - requirements: 2
  - evidence: `dag.go` — extracted the `dagTaskOrder` comparator (no logic
    change, byte-identical sort) so `RunnableFrontier`/`NextRunnable` and
    `frontier.go`'s incremental path share one ordering by construction.
    `frontier.go` — `FrontierDetector` now keeps a per-spec `frontierCache`
    (`byID`, `dependents`, `frontier` set, `ordered` IDs — ephemeral, never
    persisted). `frontierFor` fast-paths: (1) unchanged `state.Revision` →
    return cached result in O(1); (2) same task count → `diffDagTasks` walks
    `state.Tasks` once to find IDs whose `Status` changed (O(V), cheap status
    compare only — `Depends`/`Wave` are fixed at spec-authoring time in
    specfiles.go and never mutated by orchestration, so re-verifying them
    every call would cost as much as the rescan being avoided), then
    `isRunnable` is re-evaluated only for those tasks plus their direct
    dependents via the cached index (Requirement 2.1); else full rebuild via
    `RunnableFrontier` itself (Requirement 2.2 byte-identical bar by
    construction, not by independent reimplementation). First attempt
    re-verified `Depends` every call and regressed the 100-task case by
    ~14%; removing that check (relying on the immutability invariant instead)
    fixed it — see T5 evidence. `go test ./internal/core/... -run TestDag
    -race -count=1`: pass, unmodified. `go test ./... -race -count=1`: 1640
    passed, 14 packages, zero new race findings.

## Wave 4

- [x] T4 — Concurrency/invariant regression check
  - why: confirm the incremental frontier introduces no race and preserves INV1 (atomic, versioned, concurrency-safe state mutation), per Requirement 3
  - role: verifier
  - files: N/A
  - contract: run the full race-detector test suite and the orchestration stress target against T3's changes
  - acceptance: zero new race detector findings; `make stress-orchestration` completes without new failures attributable to T3
  - verify: cd /var/www/html/rai/up/specd && make test && make stress-orchestration
  - depends: T3
  - requirements: 3
  - evidence: `make test` (workflow integration tests + `go test ./...
    -race -count=1`) — pass, all packages. `make stress-orchestration` —
    pass (`internal/integration` 1.3s, `internal/core` 1.5s), zero new
    failures. `frontierCache` is per-`FrontierDetector`-instance, in-process,
    never written to `state.json` or shared across goroutines without the
    detector's own access pattern (one detector per `watch` invocation,
    sequential polling loop) — no new concurrency surface introduced; INV1
    is unaffected since no state-mutation path changed (frontier computation
    is read-only, per the existing `Observe` doc comment).

- [x] T5 — Benchmark comparison report
  - why: action-prompt rule — reject regressions >5%; gate G2 requires a recorded, validated benchmark baseline before observability work (S5) begins
  - role: reviewer
  - files: internal/core/dag_bench_test.go
  - contract: compare T3's post-change benchmark output against T2's baseline at all three sizes; compute percentage delta for `ns/op` at each size. Confirm the 20-task case did not regress >5% and the 500-task case improved.
  - acceptance: written before/after comparison table (ns/op, allocs/op, % delta) for 20/100/500 task sizes, attached as task evidence; no size regresses >5%
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$ -count=5
  - depends: T4
  - requirements: 2
  - evidence: `go test ./internal/core/... -bench=BenchmarkRunnableFrontier
    -benchmem -run=^$ -count=5`, avg of 5 runs vs. T2's avg-of-3 baseline:
    | size | baseline ns/op | post ns/op | Δ ns/op | baseline B/op | post B/op | Δ B/op | baseline allocs/op | post allocs/op | Δ allocs/op |
    |------|---------------:|-----------:|--------:|--------------:|----------:|-------:|--------------------:|----------------:|-------------:|
    | 20   | 136047         | 102827     | -24.4%  | 214210        | 132089    | -38.3% | 845                  | 769             | -9.0%        |
    | 100  | 2233721        | 1691362    | -24.3%  | 4341452       | 2590699   | -40.3% | 12609                | 12197           | -3.3%        |
    | 500  | 80728061       | 49983813   | -38.1%  | 133437798     | 71996715  | -46.1% | 266166                | 264005          | -0.8%        |

    No size regresses (all improve); the 500-task case improves 38.1%, well
    past the >5% target, and improvement *grows* with size as expected for
    an O(V+E)→incremental change. Gate G2 (benchmark baseline recorded and
    validated before observability/S5 work) is satisfied.
