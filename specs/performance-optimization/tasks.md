# Tasks — Performance Optimization (S2)

## Wave 1

- [ ] T1 — Trace the Observe()/RunnableFrontier/NextRunnable call graph
  - why: confirm whether both `RunnableFrontier` and `NextRunnable` are hot per-observation (open question in spec.md Design/Risks) before designing the incremental index, and confirm there's no existing dependents-index already partially built elsewhere
  - role: investigator
  - files: internal/core/frontier.go, internal/core/dag.go, internal/core/orchestration_driver.go
  - contract: read `Observe()` in full and every call site of `RunnableFrontier`/`NextRunnable` in `internal/core/` and `internal/cmd/`; report which functions are called per task-completion event vs. once per orchestration run vs. once per CLI invocation (e.g. `specd next`). Do NOT write or modify code.
  - acceptance: a written call graph showing every caller of `RunnableFrontier`/`NextRunnable`, annotated with call frequency (per-event / per-run / per-CLI-invocation)
  - verify: N/A
  - depends: —
  - requirements: 2

## Wave 2

- [ ] T2 — Add baseline benchmarks
  - why: record pre-optimization performance per Requirement 1, before any production code changes (action-prompt rule: measure before and after)
  - role: builder
  - files: internal/core/dag_bench_test.go (new)
  - contract: add `BenchmarkRunnableFrontier20`, `BenchmarkRunnableFrontier100`, `BenchmarkRunnableFrontier500` (or table-driven equivalent with `b.Run` subtests), generating synthetic task graphs with realistic wave/depends shape (not a degenerate chain or fully disconnected set — mirror patterns visible in `internal/core/dag_test.go` fixtures). Run and record `ns/op`/`allocs/op` for each size in this task's evidence.
  - acceptance: benchmark file compiles and runs; baseline numbers recorded in task evidence for all three sizes
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$
  - depends: T1
  - requirements: 1

## Wave 3

- [ ] T3 — Implement incremental frontier maintenance
  - why: replace the O(V+E)-per-call full rescan with dependent-index-based incremental updates, per Requirement 2 and T1's call-graph findings
  - role: builder
  - files: internal/core/dag.go, internal/core/frontier.go
  - contract: build a `dependents map[string][]string` index once per `Observe()` call (or shared across `RunnableFrontier`+`NextRunnable` within one call if T1 found both are hot in the same path); on a task completion, only re-evaluate runnability for that task's direct dependents rather than the full task list. Public function signatures (`RunnableFrontier`, `NextRunnable`, `Observe`) MUST NOT change. Every existing case in `internal/core/dag_test.go` MUST still pass unmodified, with byte-identical output (same task IDs, same order) — this is the correctness bar per Requirement 2.2. Do NOT add any new field to `state.json` or any persisted struct (Requirement 3.2) — the index is rebuilt from `DagTask.Depends` on each call, not persisted.
  - acceptance: `go test ./internal/core/... -run TestDag -race -count=1` passes unmodified; new benchmark numbers from T2's fixtures show no regression at 20 tasks and measurable improvement at 100/500 tasks
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -race -count=1 && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$
  - depends: T2
  - requirements: 2

## Wave 4

- [ ] T4 — Concurrency/invariant regression check
  - why: confirm the incremental frontier introduces no race and preserves INV1 (atomic, versioned, concurrency-safe state mutation), per Requirement 3
  - role: verifier
  - files: N/A
  - contract: run the full race-detector test suite and the orchestration stress target against T3's changes
  - acceptance: zero new race detector findings; `make stress-orchestration` completes without new failures attributable to T3
  - verify: cd /var/www/html/rai/up/specd && make test && make stress-orchestration
  - depends: T3
  - requirements: 3

- [ ] T5 — Benchmark comparison report
  - why: action-prompt rule — reject regressions >5%; gate G2 requires a recorded, validated benchmark baseline before observability work (S5) begins
  - role: reviewer
  - files: internal/core/dag_bench_test.go
  - contract: compare T3's post-change benchmark output against T2's baseline at all three sizes; compute percentage delta for `ns/op` at each size. Confirm the 20-task case did not regress >5% and the 500-task case improved.
  - acceptance: written before/after comparison table (ns/op, allocs/op, % delta) for 20/100/500 task sizes, attached as task evidence; no size regresses >5%
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... -bench=BenchmarkRunnableFrontier -benchmem -run=^$ -count=5
  - depends: T4
  - requirements: 2
