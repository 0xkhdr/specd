# Tasks — MCP `--http` Exposure & Auth (A2)

## Wave 1 — Documentation (ships independently)
- [x] T1 — Document exposure posture
  - why: state the supported model (Req 1)
  - role: builder
  - files: SECURITY.md, docs/mcp-guide.md
  - contract: add an HTTP-transport section: loopback-by-design, non-loopback =
    operator risk (unauthenticated workflow control), mitigation = SPECD_MCP_TOKEN
    + reverse-proxy TLS.
  - acceptance: both docs describe risk and mitigation with an example.
  - verify: N/A (doc review)
  - depends: —
  - requirements: 1

## Wave 2 — Bind guard
- [x] T2 — Non-loopback startup warning
  - why: no silent external bind (Req 2)
  - role: builder
  - files: internal/mcp/transport_http.go
  - contract: reuse loopback classification; if non-loopback and token unset,
    print stderr warning naming risk + mitigation.
  - acceptance: loopback silent; non-loopback+no-token warns.
  - verify: go test ./internal/mcp/ -run "Bind|Warn"
  - depends: —
  - requirements: 2

## Wave 3 — Bearer-token auth
- [x] T3 — Token middleware on /rpc and /sse
  - why: require auth when exposed (Req 3)
  - role: builder
  - files: internal/mcp/transport_http.go
  - contract: middleware no-op when SPECD_MCP_TOKEN empty; else require
    `Authorization: Bearer <token>`, constant-time compare, 401 on mismatch, no
    dispatch on failure.
  - acceptance: set token → 401 without header; correct token → 200; unset →
    unchanged.
  - verify: go test ./internal/mcp/ -run "Auth|Token"
  - depends: T2
  - requirements: 3

- [x] T4 — Auth + no-leak regression tests
  - why: lock the auth contract and token hygiene (Req 2,3)
  - role: verifier
  - files: internal/mcp/transport_http_test.go
  - contract: assert 401/200 matrix; assert warning + diagnostics never echo the
    token value; assert unset-token back-compat.
  - acceptance: tests fail if auth bypassed or token logged.
  - verify: go test ./internal/mcp/ -run "Auth|Token|Bind"
  - depends: T3
  - requirements: 2,3
