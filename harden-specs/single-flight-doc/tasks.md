# Tasks — Single-Flight MCP Server Documentation (A9)

## Wave 1 — Code + docs
- [ ] T1 — Comment the single-flight mutex
  - why: intent must be explicit at call site (Req 1)
  - role: builder
  - files: internal/mcp/transport_http.go
  - contract: add comment at mutex stating intentional single-flight for
    determinism / single-agent model; deliberate ceiling, not a bug.
  - acceptance: comment present; no behavior change.
  - verify: go build ./... && go test ./internal/mcp/
  - depends: —
  - requirements: 1

- [ ] T2 — Document concurrency model in mcp-guide
  - why: hosts must not load-test as concurrent (Req 2)
  - role: builder
  - files: docs/mcp-guide.md
  - contract: add "Concurrency model" subsection: one in-flight request across
    /rpc and /sse; rationale = determinism, local-first single-agent.
  - acceptance: doc states the contract + rationale.
  - verify: N/A (doc review)
  - depends: —
  - requirements: 2
