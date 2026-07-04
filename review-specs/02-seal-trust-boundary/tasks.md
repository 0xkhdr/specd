# Tasks W2 — Seal the Trust Boundary

> Dogfooded: driven as a spec in `.specd/specs/`, closed via `specd task complete`.

## Wave 1 — MCP deny list

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P2.1 | craftsman | `internal/core/manifest_tools.go`, `internal/mcp/server.go`, `internal/mcp/parity_test.go` | — | `go test ./internal/mcp -run 'TestDenyList|TestParity'` | `tools/list` excludes approve/init/mcp/brain; `tools/call approve` → policy error; parity test asserts the deny list; malformed manifest.json → empty policy, not open |

## Wave 2 — orchestration fail-closed

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P2.2 | craftsman | `internal/cmd/brain.go`, `internal/orchestration/session.go` | — | `go test ./internal/cmd -run TestBrainFailClosed` | `brain start` on default config exits non-zero naming missing precondition; requires both `orchestration.enabled` and `mode: orchestrated`; no session file written on refusal |
| P2.3 | craftsman | `internal/cmd/{pinky,registry}.go` (or delete `pinky.go` + ADR entry in `docs/charter.md`) | P2.2 | `go test ./internal/cmd -run TestSurfaceMatchesADR` | pinky verbs registered per ADR-3, or superseding ADR recorded and file deleted; no unreachable surface |

## Wave 3 — regression

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P2.4 | validator | `internal/mcp/parity_test.go`, `internal/cmd/e2e_test.go` | P2.1, P2.2, P2.3 | `go test ./internal/mcp ./internal/cmd` | disabled orchestration ⇒ byte-unchanged CLI/check output; agent cannot reach any human gate over MCP |

## Traceability (task → requirement → finding)
- P2.1 → R2.1, R2.4 → F4 · P2.2 → R2.2 → F11 · P2.3 → R2.3 → F11 · P2.4 → R2.1–R2.4
