# Requirements — cmd-mcp-sync

> Align the MCP tool surface with the optimized CLI. Because `specd mcp` exposes every CLI command 1:1, the merges and deprecations must propagate automatically — and be proven to. Depends on `cmd-merge` and `cmd-deprecate`.

## Context

Per the analysis document §3.2, the MCP layer is a thin 1:1 wrapper over the CLI registry; there is no separate API surface. Therefore:
- Merged commands must disappear as standalone MCP tools and reappear as parameters of the survivor tool.
- Deprecated commands must drop from the MCP tool list (retired) or be hidden (meta-hidden).
- Intent-level tools (`brain_orchestrate`, `brain_status`, …) must map onto the consolidated CLI flags.

## Requirements

### REQ-001 — 1:1 surface parity
**User story:** As an MCP client, I want the tool list to exactly mirror the optimized CLI so that I never call a tool that no longer exists or miss one that does.

- THE SYSTEM SHALL expose exactly one MCP tool per surviving CLI command.
- WHEN a command is merged in the CLI THE SYSTEM SHALL remove its standalone MCP tool and expose the behavior via the survivor tool's arguments.
- IF an MCP tool references a removed CLI command THEN THE SYSTEM SHALL fail the parity test.
- THE SYSTEM SHALL exclude meta-hidden commands from the default advertised tool list.

**Rationale:** Parity is what lets the "the CLI is the canonical interface" invariant hold; drift would force separate MCP docs, doubling the surface.

### REQ-002 — Intent-tool remapping
**User story:** As an agent using intent-level tools, I want them routed to consolidated flags so that orchestration tools stay stable across the merge.

- THE SYSTEM SHALL map `brain_orchestrate` onto `brain start` (incl. `--auto-step`).
- THE SYSTEM SHALL map `brain_status` onto `brain status` (incl. `--verbose`/`--ledger`).
- THE SYSTEM SHALL keep `brain_approve|pause|cancel|resume` routed to their surviving CLI paths.
- WHERE an intent tool referenced a now-merged subcommand THE SYSTEM SHALL route it to the absorbing flag.

**Rationale:** Intent tools are the recommended agent entry points; remapping them preserves the orchestration UX through consolidation.

### REQ-003 — Parity is test-enforced
**User story:** As a maintainer, I want CLI↔MCP parity asserted in CI so that future CLI edits cannot silently desync the MCP surface.

- THE SYSTEM SHALL provide a test enumerating CLI survivors and MCP tools and asserting equality (modulo meta-hidden).
- IF the sets differ THEN THE SYSTEM SHALL fail with the symmetric difference listed.

**Rationale:** Automated parity is the mechanism that keeps a single canonical interface from quietly forking.

# Design — cmd-mcp-sync

## Overview
cmd-mcp-sync ensures the MCP server advertises a tool surface identical to the optimized CLI survivor set. Since the MCP layer is generated from the same `CommandMeta` registry, the primary work is verifying generation honors `Hidden`, merged flags, and removed entries — plus remapping intent-level tools. Input: the post-`cmd-merge`/`cmd-deprecate` registry.

## Architecture
The MCP tool generator iterates `CommandMeta`. After merge/deprecate, it must (a) skip `Hidden` and retired entries, (b) surface merged flags as tool input-schema properties on the survivor, (c) resolve intent-tool aliases to survivor handlers. A parity test compares the generated tool list against the CLI survivor list.

## Components and interfaces
- **MCP tool generator** — `internal/mcp/*`; input: `CommandMeta` registry; output: advertised tool list + input schemas.
- **Intent-alias table** — maps `brain_*`/`pinky_*` intent names to survivor CLI invocations with pre-filled flags.
- **Parity test** — `internal/mcp/parity_test.go`; asserts `set(MCP tools) == set(CLI survivors not Hidden)`.

## Data models
No new on-disk model. MCP tool descriptor: `{name, cli_command, input_schema}`. Input schema for a survivor gains properties for each absorbed flag (e.g. `report` tool schema gains `serve`, `watch`, `history`, `diff`).

## Error handling
- Generator encounters a merged source name → skip + log; never advertise a dead tool.
- Intent alias targets a removed command → fail generation loudly (build-time test), not at runtime.
- Client calls a retired tool → MCP returns method-not-found mirroring CLI exit 3 semantics.

## Verification strategy
- `specd check cmd-mcp-sync` — gate spec artifacts.
- `go test ./internal/mcp/ -run TestCLIMCPParity` — set equality.
- `specd mcp --http :0 &` then list tools; assert `report` schema includes absorbed flags and `serve`/`watch` tools are absent.
- `specd verify cmd-mcp-sync <task>` — per-task evidence.

## Risks and open questions
- **Risk:** Intent tools have hand-written schemas that won't auto-update from the registry. Mitigation: route them through the alias table and assert in `TestCLIMCPParity` that each alias resolves to a survivor.
- **Risk:** HTTP/SSE transport caches an old tool list. Mitigation: regenerate on server start; parity test runs against a fresh process.
- **Open question:** Should meta-hidden `fusion` be advertised over MCP at all? Resolved: yes for host bootstrap, but excluded from the default list and only returned under an explicit capability query — documented in `cmd-docs`.
