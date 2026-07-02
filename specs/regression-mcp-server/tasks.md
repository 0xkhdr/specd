# S5 Tasks — MCP Server Regression

Requirement coverage: R5. Dependencies: S1.

## Wave 1 — Baseline (after S1 green)

- [ ] Snapshot the current tool list (names + arg schemas) as a golden fixture.
  Files: `internal/mcp/tools.go`, `internal/mcp/testdata/`.
- [ ] Inventory transport/auth coverage in `transport_http_auth_test.go`,
  `transport_test.go`; note gaps.
- **Validation:** `go test ./internal/mcp/... -race -count=1`

## Wave 2 — Core regression tests (depends on Wave 1)

- [x] JSON-RPC framing over stdio and HTTP: valid request → well-formed result.
  File: `internal/mcp/transport_test.go` (extend).
- [x] Constant-time auth: bad token rejected, valid accepted. File:
  `internal/mcp/transport_http_auth_test.go` (extend).
- [x] Tool-list parity: assert against golden fixture (add/remove fails). File:
  `internal/mcp/parity_test.go` (extend).
- [x] Malformed frame → JSON-RPC error, no panic. File:
  `internal/mcp/transport_http_test.go` (extend).
- **Validation:** `go test ./internal/mcp/... -run 'Auth|Transport|Parity|Tools' -race -count=1` ✅

## Wave 3 — SSE & determinism (depends on Wave 2)

- [ ] Phase-watcher SSE emits events for a state transition. File:
  `internal/mcp/watcher_test.go` (extend).
- [ ] Loopback-default + non-loopback warning assertion. File:
  `internal/mcp/transport_http_test.go`.
- [ ] Run `-count=2` for stability.
- **Validation:** `go test ./internal/mcp/... -count=2`

## Rollout & cleanup

- [ ] Confirm `internal/mcp` ≥88% (`make cover-check`).
- **Rollback:** revert extensions; keep golden fixture under version control.
- **Completion evidence:** green MCP suite + parity fixture.
