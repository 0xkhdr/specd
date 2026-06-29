# Spec — HTTP Transport Timeouts (A1)

**Priority:** P1 · **Wave:** 1 · **Domain:** web-service hardening (slowloris).

## Introduction

Both long-running HTTP listeners set only `ReadHeaderTimeout: 10s`. There is no
`WriteTimeout` and no `IdleTimeout` anywhere in the codebase. Slow-body and
slow-read clients can therefore pin a connection/goroutine indefinitely
(slowloris-class resource exhaustion). The `/rpc` body is already size-bounded,
but the *time* dimension is unbounded once headers are read.

This spec bounds the time dimension on both listeners while preserving the
long-lived MCP `/sse` stream.

## Current-state grounding

- `internal/mcp/transport_http.go:50-52` — MCP `--http` server: `Addr`,
  `ReadHeaderTimeout: 10 * time.Second` only.
- `internal/cmd/serve.go:186` — dashboard `serve` server:
  `http.Server{Addr, Handler, ReadHeaderTimeout: 10 * time.Second}` only.
- `internal/mcp/transport_http.go:141-148` — `/rpc` body bound via
  `ContentLength` check + `io.LimitReader(maxRPCBody+1)` (already good).
- MCP server multiplexes `/rpc` (short, bounded) and `/sse` (long-lived stream)
  on the same listener.
- `grep` confirms no `WriteTimeout`/`IdleTimeout` present anywhere.

## Requirements

### Requirement 1 — Idle connection bound on both listeners
**User story:** As an operator, I want idle keep-alive connections reclaimed, so
a client cannot hold sockets/goroutines open indefinitely.

**Acceptance criteria:**
1. The dashboard server (`serve.go`) SHALL set `IdleTimeout` (default 60s).
2. The MCP `--http` server SHALL set `IdleTimeout` (default 60s).
3. `ReadHeaderTimeout` SHALL remain set on both.

### Requirement 2 — Write bound on bounded responses
**User story:** As an operator, I want a slow-read client unable to stall a
response writer forever.

**Acceptance criteria:**
1. The dashboard server SHALL set `WriteTimeout` (static/short responses are safe
   to bound).
2. The MCP server SHALL bound `/rpc` write time **without** killing the `/sse`
   long-lived stream.

### Requirement 3 — SSE stream not severed by write deadline
**User story:** As an MCP host, I want my event stream to stay open past the
RPC write bound.

**Acceptance criteria:**
1. A server-level `WriteTimeout` SHALL NOT apply to `/sse`, OR `/sse` SHALL be
   served by a handler/connection whose write deadline is reset/cleared per
   event.
2. A test SHALL hold an `/sse` connection open longer than the `/rpc`
   `WriteTimeout` and assert it is not severed by the timeout.

## Design

- `serve.go`: add `WriteTimeout` and `IdleTimeout` to the `http.Server` literal.
- `transport_http.go`: set `IdleTimeout` on the server; do **not** set a global
  `WriteTimeout` that would cut `/sse`. Instead either:
  - split listeners/handlers so only the `/rpc` mux has a write bound, or
  - use `http.ResponseController` (or `net.Conn` `SetWriteDeadline`) to set a
    per-response write deadline on `/rpc` and clear/extend it on `/sse`.
- Keep all timeout values as named constants near the existing
  `ReadHeaderTimeout` for discoverability.

## Out of scope

- TLS termination (separate posture; see mcp-http-exposure-auth spec).
- Request-rate limiting / connection caps.
- Changing the existing body-size bound.

## Risks

- **SSE regression:** a naive `WriteTimeout` silently kills streams — covered by
  the Requirement 3 long-hold test.
- **Too-aggressive idle bound:** 60s default is conservative; make it a constant
  so it can be tuned without code archaeology.
