# Decisions — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-17
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — Parser is intentionally permissive: unknown flags are silently parsed into Args.Flags, NOT rejected. R1.2 'unknown flag -> non-zero' therefore holds only for unknown COMMANDS (ExitUsage) and missing REQUIRED ARGS (handler-level UsageError), not unknown flags. T2 tests freeze this real behavior: documented-flag parse table, documented-boolean-flags-registered guard, and an explicit test asserting unknown flags are tolerated. Changing the parser to reject unknown flags is out of scope (contract: do NOT change parsing behavior). · 2026-06-17
**Context:** TODO
**Decision:** Parser is intentionally permissive: unknown flags are silently parsed into Args.Flags, NOT rejected. R1.2 'unknown flag -> non-zero' therefore holds only for unknown COMMANDS (ExitUsage) and missing REQUIRED ARGS (handler-level UsageError), not unknown flags. T2 tests freeze this real behavior: documented-flag parse table, documented-boolean-flags-registered guard, and an explicit test asserting unknown flags are tolerated. Changing the parser to reject unknown flags is out of scope (contract: do NOT change parsing behavior).
**Consequences:** TODO
**Supersedes:** —
