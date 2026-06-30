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
