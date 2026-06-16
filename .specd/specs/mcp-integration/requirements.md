# Requirements — MCP Deep-Dive & Real-World Integration

## Introduction
specd ships a native MCP (Model Context Protocol) server (`specd mcp`) that exposes every
non-meta CLI command as a JSON-RPC 2.0 tool over stdio. Today the server is functionally
complete but under-documented: a developer cannot wire specd into Claude Desktop, Cursor, or a
VS Code MCP client without reverse-engineering `internal/mcp/`. This feature turns the MCP
server into a documented, copy-paste-integrable component, with verified host configs and a gap
analysis of what the transport does and does not yet support. No protocol behaviour changes —
the deliverable is accurate documentation, configuration artifacts, and an integration smoke
test that pins the wire contract.

## Requirement 1 — Startup & invocation contract
**User story:** As a developer wiring specd into an MCP host, I want a precise description of how
the server starts and scopes itself, so that I can launch it correctly from a host config.

**Acceptance criteria:**
1. THE SYSTEM SHALL document that `specd mcp` starts a JSON-RPC 2.0 stdio server that runs until stdin closes
2. WHEN `specd mcp --root <dir>` is invoked THE SYSTEM SHALL document that every tool call is scoped to that project via a one-time chdir
3. IF the `--root` path cannot be entered THEN THE SYSTEM SHALL document that the server exits with usage code 2
4. THE SYSTEM SHALL document that only protocol bytes appear on stdout and all diagnostics go to stderr

## Requirement 2 — Wire framing contract
**User story:** As a host integrator, I want the exact stdio framing pinned, so that my client can
talk to specd without guessing the delimiter style.

**Acceptance criteria:**
1. THE SYSTEM SHALL document that the server auto-detects newline-delimited JSON framing from the first non-whitespace byte
2. WHERE the first byte is `C` THE SYSTEM SHALL document that LSP-style `Content-Length` header framing is selected for the connection lifetime
3. THE SYSTEM SHALL document the protocol version string `2024-11-05` advertised in the `initialize` result
4. IF a request is malformed THEN THE SYSTEM SHALL document that the server replies with a JSON-RPC error and keeps reading

## Requirement 3 — Tool surface & schemas
**User story:** As an agent author, I want the exposed tool list and its schemas described, so that
I know which specd capabilities are reachable and how to call them.

**Acceptance criteria:**
1. THE SYSTEM SHALL document that every `core.Commands` entry except `help`, `version`, and `mcp` is exposed as a tool named `specd_<command>`
2. THE SYSTEM SHALL document that each tool takes an ordered `args` string array for positionals plus one typed property per command flag
3. THE SYSTEM SHALL document the `readOnlyHint` and `destructiveHint` annotations and which commands carry them
4. WHEN a new command is added to `core.Commands` THE SYSTEM SHALL document that it surfaces as a tool automatically with no separate registration

## Requirement 4 — Host configuration examples
**User story:** As a developer, I want copy-paste configs for the major MCP hosts, so that I can
connect specd in under five minutes.

**Acceptance criteria:**
1. THE SYSTEM SHALL provide a Claude Desktop `mcpServers` configuration block that launches `specd mcp --root <project>`
2. THE SYSTEM SHALL provide a Cursor MCP configuration example
3. THE SYSTEM SHALL provide a generic VS Code MCP client configuration example
4. IF a host requires `Content-Length` framing THEN THE SYSTEM SHALL document that no specd-side change is needed because framing auto-detects

## Requirement 5 — Limitations & gap analysis
**User story:** As an adopter, I want an honest list of current limitations, so that I can plan
around them instead of hitting them in production.

**Acceptance criteria:**
1. THE SYSTEM SHALL document that only stdio transport exists today, with no HTTP or SSE transport
2. THE SYSTEM SHALL document that MCP resources and prompts are not yet implemented and only the `tools` capability is advertised
3. THE SYSTEM SHALL document that `listChanged` is false and the tool list is static per process

## Requirement 6 — Integration smoke test
**User story:** As a maintainer, I want an automated test that exercises the MCP wire contract, so
that documentation can never silently drift from behaviour.

**Acceptance criteria:**
1. WHEN the smoke test sends an `initialize` request THE SYSTEM SHALL assert the response advertises protocol version `2024-11-05` and the `tools` capability
2. WHEN the smoke test sends a `tools/list` request THE SYSTEM SHALL assert `specd_status` is present with its annotations
3. WHEN the smoke test sends a `tools/call` for `specd_status` against a real spec THE SYSTEM SHALL assert the result carries `structuredContent`
4. IF an unknown tool name is called THEN THE SYSTEM SHALL assert the server returns an invalid-params error rather than crashing
