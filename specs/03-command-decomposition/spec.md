# Stage 03 — Command Decomposition (check / task / verify)

## Scope

Break the three longest, highest-risk command functions into small, testable
units and extract the gate logic they share. Targets:

- `internal/cmd/check.go` — `RunCheck` is 234 lines, ~7 inline gates.
- `internal/cmd/task.go` — `RunTask` is the evidence gate; one 4-case switch
  with duplicated blocker-removal logic.
- `internal/cmd/verify.go` — `RunVerify` mixes process exec, record building,
  and presentation.

The aim is *behavior-preserving* refactor: identical exit codes, identical JSON
and human output, identical gate semantics. No gate weakens.

## Current state & findings

### F1 — [HIGH maintainability] `RunCheck` is a 234-line gate pipeline
`check.go:13-247`. Gates 1–7 + evidence are inlined into one function sharing
mutable `violations`/`warnings` slices. The review explicitly flags this
(prompt §"Specific Files", item 1). Hard to test a single gate in isolation;
adding a gate means editing the monolith.

**Intent:** define a gate as a pure function over a shared context:
```go
type checkCtx struct {
    root, slug string
    reqMd      *string
    doc        *core.ParsedTasks
    state      *core.State
    cfg        core.Config
}
type gate func(checkCtx) (violations, warnings []core.Violation)
```
Extract `gateEars`, `gateDesign`, `gateTaskSchema`, `gateDAG`, `gateSync`,
`gateTraceability`, `gateEvidence`. `RunCheck` becomes: load context →
`for _, g := range gates { v, w := g(ctx); ... }` → render. Each gate gets its
own table test. These same gate functions are reused by `approve`'s readiness
checks where overlapping (coordinate with Stage 05; for now keep `approve`
calling `core.PhaseReadiness`).

Move the gate functions into `internal/core` (e.g. `gates.go`) so both `check`
and future callers reuse them and `cmd` stays thin (architecture goal:
"commands thin, logic in core").

### F2 — [HIGH integrity] `RunTask` duplicated blocker-removal × 4
`task.go:101-201`. Each `case` rebuilds `state.Blockers` by filtering out `id`
(four near-identical loops at :147, :163, :179, :192). Drift risk: a future
edit to one case's filter and not others silently corrupts blocker tracking —
and this is *the evidence gate*, the integrity core.

**Intent:** extract `removeBlocker(state, id)` and `addBlocker(state, id,
reason, turn)` helpers in core. Each case calls one. Extract the
complete-path evidence/verification gating into
`validateComplete(ts, docTask, args) (evidence string, err error)` so the
gate logic is unit-testable without the lock/IO wrapper. `deriveStatus`
(`task.go:10-48`) stays but gets its own test matrix (all-pending, mixed,
all-complete, all-blocked).

### F3 — [MEDIUM] `RunVerify` mixes concerns
`verify.go:47-163`. Process exec, `VerificationRecord` construction, state
save, and stdout formatting are one block. Stage 01 already touches the exec
path (env scrub); here split:
- `runVerifyCommand(ctx, root, shell, command) (rec *core.VerificationRecord)`
  — pure-ish, returns the record; unit-testable with a fake command.
- presentation stays in `RunVerify`.

### F4 — [LOW] Repeated `role ∈ ReadonlyRoles` linear scans
`check.go:102-108`, `check.go:201-208`, and elsewhere loop over
`core.ReadonlyRoles` / `core.ValidRoles`. Extract `core.IsReadonlyRole(role)
bool` and `core.IsValidRole(role) bool` (back them with a `map[string]bool`
built once at init). Removes duplication and a perf nit (feeds Stage 06).

### F5 — [LOW] `min3`/`min` duplicate helpers
`check.go:249` `min3` and `verify.go:227` `min` both exist; Go 1.21+ has
builtin `min`. Confirm `go.mod` Go version (≥1.21) and delete both, using the
builtin. If `go.mod` is <1.21, hoist a single `core.Min`.

## Non-goals
- Changing any gate's pass/fail semantics or messages (those are user/agent
  contracts; only relocate them).
- Reworking JSON output shape — that is Stage 04.

## Acceptance criteria
1. `RunCheck` body ≤ ~40 lines; each gate is a separately tested function in
   `internal/core`.
2. `RunTask` has zero duplicated blocker loops; `validateComplete` and
   `deriveStatus` each have a table test.
3. `RunVerify` exec logic separated from presentation.
4. `IsReadonlyRole`/`IsValidRole` replace all inline scans.
5. **Golden parity:** existing `commands_test.go`, `lifecycle_test.go`,
   `check`-related tests pass unchanged (proves behavior preserved).
6. `go test -race ./...` green.
