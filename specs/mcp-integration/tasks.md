# Tasks — MCP Orchestration Surface

## Wave 14 — Generated MCP Tool Surface

- [ ] T1 — Expose Brain and Pinky through command metadata
  - why: MCP tools must remain generated from the canonical CLI registry.
  - role: builder
  - files: internal/core/commands.go, internal/mcp/tools.go, internal/mcp/tools_test.go, internal/mcp/testdata/tool_schemas.golden.json
  - contract: Describe orchestration subcommands/flags, mutation annotations, and bounded schemas without hand-registering MCP-only tools.
  - acceptance: Tool discovery is deterministic; registry/help/MCP parity passes; existing tools do not disappear.
  - verify: go test ./internal/mcp/... ./internal/cmd/... -run 'Test.*Tool|TestRegistryMatchesHelp' -count=2
  - depends: brain-core/T6, pinky-core/T5, program-orchestration/T4
  - requirements: R5.1, R5.2, R5.4, R5.11

## Wave 15 — CLI/MCP Parity

- [ ] T2 — Add orchestration CLI/MCP parity tests
  - why: Transport-specific business logic would undermine specd's architecture.
  - role: verifier
  - files: internal/mcp/integration_test.go, internal/mcp/server_test.go
  - contract: Compare CLI JSON with MCP results for status and mutations, including exit errors, invalid IDs, approvals, evidence failures, and cancellation.
  - acceptance: Equivalent invocations return equivalent structured payloads and error semantics.
  - verify: go test ./internal/mcp/... -run 'Test.*(CLI.*MCP.*Parity|Orchestration)' -count=2
  - depends: T1
  - requirements: R5.3, R5.5, R5.6, R5.7

## Wave 16 — Bounded MCP Interactions

- [ ] T3 — Harden bounded session interactions and server instructions
  - why: MCP requests must not block indefinitely or misstate host-control capabilities.
  - role: builder
  - files: internal/mcp/server.go, internal/mcp/transport.go, internal/mcp/transport_http.go, internal/mcp/server_test.go
  - contract: Update instructions and enforce bounded start/step/status/watch behavior, payload limits, approval/verify trust boundaries, and cooperative cancellation wording.
  - acceptance: No request waits for a worker; malformed/oversized calls return standard MCP errors; stdio and HTTP/SSE stay equivalent.
  - verify: go test ./internal/mcp/... -run 'TestMCP.*(Bounded|Instructions|HTTP|SSE|Malformed)' -count=2
  - depends: T2
  - requirements: R5.8, R5.9, R5.10, R5.11

## Wave 17 — Host Compatibility

- [ ] T4 — Extend doctor and compatibility evidence
  - why: Supported hosts need verifiable orchestration capability without provider-specific integration.
  - role: builder
  - files: internal/mcp/probe.go, internal/cmd/doctor.go, docs/agent-harness-compat.md, internal/mcp/probe_test.go
  - contract: Probe discovery of Brain/Pinky tools and report host reload/trust requirements without claiming agent-spawn control.
  - acceptance: Doctor distinguishes server capability from host lifecycle support and preserves project-scoped safety.
  - verify: go test ./internal/mcp/... ./internal/cmd/... -run 'Test.*(Probe|Doctor|Compatibility)' -count=2
  - depends: T3
  - requirements: R5.9, R5.12

## Wave 18 — MCP End-to-End

- [ ] T5 — Add stdio and HTTP/SSE orchestration lifecycle tests
  - why: The public remote-driving path must prove the full contract.
  - role: verifier
  - files: internal/mcp/orchestration_integration_test.go
  - contract: Drive start, status, claim, heartbeat, report, Brain step, pause/resume, blocker, retry, and cancellation over both transports using the fake worker.
  - acceptance: Both transports converge to identical state with no deadlock, bypass, race, or unbounded response.
  - verify: go test ./internal/mcp/... -race -count=2
  - depends: T4, brain-core/T8
  - requirements: R5.3, R5.5, R5.6, R5.8, R5.11

## Wave 19 — Documentation and Full Gate

- [ ] T6 — Update MCP and agent integration documentation
  - why: Users need an accurate workflow and trust model.
  - role: builder
  - files: docs/mcp-guide.md, docs/agent-integration.md, docs/troubleshooting.md, docs/command-reference.md
  - contract: Document generated tools, bounded polling/stepping, host worker lifecycle, approvals, evidence, cancellation, and recovery.
  - acceptance: Examples use `specd_brain`/`specd_pinky`, never promise embedded LLM execution, and state exact host responsibilities.
  - verify: make ci
  - depends: T5, config-extension/T4
  - requirements: R5.1, R5.8, R5.9, R5.10, R5.12
