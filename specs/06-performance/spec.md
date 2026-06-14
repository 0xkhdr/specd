# Stage 06 — Performance

## Scope

Pure optimization, run last among code changes (after correctness is locked) so
benchmarks compare against settled behavior. specd is a short-lived CLI, so
these are modest wins — prioritize the ones that also improve clarity. No
behavior change; every existing test must pass byte-identical.

## Current state & findings

### F1 — [MEDIUM] Regex compiled per call / in loops
- `internal/core/boot_detectors.go:45` — `regexp.MustCompile` inside a `for`
  loop over candidate names. Recompiles on every iteration. Hoist by building a
  small cache or compiling the loop-invariant parts once. (Flagged in Stage 05
  with a TODO marker.)
- `internal/core/report.go:46` — `regexp.MustCompile(... + QuoteMeta(heading))`
  built per `extractSection` call. If headings come from a fixed set, memoize
  in a `map[string]*regexp.Regexp`; otherwise at least document why per-call is
  required.
All package-level regexes (`tasksparser.go:47-52`, `render.go:111-143`,
`decision.go:12`, `memory.go:14`, `slug.go`) are already correctly hoisted —
**good**, leave them.

**Intent:** hoist the two per-call/in-loop compilations; add a benchmark for
boot detection to confirm the win and guard regressions.

### F2 — [LOW] `DagTasksFromState` allocates a fresh slice every call
`render.go:43` builds a new `[]DagTask` from the state map on each call, and it
is called in hot-ish paths (`deriveStatus` in `task.go:40`, `next`, `status`,
`waves`). For typical spec sizes (tens of tasks) this is negligible, but it is
called multiple times per command.

**Intent:** measure first. If a single command calls it >1×, cache the derived
slice within the command invocation (pass it down) rather than recomputing. Do
**not** add a stateful cache to `DagTasksFromState` itself (would break the
pure-function contract and the frozen-clock test model). Pre-size the slice with
`make([]DagTask, 0, len(state.Tasks))`.

### F3 — [LOW] `byID()` rebuilt on every DAG operation
`dag.go:44-50` rebuilds the id→task map in `DetectCycle`, `WaveViolations`,
`NextRunnable`, `RunnableFrontier`, `CriticalPath`, `OrphanDeps`. A single
`check` run calls several of these, each rebuilding the map. For large specs
this is repeated O(n) work.

**Intent:** for `check`'s gate pipeline (Stage 03 `GateDAG`), build `byID` once
and pass it to the DAG predicates via an internal variant
(`detectCycleWith(m)`, etc.), keeping the public functions as thin wrappers that
build the map. Only do this if a benchmark shows it matters; otherwise document
and skip.

### F4 — [LOW] Slice pre-allocation misses
Audit `make`-able slices built by `append` from a known length:
- `check.go` `violations`/`warnings` grow unbounded — fine (count unknown).
- `task.go` blocker rebuild (Stage 03 already addresses via `RemoveBlocker`;
  ensure it pre-sizes).
- `dag.go:182-189` `pending`/`blocked` — pre-size to `len(remaining)`.
- `state.go` `deriveStatus` `vals` already pre-sized (`task.go:11`) — good.

**Intent:** add `make(..., 0, n)` where `n` is known and the slice is on a path
called per task. Cosmetic but cheap.

### F5 — [LOW] `json.MarshalIndent` everywhere
Every JSON command calls `json.MarshalIndent` (buffers whole doc). Output is
small (one state/report), so streaming `json.Encoder` buys nothing and would
complicate the Stage 04 `PrintJSON` helper. **Decision: no change** — record
that MarshalIndent is intentional for small, human-readable output.

## Non-goals
- Micro-optimizing the short-lived process startup — not worth complexity.
- Caching across process invocations (would fight determinism).

## Acceptance criteria
1. No `regexp.MustCompile` remains inside a loop or per-call hot path (F1);
   benchmark proves boot-detection improvement.
2. `byID`/`DagTasksFromState` recomputation reduced where a benchmark shows >1×
   per command; pure-function contracts preserved.
3. Known-length slices pre-allocated.
4. **Zero** behavior/output change: full suite passes byte-identical, including
   golden files.
5. `go test -race ./... && go test -bench . ./internal/core/` green.
