# spec.md — Native MCP Server (`specd mcp`)

**Status:** proposed
**Source:** specd-report.html §8 idea **A1** (impact: very high · effort: med · moat: high) · §9 north-star item **#1**
**Date:** 2026-06-16
**Scope:** new `internal/cmd/mcp.go` + `internal/mcp` package; no change to gate/lifecycle semantics.

---

## 1. Objective

Expose every read-safe and state-mutating specd command as **Model Context
Protocol (MCP)** tools over stdio, so any MCP host (Claude Code, Cursor, any
MCP-aware agent) calls `specd_next`, `specd_verify`, `specd_check` natively —
no shelling out, no prompt-pack parsing. This is the single biggest lever for
"world-wide used with CLI agents": it turns specd from "a CLI an agent must be
taught to drive" into "a tool the agent already knows how to use."

> **Hard invariant (do not violate):** the binary stays **stdlib-only, zero
> runtime dependencies, zero LLM calls, deterministic**. The MCP server is a
> thin JSON-RPC 2.0 transport over the *existing* command handlers. Implement
> MCP framing by hand on `encoding/json` — do **not** add an MCP SDK dependency.

---

## 2. Context (current code to build on)

- Command handlers live in `internal/cmd/*.go`, dispatched through
  `internal/cmd/registry.go`. Each already supports `SPECD_JSON=1` structured
  output via `internal/core/output.go`.
- `specd help --json` already dumps the full command schema — reuse it as the
  source of truth for generating MCP tool definitions.
- Exit codes are deterministic (`internal/core/exit.go`): 0 ok · 1 gate/verify
  fail · 2 usage · 3 not found. These map cleanly to MCP tool results
  (`isError` + structured content).
- State mutations are already atomic + lock-guarded (`lock.go`, `state.go`
  CAS) — safe to drive from a long-lived server process.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd mcp` is invoked, the system SHALL start a JSON-RPC 2.0
  server speaking MCP over stdio (newline-delimited / Content-Length framed),
  and SHALL respond to `initialize`, `tools/list`, and `tools/call`.
- **R2 (H)** THE SYSTEM SHALL generate the `tools/list` response from the same
  command schema that backs `specd help --json`, so a new command surfaces as
  an MCP tool with no separate registration.
- **R3 (H)** WHEN a `tools/call` names a specd command, the system SHALL invoke
  the existing handler with `SPECD_JSON=1` semantics and return its structured
  JSON as the tool result `content`, mapping non-zero exit codes to
  `isError: true` without crashing the server.
- **R4 (M)** WHERE a tool mutates state (`approve`, `task`, `verify`, `midreq`,
  `decision`, `memory`), the tool definition SHALL be annotated
  (`readOnlyHint: false`); read-only commands (`status`, `waves`, `context`,
  `check`, `next`, `dispatch`, `report`) SHALL be annotated `readOnlyHint: true`.
- **R5 (M)** IF a request is malformed JSON-RPC, the system SHALL return a
  spec-compliant JSON-RPC error object (codes −32700/−32600/−32601/−32602) and
  SHALL keep the server alive for subsequent requests.
- **R6 (M)** WHILE the server runs, each `tools/call` SHALL be processed under
  the same per-spec advisory lock used by the CLI, so a concurrent CLI
  invocation and an MCP call never corrupt `state.json`.
- **R7 (L)** WHERE `--root <path>` is passed to `specd mcp`, all tool calls
  SHALL resolve specs against that root; otherwise root is located as the CLI
  does today.

## 4. Design / approach

1. **Transport** — `internal/mcp/transport.go`: read framed messages from
   stdin, write to stdout, log to stderr only. Support both `Content-Length`
   headers and newline-delimited JSON; auto-detect on first byte.
2. **Schema bridge** — `internal/mcp/tools.go`: walk the existing help/command
   schema, emit one MCP tool per command. Map each flag/positional to a JSON
   Schema property. One generator, no hand-maintained duplicate.
3. **Dispatch** — `tools/call` → look up the command in `registry.go` → build
   an argv → run the handler with an in-memory `SPECD_JSON=1` writer → wrap
   stdout JSON as `content[0].text` (and `content[0].json` when present).
4. **Errors** — translate handler exit code: 0 → ok; 1/2/3 → `isError: true`
   with the structured error payload intact.
5. **No new deps** — JSON-RPC envelope structs are ~40 lines of `encoding/json`.

## 5. Non-goals

- No SSE/HTTP transport in this spec (stdio only; HTTP can be a follow-up).
- No new business logic — the server only re-exposes existing commands.
- No LLM calls, no network, no template reads from disk at runtime.
- No MCP SDK / third-party dependency.

## 6. Acceptance criteria

- `specd mcp` answers a real `initialize` + `tools/list` + `tools/call`
  handshake (covered by an integration test that drives the server over a pipe).
- `tools/list` count equals the mutating+read command count derived from the
  help schema (asserted, so adding a command without surfacing it fails CI).
- A `specd_verify` tool call produces the same JSON as `SPECD_JSON=1 specd
  verify` for the same spec/task.
- Malformed input yields a JSON-RPC error and the server survives.
- `go vet ./...`, race suite, and `make ci` stay green; binary remains
  stdlib-only (`go list -deps` shows no new module).
