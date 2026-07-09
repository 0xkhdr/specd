# CLI Contracts and Help Truth Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| CCH-T1 | 1 | - | craftsman | `internal/core/commands.go`, `internal/cmd/registry.go` | Metadata covers all handlers and declared deferred commands. | `go test ./internal/core ./internal/cmd -count=1` |
| CCH-T2 | 1 | - | craftsman | `internal/cli`, `internal/cmd/registry.go` | Unknown verbs/flags fail closed with stable exit codes. | `go test ./internal/cli ./internal/cmd -count=1` |
| CCH-T3 | 2 | CCH-T1,CCH-T2 | craftsman | `docs/command-reference.md`, `docs/CHEATSHEET.md` | Docs reflect real command and flag contract. | `./scripts/docs-lint.sh` |
| CCH-T4 | 2 | CCH-T1 | craftsman | `internal/integration/`, `internal/core/commands.go` | Conformance test detects docs/help/MCP palette drift. | `go test ./internal/integration -count=1` |
| CCH-T5 | 3 | CCH-T3,CCH-T4 | validator | `.` | CLI contract passes vet and full tests. | `go vet ./... && go test ./... -count=1` |

## Implementation Notes
- Prefer table-driven tests for each verb.
- Keep exit-code assertions explicit.
- If docs mention flags, command metadata must expose them.

