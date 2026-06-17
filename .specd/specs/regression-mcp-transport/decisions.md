# Decisions — Regression: MCP Server + Transport (stdio/HTTP-SSE, tools, host compat)

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-17
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — T5 CLI↔MCP parity test placed in package mcp_test (integration_test.go), not the internal package mcp tools_test.go named in the task contract. Reason: internal/cmd imports internal/mcp, so an internal (package mcp) test importing cmd.Dispatch — required to invoke the real CLI dispatch path — would create an import cycle and fail to compile. The external test package mcp_test (already used by integration_test.go and transport_http_test.go) is the only place that can import cmd. No production code changed. · 2026-06-17
**Context:** TODO
**Decision:** T5 CLI↔MCP parity test placed in package mcp_test (integration_test.go), not the internal package mcp tools_test.go named in the task contract. Reason: internal/cmd imports internal/mcp, so an internal (package mcp) test importing cmd.Dispatch — required to invoke the real CLI dispatch path — would create an import cycle and fail to compile. The external test package mcp_test (already used by integration_test.go and transport_http_test.go) is the only place that can import cmd. No production code changed.
**Consequences:** TODO
**Supersedes:** —
