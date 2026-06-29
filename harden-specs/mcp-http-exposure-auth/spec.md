# Spec — MCP `--http` Exposure & Auth (A2)

**Priority:** P1 · **Wave:** 1 · **Domain:** network-service auth/exposure.

## Introduction

`loopbackAddr()` defaults an empty/host-less address to loopback (good default),
but an explicit `--http 0.0.0.0:port` binds externally with **no auth token and
no TLS**. `SECURITY.md` documents the sandbox and config-precedence boundaries
but says nothing about the HTTP transport's auth posture. Anyone who binds the
MCP server to a non-loopback interface exposes full workflow control (dispatch,
phase transitions) unauthenticated.

This spec closes the gap with a layered response: document the posture, warn on
non-loopback binds, and add optional bearer-token auth.

## Current-state grounding

- `internal/mcp/transport_http.go:154-156` — `loopbackAddr` defaults
  empty/host-less addr to loopback.
- An explicit non-loopback `--http` bind is accepted with no auth and no TLS.
- `/rpc` exposes dispatch + phase transitions; `/sse` exposes the event stream.
- `SECURITY.md` — covers sandbox + config precedence; no HTTP auth section.
- `docs/mcp-guide.md` — MCP transport docs; no exposure/auth note today.

## Requirements

### Requirement 1 — Document the exposure posture (minimum, ships now)
**User story:** As an operator, I want the supported exposure model stated, so I
do not bind externally believing it is safe.

**Acceptance criteria:**
1. `SECURITY.md` SHALL state `--http` is loopback-by-design and binding
   non-loopback is at operator risk (and what that risk is: unauthenticated
   workflow control).
2. `docs/mcp-guide.md` SHALL mirror the posture with a concrete example.

### Requirement 2 — Bind-guard warning on non-loopback
**User story:** As an operator, I want a loud warning when I bind externally, so
an accidental `0.0.0.0` bind is not silent.

**Acceptance criteria:**
1. When `--http` resolves to a non-loopback address AND no auth token is set,
   the server SHALL emit a clear stderr warning at startup.
2. The warning SHALL name the risk and the mitigation (set `SPECD_MCP_TOKEN`).
3. Loopback binds SHALL NOT warn.

### Requirement 3 — Optional bearer-token auth
**User story:** As an operator, I want to require a token on external binds, so
the surface is not open by default when exposed.

**Acceptance criteria:**
1. When `SPECD_MCP_TOKEN` is set, `/rpc` AND `/sse` SHALL require a matching
   `Authorization: Bearer <token>` header.
2. Missing/incorrect token SHALL return `401` and SHALL NOT dispatch.
3. Token comparison SHALL be constant-time.
4. When `SPECD_MCP_TOKEN` is unset, behavior SHALL be unchanged (back-compat for
   the loopback default path).

## Design

- Add a `tokenAuth` middleware wrapping both `/rpc` and `/sse` handlers; no-op
  when `SPECD_MCP_TOKEN` is empty; `crypto/subtle.ConstantTimeCompare` otherwise.
- At startup, classify the resolved bind via the existing loopback logic; if
  non-loopback and token empty, print the warning.
- Read the token from env via the existing config/env plumbing (do not log it).
- Keep TLS out of scope here — document termination via a reverse proxy.

## Out of scope

- Built-in TLS (recommend reverse-proxy termination instead).
- Per-method authorization / RBAC.
- Changing the loopback default.

## Risks

- **Token leakage in logs:** never log the token or the `Authorization` header;
  add a test asserting the warning/diagnostics never echo the token value.
- **Back-compat break:** unset token MUST preserve current behavior — covered by
  Req 3.4 test.
