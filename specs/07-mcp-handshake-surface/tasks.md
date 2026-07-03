# Tasks 07 — MCP & Handshake Surface

> **Build waves:** F (T7.1–T7.6). See `specs/progress.md`.
> **Depends on domains:** 06, 10, 02–05, 08. **Unblocks:** 09 (brain tools).

## Wave 1 — server & core tools

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T7.1 | craftsman | `internal/mcp/server.go`, `internal/cmd/mcp.go` | — | `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \| go run . mcp` | stdio JSON-RPC responds |
| T7.2 | craftsman | `internal/mcp/tools_core.go` | T7.1 | `go test ./internal/mcp -run TestMCPParity` | tool result == CLI JSON |
| T7.3 | craftsman | `internal/core/manifest_tools.go` | T7.1 | `go test ./internal/core -run TestForbiddenTool` | forbidden tool refused |

## Wave 2 — handshake & policy

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T7.4 | craftsman | `internal/cmd/handshake.go`, `internal/core/handshake.go` | — | `go run . handshake bootstrap --json \| grep -q version` | bootstrap returns version + policy digest |
| T7.5 | craftsman | `internal/mcp/tools_brain.go` | T7.2 | `go test ./internal/mcp -run TestBrainToolsGatedByConfig` | brain tools absent when orchestration off |
| T7.6 | validator | `internal/mcp/parity_test.go` | T7.2 | `go test ./internal/mcp -run TestMCPParity` | every exposed verb has a parity test |

## Traceability (task → requirement)
- T7.1 → R7.1 · T7.2 → R7.2, R7.6 · T7.3 → R7.4 · T7.4 → R7.5 · T7.5 → R7.3 · T7.6 → R7.2
