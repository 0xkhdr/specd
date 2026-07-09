# Orchestration Worker Rigor Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| OWR-T1 | 1 | - | craftsman | `internal/orchestration/acp.go`, `internal/orchestration/lease.go` | ACP and lease state machines represent dispatch, accept, reject, timeout, and completion. | `go test ./internal/orchestration -count=1` |
| OWR-T2 | 1 | - | craftsman | `internal/cmd/brain_worker.go`, `internal/core/evidence.go` | Worker reports rejected unless task, lease, evidence, and HEAD all validate. | `go test ./internal/cmd ./internal/core -run 'Test.*Worker|Test.*Evidence' -count=1` |
| OWR-T3 | 2 | OWR-T1,OWR-T2 | craftsman | `internal/cmd/brain_run.go`, `internal/orchestration/decide.go` | Brain status uses precise worker states and no overclaiming. | `go test ./internal/cmd ./internal/orchestration -run 'Test.*Brain|Test.*Decide' -count=1` |
| OWR-T4 | 2 | OWR-T3 | craftsman | `docs/agent-integration.md`, `docs/command-reference.md` | Docs describe real worker modes and limitations. | `./scripts/docs-lint.sh` |
| OWR-T5 | 3 | OWR-T4 | validator | `.` | Race-enabled full suite passes. | `go test ./... -race -count=1` |

## Implementation Notes
- Use explicit enums over free-form status strings.
- Keep worker acceptance as data validation, not prose parsing.
- If host dispatch is not implemented, mark it deferred in command metadata and docs.

