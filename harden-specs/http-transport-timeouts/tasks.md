# Tasks — HTTP Transport Timeouts (A1)

## Wave 1 — Dashboard listener
- [x] T1 — Bound dashboard server time dimension
  - why: slow-read/idle clients pin goroutines (Req 1,2)
  - role: builder
  - files: internal/cmd/serve.go
  - contract: add `WriteTimeout` and `IdleTimeout` constants to the `http.Server`
    literal alongside existing `ReadHeaderTimeout`.
  - acceptance: server still serves dashboard; new timeouts set.
  - verify: go test ./internal/cmd/ -run Serve
  - depends: —
  - requirements: 1,2

## Wave 2 — MCP listener (SSE-aware)
- [x] T2 — Idle bound on MCP --http server
  - why: reclaim idle keep-alive connections (Req 1)
  - role: builder
  - files: internal/mcp/transport_http.go
  - contract: set `IdleTimeout` constant on the server; keep `ReadHeaderTimeout`.
  - acceptance: idle bound set; no global WriteTimeout that severs /sse.
  - verify: go test ./internal/mcp/ -run HTTP
  - depends: —
  - requirements: 1

- [x] T3 — Per-response /rpc write deadline, SSE exempt
  - why: bound /rpc write without killing /sse (Req 2,3)
  - role: builder
  - files: internal/mcp/transport_http.go
  - contract: apply a write deadline to /rpc handling via `http.ResponseController`
    (or split mux); ensure /sse path clears/extends its write deadline per event.
  - acceptance: /rpc bounded; /sse unbounded.
  - verify: go test ./internal/mcp/ -run "HTTP|SSE"
  - depends: T2
  - requirements: 2,3

## Wave 3 — Regression guards
- [x] T4 — Slowloris + SSE-longevity tests
  - why: prove the time dimension is bounded and SSE survives (Req 1,2,3)
  - role: verifier
  - files: internal/mcp/transport_http_test.go, internal/cmd/serve_test.go
  - contract: (a) slow-body/slow-read client is cut by the bound; (b) an /sse
    connection held longer than the /rpc WriteTimeout is NOT severed.
  - acceptance: tests fail if timeouts removed or if a global WriteTimeout cuts SSE.
  - verify: go test ./internal/mcp/ ./internal/cmd/ -run "Timeout|Slow|SSE"
  - depends: T1,T3
  - requirements: 1,2,3
