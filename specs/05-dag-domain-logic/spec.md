# Stage 05 — DAG & Domain-Logic Correctness

## Scope

specd's core value: graph scheduling (`internal/core/dag.go`), EARS linting
(`ears.go`), the tasks parser (`tasksparser.go`), boot detection (`boot.go`,
`boot_detectors.go`), and enrichment freshness (`enrich*.go`). This is a
correctness + robustness pass, run after the state shape is stable (Stage 02)
and helpers exist (Stage 03/04).

## Current state & findings

### F1 — [MEDIUM] `CriticalPath` cycle handling is fragile
`dag.go:265-305`. `longest` memoizes by id but uses a per-root `seen` map for
cycle guarding. On a cycle it returns `[]string{id}` and `delete(seen, id)` on
unwind — but the **memo is populated with a partial path computed under a
specific `seen` context** (`dag.go:293`), then reused across different roots
with different `seen` state. This can return an incorrect longest path when a
cycle exists. In practice `check` rejects cycles before `CriticalPath` runs,
but `CriticalPath` is exported and could be called on cyclic input → wrong
result silently.

**Intent:** document the precondition (acyclic input) OR make it cycle-safe by
computing on the DAG only after `DetectCycle` returns nil. Add a guard: if
`DetectCycle(tasks) != nil`, `CriticalPath` returns `nil`. Add tests: linear
chain, diamond, disconnected components, single node, cyclic→nil.

### F2 — [LOW] `ordinal()` ignores non-leading digit groups & overflow
`dag.go:28-42`. `ordinal("T12")` = 12 ✓. But `ordinal("T1a2")` stops at the
first digit run = 1, and very large ids could overflow `int` (no clamp). Tie-
break ordering only, so impact is cosmetic, but two tasks `T1` and `T1x`
collide. Document the contract (ids are `T\d+`, enforced by `taskRE` at
`tasksparser.go:47`) and add an assertion/test that ordinal is total over valid
ids.

### F3 — [MEDIUM] Tasks parser robustness on malformed input
`tasksparser.go` (280 lines, line-based, 6 regexes at :47-52). The review (§7)
asks whether it degrades gracefully. Concerns:
- A meta line `  - depends: T1, , T3` — empty element handling in
  `ParseDepends`?
- Annotation regexes (`annotCompleteRE`, `annotBlockedRE`) use non-greedy
  groups with `·` separators; an evidence string containing `·` could mis-split
  (`tasksparser.go:51`).
- Duplicate task ids — does the parser reject or silently last-wins?
- A checkbox line without a following meta block.

**Intent:** enumerate malformed cases and assert each either parses
deterministically or returns a `SpecdError` with a line number — never panics,
never silently drops a task. Add a fuzz-ish table test. Where evidence may
contain `·`, escape it on write (`ApplyTaskAnnotation`) or switch the on-disk
separator to something evidence cannot contain, with a migration-tolerant
reader. Coordinate with Stage 03 (task.go writes annotations).

### F4 — [LOW] EARS regex coverage
`ears.go` (131 lines). Verify the patterns cover the EARS templates (ubiquitous,
event-driven "When", state-driven "While", unwanted "If", optional "Where",
complex). The review (§7) flags edge cases. Audit against the canonical EARS
grammar; add cases for lowercase keywords, leading whitespace, and combined
clauses ("When X, while Y"). Treat as lint *warnings* vs hard violations
consistently (today `check` makes them violations — confirm intended).

**Intent:** document which EARS forms are recognized; add table tests for each
form + known false-positive/negative guards. No behavior change unless a
template-valid requirement is wrongly flagged.

### F5 — [MEDIUM] Boot detection determinism & edge cases
`boot.go` + `boot_detectors.go` (448 lines, many detectors). Review (§7) asks:
empty repo, monorepo, multiple languages. Concerns:
- `boot_detectors.go:45` compiles a regex **inside a loop** (also a Stage 06
  perf item) — and per-call regex build can differ if the input list changes;
  determinism is fine but wasteful.
- Detector ordering: if two detectors match (polyglot repo), is the winner
  deterministic? Must be a stable, documented priority.
- `CheckBootFreshness` (used by `check --boot`) must be a pure function of repo
  state; ensure no time/random/env nondeterminism.

**Intent:** assert deterministic detector ordering (stable sort by a fixed
priority), document tie-breaks, and add golden tests for: empty dir, Go-only,
Node-only, Go+Node monorepo, unknown. Hoist the in-loop regex (defer actual
hoist to Stage 06 but note here).

### F6 — [LOW] Enrichment freshness contract clarity
`enrich.go` / `enrich_evidence.go` (195 + 192 lines). Review (§7) asks if the
freshness check is robust and the contract clear. Document the enrich contract
(what makes enrichment stale, what `check --enrich` compares) in
`docs/` and add tests for stale vs fresh detection. No behavior change expected
unless tests reveal a gap.

## Non-goals
- Rewriting the parser as a full grammar/AST — line-based is adequate; only
  harden edges.
- Adding new EARS forms beyond the documented templates.

## Acceptance criteria
1. `CriticalPath` returns `nil` on cyclic input; correct on diamonds/chains;
   tested.
2. Parser never panics on malformed `tasks.md`; returns line-numbered
   `SpecdError` or deterministic parse; duplicate-id policy defined + tested.
3. Annotation round-trip is lossless even when evidence contains separators.
4. EARS recognized forms documented; no template-valid requirement misflagged.
5. Boot detection has documented deterministic ordering + golden tests for
   empty/single/poly/unknown repos.
6. Enrich freshness contract documented + tested.
7. `go test -race ./...` green.
