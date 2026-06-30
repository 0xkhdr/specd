# Requirements — cmd-audit

## Requirement 1 — Complete command inventory

**User story:** As a maintainer, I want a machine-readable inventory of every command and its attributes so that no command is silently dropped or kept without justification.

**Acceptance criteria:**
1. THE SYSTEM SHALL enumerate every top-level command present in `internal/core/commands.go`.
2. WHEN a command exposes subcommands THE SYSTEM SHALL record each subcommand in the audit metadata.
3. THE SYSTEM SHALL record per row command category mutability phase-gate exit-contract and audience flags.
4. IF a command in the registry is absent from `docs/command-reference.md` THEN THE SYSTEM SHALL flag it as `undocumented`.

## Requirement 2 — Overlap scoring

**User story:** As a maintainer, I want each command scored for functional overlap so that merge candidates are identified by evidence rather than intuition.

**Acceptance criteria:**
1. THE SYSTEM SHALL assign each command an `overlap_with` field naming any broader command that could subsume it or `none`.
2. THE SYSTEM SHALL assign a disposition of `keep`, `merge`, `deprecate`, or `meta-hidden`.
3. WHERE two commands share a non-empty overlap target THE SYSTEM SHALL mark the narrower command `merge` unless the survivor ledger keeps it.
4. THE SYSTEM SHALL classify the non-negotiable commands from `PROMPT.md` section 5.1 as `keep`.

## Requirement 3 — Disposition ledger drives downstream specs

**User story:** As the author of downstream command-palette specs, I want a single source-of-truth ledger so that merge and deprecation work does not diverge.

**Acceptance criteria:**
1. THE SYSTEM SHALL emit the audit as `.specd/specs/cmd-audit/audit.csv`.
2. THE SYSTEM SHALL produce a summary asserting the surviving command count is less than or equal to 20.
3. IF the surviving command count exceeds 20 THEN THE SYSTEM SHALL list the overflow commands requiring further merge.
