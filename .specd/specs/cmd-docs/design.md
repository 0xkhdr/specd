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
