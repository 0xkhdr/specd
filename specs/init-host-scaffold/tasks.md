# Init Host Scaffold Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| IHS-T1 | 1 | - | craftsman | `internal/core/agents.go`, `internal/core/scaffold.go` | Shared host model represents Claude Code, Codex, MCP config, Pinky agents, and managed blocks. | `go test ./internal/core -count=1` |
| IHS-T2 | 1 | - | craftsman | `internal/cmd/lifecycle.go`, `internal/core/commands.go` | Init flags are real, documented in command metadata, and reject unsupported values. | `go test ./internal/cmd ./internal/core -count=1` |
| IHS-T3 | 2 | IHS-T1,IHS-T2 | craftsman | `embed_templates/`, `internal/core/agents.go` | Codex and Claude scaffold files valid; Pinky frontmatter includes required fields and role tools. | `go test ./internal/core -run 'Test.*Agent|Test.*Scaffold' -count=1` |
| IHS-T4 | 2 | IHS-T1,IHS-T2 | craftsman | `internal/cmd/lifecycle.go`, `internal/core/scaffold.go` | `--dry-run`, `--repair`, `--refresh` are deterministic and preserve user content outside managed blocks. | `go test ./internal/cmd -run 'Test.*Init' -count=1` |
| IHS-T5 | 3 | IHS-T3,IHS-T4 | validator | `.` | Full suite catches flakiness. | `go test ./... -count=2` |

## Implementation Notes
- Use golden tests for generated host files.
- Make managed-block delimiters explicit and stable.
- Keep tests network-free and tempdir-based.

