# Stage 06 — Tasks

Branch: `refactor/06-performance`. **No behavior change.** Every golden/test must
pass byte-identical. Add benchmarks before optimizing to prove the win.

## T1 — Benchmark baseline
**Files:** `internal/core/dag_test.go`, `internal/core/boot_test.go`.

1. Add `BenchmarkDetectCycle`, `BenchmarkNextRunnable` over a synthetic
   ~200-task DAG.
2. Add `BenchmarkBootDetect` over a representative repo fixture.
3. Record baseline numbers in the PR description.

**Verify:** `go test -bench . -benchmem ./internal/core/`

## T2 — Hoist in-loop / per-call regex (F1)
**Files:** `internal/core/boot_detectors.go:45`, `internal/core/report.go:46`.

1. `boot_detectors.go:45`: move the `regexp.MustCompile(`(?i)\b`+QuoteMeta(c)+`\b`)`
   out of the loop. If `c` varies per iteration, build a `map[string]*regexp.Regexp`
   cache at package scope, or precompute the slice of compiled regexes once
   before the hot loop. Remove the per-iteration `// TODO(stage06)` marker from
   Stage 05.
2. `report.go:46`: add a package-level `var sectionRECache = map[string]*regexp.Regexp{}`
   guarded by a `sync.Mutex` (or build a compiled regex keyed by heading once).
   Reuse compiled entries across calls.

**Verify:** `go test ./internal/core/ -run 'Boot|Report' && go test -bench BootDetect ./internal/core/`
(compare to T1 baseline — must improve or stay equal.)

## T3 — Reduce DAG map rebuilds (F3) — conditional
**Files:** `internal/core/dag.go`, `internal/core/gates.go` (Stage 03 `GateDAG`).

1. Only if T1 benchmark shows `byID` rebuild is material: add internal
   `*With(m map[string]DagTask)` variants of `DetectCycle`, `WaveViolations`,
   `OrphanDeps` and have `GateDAG` build `byID` once and reuse.
2. Keep public functions building the map (back-compat).
3. If benchmark shows it is negligible, skip and add a code comment documenting
   the decision.

**Verify:** `go test ./internal/core/ -run DAG`

## T4 — Pre-allocate known-length slices (F4)
**Files:** `internal/core/dag.go:182-189`, others grep finds.

1. `pending`/`blocked` in `NextRunnable` → `make([]DagTask, 0, len(remaining))`.
2. `DagTasksFromState` (`render.go:43`) → `make([]DagTask, 0, len(state.Tasks))`.
3. Confirm Stage 03 `RemoveBlocker` pre-sizes its kept slice.

**Verify:** `go test ./internal/core/`

## T5 — Avoid duplicate DagTasksFromState per command (F2) — conditional
**Files:** commands that call it >1× (grep `DagTasksFromState` in `internal/cmd`).

1. Where one command invocation derives the slice multiple times, compute once
   and pass down. Do not memoize inside the core function.

**Verify:** `go test ./internal/cmd/`

## Done-when
- No regex compiled in a loop/per-call hot path.
- Benchmarks ≥ baseline (improved or equal, never worse).
- `go vet ./... && gofmt -l . && go test -race ./...` green, goldens unchanged.
