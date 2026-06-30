# Requirements — cmd-audit

> Measure the full command surface before any subtraction. This spec produces the canonical, evidence-backed inventory that `cmd-merge` and `cmd-deprecate` consume.

## Context

The specd CLI exposes **33 top-level commands** (verified against `internal/core/commands.go`):

```
init doctor migrate fusion new approve decision midreq memory next dispatch
verify task status mode check context validate schema report serve replay
diff watch waves brain pinky program update uninstall version mcp help
```

The optimization target is an active palette of **≤20 surviving commands** (16 workflow + 4 meta). No subtraction may proceed without a per-command audit row justifying keep/merge/deprecate against the decision matrix in `PROMPT.md §5`.

## Requirements

### REQ-001 — Complete command inventory
**User story:** As a maintainer, I want a machine-readable inventory of every command and its attributes so that no command is silently dropped or kept without justification.

- THE SYSTEM SHALL enumerate every top-level command present in `internal/core/commands.go`.
- WHEN a command exposes subcommands THE SYSTEM SHALL record each subcommand as a distinct audit row.
- THE SYSTEM SHALL record, per row: command, category, mutates_state (bool), phase_gate (bool), unique_exit_contract (bool), user_facing (bool), agent_facing (bool).
- IF a command in the registry is absent from `docs/command-reference.md` THEN THE SYSTEM SHALL flag it as `undocumented`.

**Rationale:** A minimal palette is only defensible if the starting set is measured exhaustively; the inventory is the denominator for the ≤20 target.

### REQ-002 — Overlap scoring
**User story:** As a maintainer, I want each command scored for functional overlap so that merge candidates are identified by evidence, not intuition.

- THE SYSTEM SHALL assign each command an `overlap_with` field naming any broader command that could subsume it, or `none`.
- THE SYSTEM SHALL assign a `disposition` of one of: `keep`, `merge`, `deprecate`, `meta-hidden`.
- WHERE two commands share a non-empty `overlap_with` target THE SYSTEM SHALL mark the narrower one `merge`.
- THE SYSTEM SHALL classify the 12 non-negotiable commands from `PROMPT.md §5.1` as `keep`.

**Rationale:** Overlap scoring is the mechanism that drives the surface from 33 toward 20 without removing backbone commands.

### REQ-003 — Disposition ledger drives downstream specs
**User story:** As the author of `cmd-merge` and `cmd-deprecate`, I want a single source-of-truth ledger so that downstream specs do not re-derive dispositions and diverge.

- THE SYSTEM SHALL emit the audit as `.specd/specs/cmd-audit/audit.csv` with one row per command/subcommand.
- THE SYSTEM SHALL produce a summary asserting the surviving-count is ≤20.
- IF the surviving-count exceeds 20 THEN THE SYSTEM SHALL list the overflow commands requiring further merge.

**Rationale:** A shared ledger is the single artifact preventing `cmd-merge` and `cmd-deprecate` from contradicting each other.

# Design — cmd-audit

## Overview
cmd-audit is a read-only analysis spec. It inventories the live command registry, scores each command for overlap and disposition, and emits a CSV ledger plus a markdown summary. It mutates no Go code; its only outputs are analysis artifacts under `.specd/specs/cmd-audit/`. Every other spec in this suite consumes `audit.csv` as its input contract.

## Architecture
Single-pass pipeline:
1. **Extract** — parse `internal/core/commands.go` for the `CommandMeta` registry; cross-reference `docs/command-reference.md`.
2. **Classify** — apply the `PROMPT.md §5` decision matrix to each row, seeding `keep` for the §5.1 non-negotiables.
3. **Score** — compute `overlap_with` from the §5.2 merge map and the analysis document.
4. **Emit** — write `audit.csv` + `audit-summary.md`; assert ≤20 survivors.

No runtime code path is touched; the audit reads source as data.

## Components and interfaces
- **Registry extractor** — input: `internal/core/commands.go`; output: list of `{command, subcommand, category, flags}`.
- **Doc cross-referencer** — input: `docs/command-reference.md`; output: documented-command set for the `undocumented` flag.
- **Disposition classifier** — input: extracted rows + §5.1/§5.2 tables; output: `{disposition, overlap_with, rationale}` per row.
- **Ledger writer** — output contract: `audit.csv` columns = `command,subcommand,category,mutates_state,phase_gate,unique_exit_contract,user_facing,agent_facing,overlap_with,disposition,rationale`.

## Data models
`audit.csv` row schema (CSV header above). Disposition enum: `keep | merge | deprecate | meta-hidden`. Category enum mirrors `command-reference.md` sections: `lifecycle | execution | inspection | record | program | orchestration | meta`.

`audit-summary.md` schema: counts table (total / keep / merge / deprecate / meta-hidden / surviving) + overflow list.

## Error handling
- Registry parse failure → abort with non-zero; emit no partial CSV (atomic write to temp then rename).
- A command present in docs but absent from registry → record as `doc-orphan` row, do not crash.
- Surviving-count > 20 → emit summary anyway with explicit `OVERFLOW` marker so the gate fails loudly rather than silently passing.

## Verification strategy
Verified with the specd harness itself plus shell assertions:
- `specd check cmd-audit` — gate the spec artifacts.
- `test -f .specd/specs/cmd-audit/audit.csv` — ledger exists.
- `awk -F, 'NR>1 && $10=="keep"' audit.csv | wc -l` plus meta-hidden count ≤ 20 — survivor assertion.
- `grep -c OVERFLOW audit-summary.md` returns 0.

## Risks and open questions
- **Risk:** `fusion` and `migrate` are absent from the analysis document; mis-categorizing them skews the count. Mitigation: classify from source (`fusion`=host bootstrap/policy oracle → `meta-hidden`; `migrate`=one-time config migration → `deprecate`).
- **Open question:** Should `program` subcommands count individually toward the 20? Resolved: the binary `program` collapses into `status --program`, so it scores as a single `deprecate` row.
- **Risk:** Subcommand explosion in `brain`/`pinky` inflates rows. Mitigation: top-level `brain`/`pinky` each count once toward survivors; subcommands are merge-target rows, not survivor rows.
