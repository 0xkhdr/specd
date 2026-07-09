# Context Manifest Scope and Budget Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| CMS-T1 | 1 | - | craftsman | `internal/context/manifest.go` | Manifest entries include deterministic reason codes and citations. | `go test ./internal/context -count=1` |
| CMS-T2 | 1 | - | craftsman | `internal/core/gates/contextbudget.go` | Budget diagnostics include bytes, estimated tokens, limit, and largest contributors. | `go test ./internal/core/gates -run 'Test.*Context|Test.*Budget' -count=1` |
| CMS-T3 | 2 | CMS-T1 | craftsman | `internal/context/manifest.go`, `internal/core/tasksparser.go` | Task file scope is honored; unrelated files excluded unless reason present. | `go test ./internal/context ./internal/core -count=1` |
| CMS-T4 | 2 | CMS-T2,CMS-T3 | craftsman | `docs/agent-integration.md`, `docs/command-reference.md` | Docs describe context reasons, budget diagnostics, and truncation behavior. | `./scripts/docs-lint.sh` |
| CMS-T5 | 3 | CMS-T4 | validator | `.` | Flake check passes. | `go test ./... -count=2` |

## Implementation Notes
- Prefer small typed reason enum.
- Keep manifest JSON schema backward compatible if existing consumers depend on it.
- Avoid reading whole repository as fallback.

