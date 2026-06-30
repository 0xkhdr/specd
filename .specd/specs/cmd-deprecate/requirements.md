# Requirements — cmd-deprecate

> Remove low-utility commands from the runtime palette. Unlike `cmd-merge`, these behaviors are not absorbed — they move to the install script, become hidden meta, or are retired with a grace-period alias. Consumes `cmd-audit/audit.csv`.

## Context

Deprecation targets (disposition `deprecate` or `meta-hidden` in the ledger):

| Command | Disposition | Resolution |
|---------|-------------|-----------|
| `update`    | deprecate | Install-script only; not a runtime command |
| `uninstall` | deprecate | Install-script only |
| `migrate`   | deprecate | One-time config migration; install-script / `init --migrate` |
| `mcp`       | meta-hidden | Kept, hidden from daily palette |
| `fusion`    | meta-hidden | Host bootstrap/policy oracle; kept, hidden |
| `version`   | meta-hidden | Kept meta |
| `help`      | meta-hidden | Kept meta |

`update`, `uninstall`, `migrate` leave the binary's runtime surface. `mcp`, `fusion`, `version`, `help` stay in the binary but are excluded from the daily cheat-sheet palette.

## Requirements

## Requirement 1 — Clean removal with migration path
**User story:** As a user who scripted a deprecated command, I want a clear migration message so that my automation fails loudly with a fix, not silently.

**Acceptance criteria:**

1. WHEN a deprecated command is invoked THE SYSTEM SHALL exit non-zero with a one-line message naming its replacement or new home.
2. WHILE a grace period is active THE SYSTEM SHALL keep the command functional but print a deprecation warning to stderr.
3. THE SYSTEM SHALL document each deprecation's removal version.
4. IF a deprecated command has no replacement THEN THE SYSTEM SHALL state it is install-script-only.

**Rationale:** Loud, instructive failure preserves trust while still shrinking the palette toward memorizability.

## Requirement 2 — Meta-hidden classification
**User story:** As an agent budgeting working memory, I want meta commands excluded from the active palette so that only workflow commands occupy the cheat sheet.

**Acceptance criteria:**

1. THE SYSTEM SHALL mark `mcp`, `fusion`, `version`, `help` as meta-hidden in the registry.
2. WHEN `help` lists commands without `--all` THE SYSTEM SHALL omit meta-hidden commands.
3. WHERE `--all` is passed THE SYSTEM SHALL include meta-hidden commands.
4. THE SYSTEM SHALL keep meta-hidden commands fully functional.

**Rationale:** Hiding non-workflow commands keeps the agent-containable palette at 16 without deleting host-integration capability.

## Requirement 3 — Palette ceiling enforcement
**User story:** As a maintainer, I want the post-deprecation count machine-checked so that the ≤20 survivor target cannot regress.

**Acceptance criteria:**

1. THE SYSTEM SHALL ensure the daily palette (non-meta-hidden survivors) is ≤16 commands.
2. THE SYSTEM SHALL ensure total binary commands (incl. meta-hidden) is ≤20.
3. IF either ceiling is exceeded THEN THE SYSTEM SHALL fail the palette-count test.

**Rationale:** A machine-checked ceiling is the only durable guarantee that future commits do not re-bloat the surface.
