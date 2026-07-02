# S5 — MCP Server Regression

## 1. Purpose and requirement coverage

Guarantee the MCP server handles stdio + HTTP/SSE transports with correct
JSON-RPC framing, constant-time auth, and stable tool dispatch. Covers **R5**.

## 2. Verified current state

- Server core: `internal/mcp/server.go`; transports in `internal/mcp/transport.go`,
  `transport_http.go`. Auth tested by `transport_http_auth_test.go`; HTTP
  timeouts by `transport_http_timeout_test.go`.
- Tool registry + dispatch: `internal/mcp/tools.go`, `argschema.go`,
  `composite.go`; large integration surface in `integration_test.go`.
- Host capability negotiation: `hosts.go`, `negotiation.go`, `probe.go`,
  `host_compat_test.go`, `parity_test.go`. Fuzz already present:
  `host_caps_fuzz_test.go`.
- Phase watcher (SSE push): `internal/mcp/watcher.go`, `watcher_test.go`.
- `mcp` command entry is in `main.go:107` (pre-dispatch, stdio transport).
- Coverage floor `internal/mcp` = **88%** (`scripts/coverage-check.sh`).

## 3. Proposed design and end-to-end flow

Tests assert: JSON-RPC requests over stdio and HTTP produce well-framed
responses; the tool list is stable (names + arg schemas) for existing agents;
auth rejects bad tokens in constant time and accepts valid ones; SSE stream
emits phase events via the watcher; malformed frames yield JSON-RPC errors, not
panics. HTTP binds loopback by default and warns on non-loopback.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** MCP tool names + arg schemas (`tools.go`, `argschema.go`);
  JSON-RPC envelope shape; auth header contract.
- **Config:** bind address, auth token, TLS handled by reverse proxy (no
  built-in TLS — see F5).
- **Dependencies:** S1 (command routing for `mcp`).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Security:** auth comparison is constant-time; HTTP defaults to loopback and
  warns otherwise (`SECURITY.md`).
- **Errors:** malformed input → JSON-RPC error object, never a crash.
- **Compatibility:** tool list is an integration contract; additions only,
  no silent removals/renames.
- **Rollback:** additive tests.

## 6. Acceptance criteria and validation commands

- `go test ./internal/mcp/... -race -count=1` passes.
- `go test ./internal/mcp/... -run 'Auth|Transport|Tools|Watcher' -race` passes.
- `go test ./internal/mcp/... -count=2` stable.
- Tool-list parity test confirms no unintended tool add/remove.

## 7. Open decisions and deviations

- Deviation D6: fuzzing already exists here (`host_caps_fuzz_test.go`); S15
  extends fuzz to parsers, not to MCP host caps.
- F5 open: no built-in TLS; document reverse-proxy requirement (tracked under
  S14/security docs).
