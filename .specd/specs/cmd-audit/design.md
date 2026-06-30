# Design — cmd-audit

## Overview
cmd-audit is a read-only analysis spec. It inventories the live command registry, scores each command for overlap and disposition, and emits a CSV ledger plus a markdown summary under `.specd/specs/cmd-audit/`.

## Architecture
The audit uses a deterministic extract-classify-emit pipeline: parse the embedded command registry, cross-reference the command reference documentation, apply the command palette decision matrix, then write stable artifacts consumed by downstream specs.

## Components and interfaces
- Registry extractor: reads `internal/core/commands.go` and records command metadata.
- Documentation cross-referencer: reads `docs/command-reference.md` and records documented status.
- Disposition classifier: applies `PROMPT.md` section 5 and the progress survivor ledger.
- Ledger writer: emits `registry.txt`, `audit.csv`, and `audit-summary.md`.

## Data models
`registry.txt` is tab-separated and contains command, category, subcommands, flags, and documentation status. `audit.csv` uses the columns `command,subcommand,category,mutates_state,phase_gate,unique_exit_contract,user_facing,agent_facing,overlap_with,disposition,rationale`.

## Error handling
Registry parse failure aborts the audit and leaves no partial interpretation. Documentation gaps are recorded as `undocumented` instead of crashing. Survivor overflow is written explicitly in the summary so downstream gates fail visibly.

## Verification strategy
The audit is verified with shell assertions for artifact existence, disposition coverage, survivor count, and absence of overflow. `specd check cmd-audit` validates the harness-facing artifacts.

## Risks and open questions
Subcommand-rich orchestration commands could inflate row counts, so the audit counts top-level commands for the palette target while preserving subcommands in metadata. The progress survivor ledger resolves the record-command decision by retaining decision, midreq, and memory as daily workflow commands for this suite.
