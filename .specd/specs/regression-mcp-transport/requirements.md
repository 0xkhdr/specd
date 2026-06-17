# Requirements — Regression: MCP Server + Transport (stdio/HTTP-SSE, tools, host compat)

## Introduction
The MCP layer (`internal/mcp`) is specd's agentic interface: a Model Context Protocol server
exposing ~40 tools over stdio and HTTP/SSE transports, with embedded host configs for Claude
Desktop, Cursor, VS Code, Codex, and Antigravity. This is the surface that determines specd's
value as an agent-harness tool. Regression here guarantees protocol compliance, tool-schema
stability, transport parity, and host compatibility. Value: specd is a drop-in, reliable MCP
server across every major agent host.

## Requirement 1 — MCP protocol compliance
**User story:** As an MCP host, I want specd to speak compliant MCP, so that handshake, tool
listing, and tool calls work without host-specific hacks.

**Acceptance criteria:**
1. WHEN a host initializes THE SYSTEM SHALL complete the MCP handshake and advertise its capabilities
2. WHEN a host lists tools THE SYSTEM SHALL return every tool with a valid JSON-Schema input definition
3. IF a tool call has invalid arguments THEN THE SYSTEM SHALL return a structured MCP error, not crash

## Requirement 2 — Tool-schema stability & parity with CLI
**User story:** As an agent, I want each MCP tool to mirror its CLI command, so that behavior
is identical whether invoked by CLI or MCP.

**Acceptance criteria:**
1. THE SYSTEM SHALL expose one MCP tool per supported CLI command with matching semantics
2. WHEN a tool is called THE SYSTEM SHALL return the same result the equivalent CLI invocation would
3. IF a tool's input schema changes THEN THE SYSTEM SHALL fail a regression test (golden schema)

## Requirement 3 — Transport parity (stdio ↔ HTTP/SSE)
**User story:** As an operator, I want stdio and HTTP/SSE transports to be behaviorally
identical, so that transport is purely an operational choice.

**Acceptance criteria:**
1. WHEN the same tool is called over stdio and over HTTP/SSE THE SYSTEM SHALL return equivalent results
2. WHERE HTTP/SSE is used THE SYSTEM SHALL stream events in MCP-compliant SSE framing
3. IF the HTTP transport receives a malformed request THEN THE SYSTEM SHALL respond with a proper HTTP + MCP error

## Requirement 4 — Host config compatibility & guards
**User story:** As a user wiring specd into a host, I want generated host configs to be valid
for each supported host, so that setup is copy-paste reliable.

**Acceptance criteria:**
1. THE SYSTEM SHALL emit a valid config snippet for each embedded host (Claude Desktop, Cursor, VS Code, Codex, Antigravity)
2. WHEN a guard precondition is violated THE SYSTEM SHALL block the operation with an actionable message
3. THE SYSTEM SHALL keep each embedded host config parseable in that host's native format (JSON/TOML)
