# Requirements — cmd-docs

> Rewrite documentation to describe only the optimized palette. The docs are the final gate: if they still mention dead commands, the optimization is incomplete. Depends on `cmd-mcp-sync` (the surface must be final before it is documented).

## Context

Affected docs:
- `docs/command-reference.md` — every command, flag, exit code.
- `docs/agent-integration.md` — MCP tools, Brain/Pinky orchestration.
- `docs/user-guide.md` — lifecycle and artifact formats.
- `README.md` / `AGENTS.md` — feature overview, agent workflow rules.

Each surviving command needs a single-sentence cheat-sheet description (`PROMPT.md §7`). Merged behaviors are documented as flags on survivors; deprecated commands appear only in a migration appendix.

## Requirements

### REQ-001 — Docs describe only survivors
**User story:** As a reader, I want the reference to list only live commands so that I never invoke a command that was merged or retired.

- THE SYSTEM SHALL list exactly the surviving palette in `docs/command-reference.md`.
- WHEN a command was merged THE SYSTEM SHALL document its behavior under the absorbing command's flags, not as a standalone entry.
- IF a deprecated command is referenced outside the migration appendix THEN THE SYSTEM SHALL fail the docs-lint check.
- THE SYSTEM SHALL give each surviving command one cheat-sheet-length description sentence.

**Rationale:** Documentation drift re-introduces the very surface bloat the suite removes; the reference must equal the registry.

### REQ-002 — Agent-integration reflects MCP parity
**User story:** As an agent integrator, I want `agent-integration.md` to match the parity-tested MCP surface so that documented tools actually exist.

- THE SYSTEM SHALL document only MCP tools that pass `TestCLIMCPParity`.
- THE SYSTEM SHALL document intent-tool→survivor-flag mappings.
- THE SYSTEM SHALL note that meta-hidden commands are excluded from the default tool list.

**Rationale:** Integration docs are the agent's contract; they must reflect the verified, not aspirational, surface.

### REQ-003 — Migration appendix
**User story:** As an existing user, I want a single appendix mapping old commands to new homes so that I can update scripts in one pass.

- THE SYSTEM SHALL provide an `old → new` migration table covering every merged and deprecated command.
- THE SYSTEM SHALL state the removal version for each deprecated command.
- WHERE a command became install-script-only THE SYSTEM SHALL document the script invocation.

**Rationale:** A consolidated migration table is what makes the breaking change adoptable rather than disruptive.

# Design — cmd-docs

## Overview
cmd-docs is the terminal documentation spec. It rewrites the command reference, agent-integration, user-guide, README, and AGENTS to describe only the optimized palette, and adds a migration appendix. It mutates Markdown only — no Go code. Input: the final registry (post-mcp-sync) and the audit ledger's old→new mapping.

## Architecture
Generate-then-lint: (1) regenerate the command-reference table from the live registry (single source of truth), (2) hand-write cheat-sheet sentences and the migration appendix, (3) run a docs-lint that greps for any dead command name outside the appendix and fails if found. The CHEATSHEET (owned at suite level) is cross-checked against the reference.

## Components and interfaces
- **Reference generator** — input: registry survivor list; output: `command-reference.md` command/flag/exit-code table.
- **Docs linter** — input: dead-command name list (from `audit.csv` merged+deprecated rows); scans all docs; fails on any out-of-appendix hit.
- **Migration appendix** — old→new table sourced from `audit.csv`.
- Interface: the linter's dead-command list is exactly the `audit.csv` rows where disposition ∈ {merge, deprecate}.

## Data models
No code/schema change. Docs artifacts only. Migration table schema: `old_command | new_home | removed_in | notes`. Cheat-sheet line schema: `command — one sentence`.

## Error handling
- Dead command name found outside appendix → docs-lint exits non-zero naming the file:line.
- Reference table lists a command absent from the registry → generator fails (registry is authoritative).
- Cheat-sheet count ≠ palette count → cross-check fails.

## Verification strategy
- `specd check cmd-docs` — gate spec artifacts.
- `grep -c 'specd <command>' docs/command-reference.md` and per-name grep for retired commands == 0 outside appendix.
- Docs-lint script over `docs/` and `README.md`/`AGENTS.md`.
- Cross-check CHEATSHEET palette count == reference command count.
- `specd verify cmd-docs <task>` — per-task evidence.

## Risks and open questions
- **Risk:** README/AGENTS contain prose mentioning merged commands in examples. Mitigation: linter scans them too; rewrite examples to survivor flags.
- **Risk:** The migration appendix itself contains dead command names, tripping the linter. Mitigation: linter whitelists the appendix section by anchor.
- **Open question:** Should the cheat-sheet live in docs or at `.specd/specs/CHEATSHEET.md`? Resolved: canonical copy at `.specd/specs/CHEATSHEET.md` (suite deliverable); docs link to it to avoid divergence.
