# tasks.md — Native MCP Server execution plan

Companion to [`spec.md`](spec.md). Waves run in dependency order; tasks within a
wave may run in parallel. Roles: `builder` (prod code), `verifier` (tests),
`investigator`/`reviewer` (read-only, `verify: N/A`).

---

## Wave 1 — Schema & transport foundation

- [x] **T1 — Map the command schema source of truth**
  - role: investigator · depends: — · requirements: R2
  - Trace how `specd help --json` builds its schema (`internal/cmd/registry.go`,
    `internal/core/help.go`). Report the exact struct + which fields give
    name/flags/positionals/read-only-ness. Output file:line refs only.
  - verify: N/A — complete with `--unverified --evidence "<schema field map>"`

- [x] **T2 — JSON-RPC 2.0 + MCP envelope structs**
  - role: builder · depends: — · requirements: R1,R5
  - Add `internal/mcp/transport.go`: framed read/write over stdin/stdout,
    `Content-Length` and newline modes, JSON-RPC request/response/error structs.
    stdlib `encoding/json` + `bufio` only.
  - verify: `go test ./internal/mcp/ -run TestTransport -race -count=1`

## Wave 2 — Tool generation & dispatch

- [x] **T3 — Generate `tools/list` from the command schema**
  - role: builder · depends: T1,T2 · requirements: R2,R4
  - `internal/mcp/tools.go`: one MCP tool per command, flags → JSON Schema
    props, `readOnlyHint` from the read-only command set in the spec.
  - verify: `go test ./internal/mcp/ -run TestToolsList -race -count=1`

- [x] **T4 — `tools/call` dispatch into existing handlers**
  - role: builder · depends: T3 · requirements: R3,R6,R7
  - Build argv from call params, run the handler with `SPECD_JSON=1` semantics
    under the per-spec advisory lock, map exit code → `isError`. Honor `--root`.
  - verify: `go test ./internal/mcp/ -run TestToolsCall -race -count=1`

- [x] **T5 — `specd mcp` command + registry entry**
  - role: builder · depends: T2 · requirements: R1
  - Add `internal/cmd/mcp.go` and register it. Wire `initialize` handshake.
  - verify: `make build && ./specd help | grep -q mcp`

## Wave 3 — Integration & guardrails

- [x] **T6 — End-to-end handshake test over a pipe**
  - role: verifier · depends: T4,T5 · requirements: R1,R2,R3,R5
  - Drive the server through `initialize` → `tools/list` → `tools/call`
    (`status`, `verify`) and a malformed request; assert JSON-RPC compliance and
    that the server survives bad input.
  - verify: `go test ./internal/mcp/ -run TestMCPEndToEnd -race -count=2`

- [x] **T7 — Assert tool count parity with help schema + stdlib-only**
  - role: verifier · depends: T6 · requirements: R2
  - Test that `tools/list` count equals the derived command count (adding a
    command without surfacing it fails). Assert `go list -deps` adds no module.
  - verify: `make ci`

- [x] **T8 — Review: no new deps, no LLM/network, invariants intact**
  - role: reviewer · depends: T7 · requirements: R1,R3
  - Audit the diff for runtime deps, network calls, template reads. Confirm the
    server only re-exposes existing handlers.
  - verify: N/A — complete with `--unverified --evidence "<review notes>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1–T2 | R1, R2, R5 |
| 2 | T3–T5 | R1–R4, R6, R7 |
| 3 | T6–T8 | R1–R3 |
