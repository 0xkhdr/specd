# Agent Workflow and MCP Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| AWM-T1 | 1 | - | craftsman | `internal/core/commands.go`, `internal/mcp/` | MCP palette generated from canonical command metadata; refusal set explicit and testable. | `go test ./internal/core ./internal/mcp -count=1` |
| AWM-T2 | 1 | - | craftsman | `docs/mcp-guide.md`, `docs/agent-integration.md` | Docs describe true agent workflow, MCP support, and refusal rationale. | `./scripts/docs-lint.sh` |
| AWM-T3 | 2 | AWM-T1,AWM-T2 | craftsman | `internal/integration/`, `internal/core/scaffold.go`, `embed_templates/` | Palette/scaffold/doc conformance test fails on drift. | `go test ./internal/integration -count=1` |
| AWM-T4 | 2 | AWM-T1 | craftsman | `internal/cmd/registry.go`, `internal/core/commands.go` | Deferred and unknown command behavior covered by tests; no silent no-op. | `go test ./internal/cmd ./internal/core -count=1` |
| AWM-T5 | 3 | AWM-T3,AWM-T4 | validator | `.` | Full suite passes with race detector and docs lint. | `go test ./... -race -count=1 && ./scripts/docs-lint.sh` |

## Implementation Notes
- Prefer metadata-driven rendering over string duplication.
- Keep MCP errors deterministic and machine-readable.
- If adding flags, update `docs/command-reference.md` and `docs/CHEATSHEET.md` together.

