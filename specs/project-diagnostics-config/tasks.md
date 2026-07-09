# Project Diagnostics and Config Tasks

| id | wave | deps | role | files | acceptance | verify |
|---|---:|---|---|---|---|---|
| PDC-T1 | 1 | - | craftsman | `internal/core/config_loader.go` | Loader returns effective config, diagnostics, source precedence, and stable digest. | `go test ./internal/core -run 'Test.*Config' -count=1` |
| PDC-T2 | 1 | - | craftsman | `internal/core/gates/` | Safety-relevant malformed config fails closed in gates. | `go test ./internal/core/gates -run 'Test.*Config|Test.*Gate' -count=1` |
| PDC-T3 | 2 | PDC-T1,PDC-T2 | craftsman | `internal/cmd/registry.go` | `status` and `check` expose config diagnostics in text/JSON without secrets. | `go test ./internal/cmd -run 'Test.*Status|Test.*Check|Test.*Diagnostic' -count=1` |
| PDC-T4 | 2 | PDC-T3 | craftsman | `docs/command-reference.md`, `docs/agent-integration.md` | Docs define config precedence, digest, missing config, and fail-closed cases. | `./scripts/docs-lint.sh` |
| PDC-T5 | 3 | PDC-T4 | validator | `.` | Full suite passes. | `go test ./... -count=1` |

## Implementation Notes
- Redact keys by name before rendering diagnostics.
- Keep digest over normalized config, not raw file bytes.
- Use typed diagnostics; render text at command boundary.

