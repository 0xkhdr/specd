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

### REQ-001 — Clean removal with migration path
**User story:** As a user who scripted a deprecated command, I want a clear migration message so that my automation fails loudly with a fix, not silently.

- WHEN a deprecated command is invoked THE SYSTEM SHALL exit non-zero with a one-line message naming its replacement or new home.
- WHILE a grace period is active THE SYSTEM SHALL keep the command functional but print a deprecation warning to stderr.
- THE SYSTEM SHALL document each deprecation's removal version.
- IF a deprecated command has no replacement THEN THE SYSTEM SHALL state it is install-script-only.

**Rationale:** Loud, instructive failure preserves trust while still shrinking the palette toward memorizability.

### REQ-002 — Meta-hidden classification
**User story:** As an agent budgeting working memory, I want meta commands excluded from the active palette so that only workflow commands occupy the cheat sheet.

- THE SYSTEM SHALL mark `mcp`, `fusion`, `version`, `help` as meta-hidden in the registry.
- WHEN `help` lists commands without `--all` THE SYSTEM SHALL omit meta-hidden commands.
- WHERE `--all` is passed THE SYSTEM SHALL include meta-hidden commands.
- THE SYSTEM SHALL keep meta-hidden commands fully functional.

**Rationale:** Hiding non-workflow commands keeps the agent-containable palette at 16 without deleting host-integration capability.

### REQ-003 — Palette ceiling enforcement
**User story:** As a maintainer, I want the post-deprecation count machine-checked so that the ≤20 survivor target cannot regress.

- THE SYSTEM SHALL ensure the daily palette (non-meta-hidden survivors) is ≤16 commands.
- THE SYSTEM SHALL ensure total binary commands (incl. meta-hidden) is ≤20.
- IF either ceiling is exceeded THEN THE SYSTEM SHALL fail the palette-count test.

**Rationale:** A machine-checked ceiling is the only durable guarantee that future commits do not re-bloat the surface.

# Design — cmd-deprecate

## Overview
cmd-deprecate retires three commands from the runtime (`update`, `uninstall`, `migrate`) and reclassifies four as meta-hidden (`mcp`, `fusion`, `version`, `help`). It changes registry visibility flags and removal scheduling; it does not absorb behavior into other commands (that is `cmd-merge`'s job). Input: `audit.csv` rows with disposition `deprecate` or `meta-hidden`.

## Architecture
Two mechanisms: (1) **retire** — remove from runtime registry, relocate logic to `scripts/install.sh` (update/uninstall) or behind `init --migrate` (migrate); leave a stub that exits with a migration hint during the grace window. (2) **hide** — add a `Hidden bool` attribute to `CommandMeta`; `help` and palette generation honor it. No behavior is reimplemented.

## Components and interfaces
- **Registry visibility editor** — adds/sets `Hidden` on `CommandMeta` for meta-hidden commands.
- **Deprecation stub** — for retired commands: a minimal handler printing the migration message and exiting non-zero.
- **Install-script relocation** — `scripts/install.sh` gains `update`/`uninstall` flows previously in the binary.
- **Palette generator** — filters `Hidden` commands; emits the daily palette consumed by `cmd-docs` CHEATSHEET.

## Data models
`CommandMeta` gains `Hidden bool` and `DeprecatedIn string` / `RemovedIn string` fields. No on-disk artifact schema changes. Palette manifest: ordered list of non-hidden survivor names.

## Error handling
- Invoking a retired command during grace → exit 1 + stderr warning naming the new home.
- Invoking a retired command after removal → exit 3 (not found) with the same hint, since the registry entry is gone.
- Mis-set `Hidden` causing a workflow command to vanish from `help` → caught by the palette-count test asserting the 16 expected names are present.

## Verification strategy
- `specd check cmd-deprecate` — gate spec artifacts.
- `go test ./internal/core/ -run TestPaletteCeiling` — assert ≤16 daily / ≤20 total.
- `specd help --json | jq '.commands | length'` vs `specd help --all --json` — confirm hidden exclusion.
- `specd update` (retired) exits non-zero with hint — `! specd update`.
- `specd verify cmd-deprecate <task>` — per-task evidence.

## Risks and open questions
- **Risk:** Removing `update` from the binary breaks users who self-update via the CLI. Mitigation: grace-period stub + install-script parity documented in `cmd-docs`.
- **Risk:** `fusion` is host-facing; hiding it from `help` may confuse MCP integrators. Mitigation: keep it in `help --all` and document under agent-integration, not the daily cheat sheet.
- **Open question:** Should `migrate` survive as `init --migrate` (a merge) rather than a deprecation? Resolved: it is a one-time, install-time concern → deprecate from daily runtime, expose via `init --migrate` for the rare upgrade path.
