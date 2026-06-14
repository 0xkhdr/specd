# Stage 03 — Tasks

Branch: `refactor/03-command-decomposition`. **Behavior-preserving.** Run the
full existing suite after every task; any output diff is a regression.

## T1 — Role predicates (F4)
**File:** `internal/core/agents.go` (where `ValidRoles`/`ReadonlyRoles` live; confirm via grep).

1. Add init-built sets and predicates:
   ```go
   var validRoleSet = sliceToSet(ValidRoles)
   var readonlyRoleSet = sliceToSet(ReadonlyRoles)

   func IsValidRole(r string) bool    { return validRoleSet[r] }
   func IsReadonlyRole(r string) bool { return readonlyRoleSet[r] }

   func sliceToSet(ss []string) map[string]bool {
       m := make(map[string]bool, len(ss))
       for _, s := range ss { m[s] = true }
       return m
   }
   ```
2. Replace inline scans at `check.go:90-95`, `check.go:102-108`,
   `check.go:201-208`, and any others grep finds (`grep -rn ReadonlyRoles internal`).

**Verify:** `go test ./... && gofmt -l .`

## T2 — Drop duplicate min helpers (F5)
**Files:** `internal/cmd/check.go:249`, `internal/cmd/verify.go:227`.

1. Check `go.mod` Go version. If ≥1.21, delete both `min3`/`min` and use builtin
   `min`; update `check.go:100` (`min3(len(verify),3)` → `min(len(verify),3)`)
   and `verify.go:85`.
2. If <1.21, add single `core.Min` and route both callers through it.

**Verify:** `go build ./... && go vet ./...`

## T3 — Extract check gates into core (F1)
**Files:** new `internal/core/gates.go`; rewrite `internal/cmd/check.go`.

1. Define `CheckCtx` and gate signature in `gates.go`:
   ```go
   type CheckCtx struct {
       Root, Slug string
       ReqMd      *string
       Doc        *core.ParsedTasks // adjust: same package, so *ParsedTasks
       State      *State
       Cfg        Config
   }
   type Gate func(CheckCtx) (violations, warnings []Violation)
   ```
2. Port each inlined block verbatim (same messages, same `Gate`/`Location`
   strings) into: `GateEars`, `GateDesign`, `GateTaskSchema`, `GateDAG`,
   `GateSync`, `GateTraceability`, `GateEvidence`. Keep the exact `fmt.Sprintf`
   strings from `check.go` so output is byte-identical.
3. `RunCheck` becomes: resolve root/slug/flags → build `CheckCtx` (load reqMd,
   parse tasks, load state, load config) → run ordered gate list → aggregate →
   existing JSON/human render block (unchanged). Keep the `--boot`/`--enrich`
   early returns at top.
4. Preserve gate ordering exactly: ears, design, task-schema, dag, sync,
   traceability, evidence.

**Verify:** `go test ./internal/cmd/ -run Check && go test ./...`
Add `internal/core/gates_test.go` table tests per gate (one passing + one
violating fixture each).

## T4 — Blocker helpers + complete-validation in task.go (F2)
**Files:** `internal/core/state.go` (or new `internal/core/blockers.go`); `internal/cmd/task.go`.

1. Add to core:
   ```go
   func RemoveBlocker(s *State, id string) {
       out := s.Blockers[:0]
       for _, b := range s.Blockers {
           if b.Task != id { out = append(out, b) }
       }
       s.Blockers = out
   }
   func AddBlocker(s *State, id, reason string, turn int) {
       RemoveBlocker(s, id)
       s.Blockers = append(s.Blockers, Blocker{Task: id, Reason: reason, Since: fmt.Sprintf("Turn %d", turn)})
   }
   ```
   Note: `RemoveBlocker` must allocate a fresh slice if `s.Blockers` is shared;
   use a new slice to avoid aliasing the JSON-marshaled state. Prefer:
   ```go
   kept := make([]Blocker, 0, len(s.Blockers))
   ```
2. Replace the four inline loops in `task.go` (:147-153, :163-169, :179-185,
   :192-198) with `RemoveBlocker`/`AddBlocker` calls. Blocked case:
   `AddBlocker(state, id, reason, state.Turn)`.
3. Extract complete-gate (`task.go:102-156`) into
   `validateComplete(state *core.State, ts core.TaskState, docTask *core.DagTask?, args)`:
   returns `(evidence string, serr error)` where `serr` is a `*SpecdError`
   (GateError) on any failure. Keep messages identical. `RunTask`'s
   `case TaskComplete` calls it, then applies mutations.
4. Add `deriveStatus` table test in `task_test.go` (create if absent): empty,
   not-started, mixed, all-complete→verifying, all-complete already complete,
   all-blocked→blocked.

**Verify:** `go test ./internal/cmd/ -run Task && go test ./...`

## T5 — Split verify exec from presentation (F3)
**File:** `internal/cmd/verify.go`.

1. Extract:
   ```go
   func runVerifyCommand(ctx context.Context, root, shell, command string) *core.VerificationRecord
   ```
   containing `verify.go:93-127` (exec, timing, exit-code derivation, record
   build). Reuse the env-scrub + shell-override from Stage 01.
2. `RunVerify` calls it, then does the existing save + print block unchanged.

**Verify:** `go test ./internal/cmd/ -run Verify && go test -race ./...`

## Done-when
- `go vet ./... && gofmt -l . && go test -race ./...` all green with **zero**
  changes to existing test expectations.
- `RunCheck` ≤ ~40 lines; no duplicated blocker loops in `task.go`.
- New tests: per-gate (T3), deriveStatus matrix (T4).
