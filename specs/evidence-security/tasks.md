# Evidence and Security Gate Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| ESG-T1 | 1 | - | craftsman | `internal/core/evidence.go`, `internal/core/task_complete.go` | Completion rejects missing, fake, or unresolved HEAD. | `go test ./internal/core -run 'Test.*Evidence|Test.*Complete' -count=1` |
| ESG-T2 | 1 | - | craftsman | `internal/cmd/brain_worker.go`, `internal/orchestration/`, `internal/core/evidence.go` | Worker reports validate evidence with same path as local verify. | `go test ./internal/cmd ./internal/orchestration ./internal/core -count=1` |
| ESG-T3 | 2 | ESG-T1,ESG-T2 | craftsman | `internal/core/evidence.go`, `internal/cmd/registry.go` | Output truncation is centralized and reported consistently. | `go test ./internal/core ./internal/cmd -count=1` |
| ESG-T4 | 2 | ESG-T1 | craftsman | `internal/core/gates/`, `internal/core/gates/security/`, `internal/core/config_loader.go` | Clean-worktree and sandbox policy are config-driven, diagnostics-visible, and tested. | `go test ./internal/core/gates ./internal/core -count=1` |
| ESG-T5 | 3 | ESG-T3,ESG-T4 | validator | `.` | Race-enabled full suite passes. | `go test ./... -race -count=1` |

## Implementation Notes
- Avoid duplicate `HeadPinned` logic.
- Prefer helper that validates evidence record plus repository.
- Add regression tests before changing completion behavior.

