# Tasks — Host and MCP Adherence Protocol

## Wave 1 — MCP fusion exposure
- [ ] T1 — Mark fusion as read-only MCP tool
  - why: hosts need bootstrap/policy through MCP (Req 2)
  - role: builder
  - files: internal/mcp/tools.go, internal/mcp/tools_test.go
  - contract: once the `fusion` command exists, expose it as `specd_fusion` with readOnlyHint true and no destructive hint.
  - acceptance: tools/list includes `specd_fusion`; annotations are correct.
  - verify: go test ./internal/mcp/ -run Tools
  - depends: session-bootstrap/T3
  - requirements: 2

- [ ] T2 — Decide essential exposure for fusion
  - why: startup should be discoverable with small tool lists (Req 2)
  - role: builder
  - files: internal/mcp/tools.go, internal/mcp/tools_test.go, docs/mcp-guide.md
  - contract: either include `specd_fusion` in default essential tools or document an explicit essentialTools config fallback; tests freeze the choice.
  - acceptance: essential mode startup path is unambiguous and tested.
  - verify: go test ./internal/mcp/ -run Essential
  - depends: T1
  - requirements: 2

## Wave 2 — Startup and playbook instructions
- [ ] T3 — Update MCP server instructions
  - why: models see the adherence protocol at initialization (Req 1)
  - role: builder
  - files: internal/mcp/server.go, internal/mcp/server_test.go
  - contract: mention fusion bootstrap/policy first; provide fallback to status/context/help schema; keep message concise.
  - acceptance: server instruction tests assert the startup keywords without large prose.
  - verify: go test ./internal/mcp/ -run Server
  - depends: T1
  - requirements: 1

- [ ] T4 — Document delegate-mode host protocol
  - why: subagentMode is binding configuration (Req 3)
  - role: builder
  - files: internal/core/embed_templates/AGENTS.md, docs/agent-integration.md, docs/mcp-guide.md
  - contract: state delegate behavior for base `dispatch --json` and orchestrated Brain/Pinky missions; require explicit inline fallback warning when host lacks subagents.
  - acceptance: docs include exact base and orchestrated delegate sequences.
  - verify: N/A
  - depends: configuration-mode-sentinel/T4
  - requirements: 3

- [ ] T5 — Document Brain decision playbook
  - why: hosts need deterministic handling for every decision (Req 4)
  - role: builder
  - files: docs/agent-integration.md, internal/core/embed_templates/AGENTS.md
  - contract: list handling for dispatch/wait/awaiting-approval/escalate/policy-violation/complete-session and Pinky lifecycle with verify-ref requirement.
  - acceptance: docs distinguish proof (`verification-ref`) from telemetry (`tokens`, `cost`, `duration`).
  - verify: N/A
  - depends: T4
  - requirements: 4

## Wave 3 — Phase exposure hardening
- [ ] T6 — Add phase-compatible MCP tests
  - why: tool surface should not invite wrong commands (Req 5)
  - role: verifier
  - files: internal/mcp/tools_test.go
  - contract: assert planning excludes execution mutations; executing includes drive-loop tools; orchestration include/exclude gating works.
  - acceptance: tests fail if future tool-list changes reintroduce phase-incompatible defaults.
  - verify: go test ./internal/mcp/ -run Phase
  - depends: T1
  - requirements: 5

- [ ] T7 — Align docs with command schema guardrails
  - why: host and shell flows should share schema-before-syntax (Req 1,2)
  - role: builder
  - files: docs/agent-integration.md, docs/mcp-guide.md
  - contract: point MCP hosts at enriched schemas/enums and shell hosts at `specd help <command> --json`.
  - acceptance: docs contain one shared command-discovery protocol.
  - verify: N/A
  - depends: command-schema-guardrails/T3, command-schema-guardrails/T5
  - requirements: 1,2
