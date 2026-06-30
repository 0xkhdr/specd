# Requirements — cmd-mcp-sync

## Introduction
Align the MCP tool surface with the optimized CLI so MCP clients see the same survivor set, absorbed flags, and orchestration intent routes as command-line users.

## Requirement 1 — 1:1 surface parity
**User story:** As an MCP client, I want the advertised tool list to mirror the optimized CLI, so that I never call a removed command or miss a survivor.

**Acceptance criteria:**
1. THE SYSTEM SHALL expose exactly one MCP command-mirror tool per surviving CLI command.
2. WHEN a command is merged in the CLI THE SYSTEM SHALL remove its standalone MCP tool and expose the behavior via the survivor tool's arguments.
3. IF an MCP command-mirror tool references a removed CLI command THEN THE SYSTEM SHALL fail the parity test with the symmetric difference.
4. THE SYSTEM SHALL exclude hidden and meta-hidden commands from the default advertised MCP tool list.

## Requirement 2 — Intent-tool remapping
**User story:** As an agent using intent-level tools, I want them routed to consolidated flags, so that orchestration tools stay stable across the merge.

**Acceptance criteria:**
1. THE SYSTEM SHALL map `brain_orchestrate` onto `brain start --auto-step`.
2. THE SYSTEM SHALL map `brain_status` onto `brain status --verbose` by default and `brain status --ledger` when the ledger view is requested.
3. THE SYSTEM SHALL keep `brain_approve`, `brain_pause`, `brain_cancel`, and `brain_resume` routed to surviving CLI paths.
4. IF an intent tool references a removed CLI command THEN THE SYSTEM SHALL fail the intent-alias resolution test.

## Requirement 3 — Parity is test-enforced
**User story:** As a maintainer, I want CLI↔MCP parity asserted in CI, so that future CLI edits cannot silently desync the MCP surface.

**Acceptance criteria:**
1. THE SYSTEM SHALL provide a test enumerating CLI survivors and MCP command-mirror tools and asserting equality modulo hidden meta commands.
2. IF the sets differ THEN THE SYSTEM SHALL fail with the symmetric difference listed.
3. THE SYSTEM SHALL run the parity test without network access or nondeterministic fixtures.
