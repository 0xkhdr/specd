# Design — MCP Expansion for External Tools

## Overview
We extend specd to two new MCP hosts by separating *discovery* from *integration*. First an
investigator establishes, from primary sources, what antigravity CLI and Codex CLI actually
support (stdio vs HTTP/SSE, config schema). Only then do we choose the integration: the cheapest
path is a declarative config that launches the unchanged `specd mcp` stdio server; the fallback,
used only when a host cannot speak stdio, is an opt-in HTTP/SSE transport adapter that reuses the
existing `route()`/`callTool()` dispatch verbatim. This keeps `core.Commands` and the stdio path
untouched (Requirement 4.3) and avoids hardcoding any host's assumptions into specd's core.

## Architecture
```
                 ┌─────────────── discovery (Wave 1) ───────────────┐
                 │ antigravity caps        Codex caps                │
                 └───────────────┬──────────────────┬───────────────┘
                                 │ stdio?            │ stdio?
                  yes ◄──────────┤                   ├──────────► no
                  │              │                   │             │
     declarative config    (same decision)     declarative config  │
  (specd mcp, unchanged)                     (specd mcp, unchanged) │
                                                                    ▼
                                            opt-in HTTP/SSE adapter (transport_http.go)
                                              POST /rpc ─► conn.handle ─► route()
                                              GET  /sse ─► event stream of responses
                                                    │
                                                    ▼
                                       existing mcp.route / callTool / buildTools
```
The adapter is a second front door onto the *same* request router; it adds transport, never
business logic — mirroring how the stdio server is itself "no business logic" (`transport.go`).

## Components and interfaces
- **`docs/mcp-hosts/antigravity.md` + config artifact (new):** capability findings + copy-paste
  config. Backs R1, R5.
- **`docs/mcp-hosts/codex.md` + config artifact (new):** capability findings + copy-paste config.
  Backs R2, R5.
- **`internal/mcp/transport_http.go` (new, opt-in):** `ServeHTTP(addr string, dispatch Dispatcher)`
  exposing `POST /rpc` (single JSON-RPC request → response) and `GET /sse` (server-sent response
  stream). Internally constructs the same `conn`/`route` path; default bind `127.0.0.1`. Backs R4.
- **`internal/cmd/mcp.go` (edited, additive):** a `--http <addr>` flag that, when present, calls
  `mcp.ServeHTTP` instead of `mcp.Serve`; absent flag = today's stdio path byte-for-byte. Backs R4.3.
- **`internal/mcp/transport_http_test.go` (new):** sandboxed test asserting HTTP `tools/list`
  equals stdio `tools/list`. Backs R6.
- **No change** to `server.go` `route`/`callTool`/`buildTools` — the adapter calls them.

## Data models
- **Host capability record:** `{host, stdio:bool, http:bool, sse:bool, configPath, configSchema, source}` — captured in the per-host docs.
- **HTTP request:** body is a single JSON-RPC 2.0 request object identical to a stdio line.
- **HTTP response:** `200` with a JSON-RPC response body; SSE frames are `data: <json>\n\n`.
- **Host config:** host-native MCP server entry pointing at either `command: specd args:[mcp,--root,<path>]` (stdio) or a URL (HTTP), selected per Requirement 3.

## Error handling
- The HTTP adapter reuses `conn.handle`'s JSON-RPC error mapping (`-32700/-32600/-32601/-32602`);
  a malformed POST body returns a JSON-RPC parse error with HTTP `200` (JSON-RPC carries the
  error), not an HTTP `500`, so MCP clients parse it uniformly.
- Per-call panic recovery in `capture()` is preserved unchanged (R7.2); the adapter adds no new
  panic surface beyond the HTTP server's own goroutine, which is wrapped.
- A non-loopback bind requires an explicit operator-supplied address; the default never exposes
  spec contents off-host (security model).
- If discovery (Wave 1) shows a host supports stdio, the HTTP adapter is not shipped on its
  account — degrade to the no-code config path.

## Verification strategy
- **Discovery (R1, R2):** investigator notes with citations; reviewed, not auto-tested.
- **Unit (R4):** `transport_http_test.go` asserts `POST /rpc` with `tools/list` returns the
  `buildTools()` set; asserts default bind is loopback; asserts stdio path unchanged via existing
  `internal/mcp` tests still passing.
- **Integration (R3, R6):** sandboxed test using `internal/testharness` drives a `tools/call`
  over HTTP against a fixture spec and compares to the stdio result.
- **System (R5):** config artifacts validated as parseable and pointing at the chosen transport.
- Requirement→test map: R1/R2→discovery docs; R3→config+http test; R4→transport_http_test.go;
  R5→config parse test; R6→sandboxed harness test; R7→reuse existing panic-recovery test.

## Risks and open questions
- **Primary risk:** antigravity and Codex MCP support are young and may change between releases;
  discovery findings must be dated and sourced. If both already support stdio MCP servers, the
  HTTP adapter (R4) becomes optional scope and may be deferred — this is a decision gate recorded
  in decisions.md before any HTTP code is written.
- Exposing HTTP, even on loopback, widens specd's attack surface; we keep it opt-in and
  loopback-default, and never add a mutating route beyond what tools already allow.
- `capture()`'s process-global stdout swap means the HTTP adapter must serialise tool calls (one
  at a time) exactly as the stdio loop does; concurrent HTTP requests must be queued, not run in
  parallel. Open question: queue vs. reject-when-busy — to be resolved in design review.
