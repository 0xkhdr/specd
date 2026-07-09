# Concurrency and Worktree Isolation Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| CWI-T1 | 1 | - | craftsman | `internal/core/lock.go`, `internal/core/state.go` | Lock/CAS tests cover concurrent same-spec mutation and reentrant lock behavior. | `go test ./internal/core -run 'Test.*Lock|Test.*State|Test.*CAS' -count=1` |
| CWI-T2 | 1 | - | craftsman | `internal/cmd/registry.go` | Revert-on-fail uses task snapshot and preserves unrelated edits. | `go test ./internal/cmd -run 'Test.*Revert' -count=1` |
| CWI-T3 | 2 | CWI-T1,CWI-T2 | craftsman | `internal/cmd/brain_run.go`, `internal/orchestration/` | Brain blocks unsafe same-worktree parallelism or assigns managed worktrees explicitly. | `go test ./internal/cmd ./internal/orchestration -run 'Test.*Brain|Test.*Concurrent|Test.*Worktree' -count=1` |
| CWI-T4 | 2 | CWI-T3 | craftsman | `docs/agent-integration.md`, `docs/command-reference.md` | Docs state concurrency, dirty worktree, and revert-on-fail safety rules. | `./scripts/docs-lint.sh` |
| CWI-T5 | 3 | CWI-T4 | validator | `.` | Race-enabled full suite passes. | `go test ./... -race -count=1` |

## Implementation Notes
- Use file snapshots instead of broad git checkout/reset.
- Prefer explicit blocking over unsafe parallel execution.
- Managed worktree creation must be deterministic and cleaned on success/failure.

