# Requirements — MCP Expansion for External Tools

## Introduction
specd's MCP server speaks JSON-RPC 2.0 over stdio with auto-detected framing (newline or
`Content-Length`). This feature extends first-class support to two additional MCP hosts —
**antigravity CLI** and **OpenAI Codex CLI** — by determining each tool's actual MCP client
capabilities, then integrating via the thinnest viable layer: a direct stdio config where the
host supports stdio MCP servers, or an opt-in HTTP/SSE transport adapter where it does not. The
guiding constraint is that specd's core (`internal/mcp`, `core.Commands`) must not be modified to
hardcode any host's assumptions; new transport, if needed, is additive and stdlib-only. Each
integration ships with a declarative config and a sandboxed integration test.

## Requirement 1 — antigravity CLI capability discovery
**User story:** As a maintainer, I want antigravity CLI's MCP client capabilities documented from
primary sources, so that the integration approach is evidence-based, not assumed.

**Acceptance criteria:**
1. THE SYSTEM SHALL record whether antigravity CLI supports stdio MCP servers, HTTP/SSE, or both
2. THE SYSTEM SHALL record antigravity's MCP config file location and schema with a citation
3. IF antigravity does not support stdio THEN THE SYSTEM SHALL record that an HTTP/SSE transport adapter is required

## Requirement 2 — Codex CLI capability discovery
**User story:** As a maintainer, I want OpenAI Codex CLI's MCP/tool mechanism documented from
primary sources, so that the integration approach is evidence-based.

**Acceptance criteria:**
1. THE SYSTEM SHALL record whether Codex CLI supports MCP servers natively or requires an adapter
2. THE SYSTEM SHALL record Codex's MCP server configuration syntax with a citation
3. IF Codex requires a non-stdio transport THEN THE SYSTEM SHALL record the required transport and its framing

## Requirement 3 — Capability-driven integration selection
**User story:** As an integrator, I want the integration path chosen by discovered capability, so
that we never ship a transport a host cannot use.

**Acceptance criteria:**
1. WHERE a host supports stdio MCP servers THE SYSTEM SHALL integrate via a declarative config that launches `specd mcp` with no new transport code
2. WHERE a host requires HTTP or SSE THE SYSTEM SHALL provide an opt-in transport adapter that wraps the existing stdio dispatch
3. THE SYSTEM SHALL discover specd's tools through `tools/list` rather than hardcoding any tool name on the host side

## Requirement 4 — Optional HTTP/SSE transport adapter
**User story:** As a developer of a non-stdio host, I want an HTTP/SSE bridge to specd's MCP
tools, so that I can reach specd without stdio.

**Acceptance criteria:**
1. WHERE the HTTP transport is enabled THE SYSTEM SHALL accept JSON-RPC requests over HTTP POST and route them through the same handler as stdio
2. THE SYSTEM SHALL bind the HTTP transport to loopback by default
3. IF the HTTP transport is not enabled THEN THE SYSTEM SHALL leave stdio behaviour byte-identical to today
4. THE SYSTEM SHALL keep the transport adapter stdlib-only with no third-party MCP SDK

## Requirement 5 — Declarative per-host configuration
**User story:** As a user, I want a copy-paste config for antigravity and Codex, so that I can
connect specd without reading source.

**Acceptance criteria:**
1. THE SYSTEM SHALL provide an antigravity CLI configuration artifact wiring specd as an MCP server
2. THE SYSTEM SHALL provide a Codex CLI configuration artifact wiring specd as an MCP server
3. IF a host config references a transport THEN THE SYSTEM SHALL match the transport chosen under Requirement 3

## Requirement 6 — Sandboxed integration tests
**User story:** As a maintainer, I want isolated tests for each integration, so that a host's
contract is verified without network flakiness.

**Acceptance criteria:**
1. WHEN an integration test runs THE SYSTEM SHALL use `internal/testharness` for a deterministic spec fixture
2. WHEN the HTTP adapter is exercised THE SYSTEM SHALL assert a `tools/list` over HTTP returns the same tools as stdio
3. THE SYSTEM SHALL keep all integration tests race-clean under `go test -race`

## Requirement 7 — Graceful degradation
**User story:** As a user on a host that lacks a specd feature, I want predictable fallback, so
that a missing capability does not corrupt state.

**Acceptance criteria:**
1. IF a host invokes a tool it cannot render the result of THEN THE SYSTEM SHALL still return a well-formed JSON-RPC result
2. THE SYSTEM SHALL preserve the existing per-call panic recovery so one bad host call never crashes the server
