# Requirements — cmd-docs

## Introduction
Rewrite documentation to describe only the optimized command palette. The docs are the final gate: if they still mention dead commands, the optimization is incomplete. Depends on `cmd-mcp-sync` (the surface must be final before it is documented).

## Requirement 1 — Docs describe only survivors
**User story:** As a reader, I want the reference to list only live commands so that I never invoke a command that was merged or retired.

**Acceptance criteria:**
1. THE SYSTEM SHALL list exactly the surviving palette in `docs/command-reference.md`.
2. WHEN a command was merged THE SYSTEM SHALL document its behavior under the absorbing command's flags, not as a standalone entry.
3. IF a deprecated command is referenced outside the migration appendix THEN THE SYSTEM SHALL fail the docs-lint check.
4. THE SYSTEM SHALL give each surviving command one cheat-sheet-length description sentence.

## Requirement 2 — Agent-integration reflects MCP parity
**User story:** As an agent integrator, I want `agent-integration.md` to match the parity-tested MCP surface so that documented tools actually exist.

**Acceptance criteria:**
1. THE SYSTEM SHALL document only MCP tools that pass `TestCLIMCPParity`.
2. THE SYSTEM SHALL document intent-tool-to-survivor-flag mappings.
3. THE SYSTEM SHALL note that meta-hidden commands are excluded from the default tool list.

## Requirement 3 — Migration appendix
**User story:** As an existing user, I want a single appendix mapping old commands to new homes so that I can update scripts in one pass.

**Acceptance criteria:**
1. THE SYSTEM SHALL provide an `old -> new` migration table covering every merged and deprecated command.
2. THE SYSTEM SHALL state the removal version for each deprecated command.
3. WHERE a command became install-script-only THE SYSTEM SHALL document the script invocation.
