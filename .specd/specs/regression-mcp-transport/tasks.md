# Tasks — Regression: MCP Server + Transport (stdio/HTTP-SSE, tools, host compat)

## Wave 1
- [ ] T1 — Inventory MCP tools, transports, and host configs vs coverage
  - why: freeze the agent-facing contract only after fully listing it (R1-R4)
  - role: investigator
  - files: internal/mcp/tools.go, internal/mcp/server.go, internal/mcp/embed_hosts
  - contract: list every tool + input schema, each transport's covered tool set, each host config; mark gaps; do NOT edit
  - acceptance: tables {tool -> schema-golden?, CLI-parity?} and {host -> config-valid test?}
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4

## Wave 2
- [ ] T2 — Protocol compliance + tool-schema golden tests
  - why: R1, R2 — compliant handshake/list/call and drift-proof schemas
  - role: builder
  - files: internal/mcp/server_test.go, internal/mcp/tools_test.go
  - contract: test handshake/capabilities, tools/list schema validity, invalid-arg structured error; snapshot each tool input schema as golden
  - acceptance: R1.1-R1.3 and R2.1, R2.3 pass; schema drift fails a test
  - verify: go test ./internal/mcp/ -run 'Server|Tools|Schema'
  - depends: T1
  - requirements: 1, 2

- [ ] T3 — Transport parity stdio vs HTTP/SSE
  - why: R3 — transport must be an operational, not behavioral, choice
  - role: builder
  - files: internal/mcp/transport_test.go, internal/mcp/transport_http_test.go, internal/mcp/integration_test.go
  - contract: run one tool matrix over both transports; assert equivalent results, SSE framing, malformed-HTTP error; use ephemeral ports
  - acceptance: R3.1-R3.3 pass; no fixed-port flake
  - verify: go test ./internal/mcp/ -run 'Transport|Integration' -race
  - depends: T1
  - requirements: 3

- [ ] T4 — Host-config validity + guard tests
  - why: R4 — copy-paste-reliable host setup and safe guards
  - role: builder
  - files: internal/mcp/hosts.go, internal/mcp/guards_test.go
  - contract: parse each embedded host config in native format (JSON/TOML); validate generated snippets; assert guard blocks with actionable message
  - acceptance: R4.1-R4.3 pass for all five hosts
  - verify: go test ./internal/mcp/ -run 'Host|Guard'
  - depends: T1
  - requirements: 4

- [ ] T5 — CLI↔MCP semantic parity tests
  - why: R2.2 — a tool must match its CLI command's result
  - role: builder
  - files: internal/mcp/tools_test.go
  - contract: for a representative set of tools, invoke CLI and MCP via a shared fixture; assert equivalent results
  - acceptance: R2.2 passes for the chosen tool set; divergences flagged as bugs
  - verify: go test ./internal/mcp/ -run Parity
  - depends: T2
  - requirements: 2

## Wave 3
- [ ] T6 — Review MCP regression for protocol gaps and flake
  - why: a non-compliant or flaky MCP server destroys agent-harness value
  - role: reviewer
  - files: internal/mcp
  - contract: review T2-T5 for missing error paths, fixed ports, schema goldens not enforced, untested hosts; flag only
  - acceptance: all five hosts + both transports covered; every tool schema golden-locked
  - verify: go test ./internal/mcp/... -race -count=2
  - depends: T2, T3, T4, T5
  - requirements: 1, 2, 3, 4
