# Concurrency and Worktree Isolation Spec

## Purpose
Prevent brain, workers, and revert-on-fail behavior from corrupting user work or racing shared state.

## Source Gaps
- GAP-ANALYSIS.md domain 8: concurrent worker and revert-on-fail hazards.
- Existing per-spec lock serializes state, but repo worktree edits may still collide.
- Revert-on-fail can affect in-flight or user edits if scope is unclear.

## Goals
- Define isolation model for parallel tasks: serial same worktree or separate managed worktrees.
- Make `--revert-on-fail` safe by default and scoped to files owned by current task/worker.
- Detect dirty worktree conflicts before worker dispatch.
- Document concurrency limits and supported worker count.

## Non-Goals
- Do not implement distributed locking.
- Do not delete user work to recover from failures.
- Do not bypass git safety checks.

## Required Knowledge
- Locking: `internal/core/lock.go`.
- Brain run: `internal/cmd/brain_run.go`.
- Registry revert logic: `internal/cmd/registry.go`.
- State/CAS: `internal/core/state.go`.

## Functional Contract
- Same worktree execution is serial unless a worker receives a dedicated managed worktree.
- Revert-on-fail only reverts files declared for current task and only when snapshot proves ownership.
- Dirty worktree conflict blocks dispatch with actionable message.
- Lock acquisition and release are logged enough for diagnostics.

## Acceptance Criteria
- Tests cover concurrent brain workers targeting same repo and same spec.
- Tests cover revert-on-fail preserving unrelated user edits.
- Status/check reports show blocked concurrency reason.
- Docs state safe parallelism rules.

## Invariants
- Never run `git reset --hard` as recovery.
- Never revert files not owned by current task snapshot.
- State mutations remain CAS-protected.
- Per-spec lock remains reentrant.

## Verification
- `go test ./internal/core ./internal/cmd ./internal/orchestration -run 'Test.*Lock|Test.*Concurrent|Test.*Revert|Test.*Brain' -count=1`
- `go test ./... -race -count=1`

