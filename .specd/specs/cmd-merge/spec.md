# Requirements — cmd-merge

> Collapse `merge`-disposition commands into broader survivors by absorbing their behavior behind flags/subcommands on the parent. Consumes `cmd-audit/audit.csv`.

## Context

Per the audit ledger, these commands merge into survivors (no new top-level commands; behavior preserved as flags):

| Merged command | Absorbing survivor | New surface |
|----------------|--------------------|-------------|
| `doctor`       | `init`             | `init --repair` |
| `mode`         | `new` + `status`   | `new --orchestrated`; `status` shows mode |
| `dispatch`     | `next`             | `next --dispatch` |
| `validate`     | `check`            | `check --schema-only` |
| `schema`       | `check`            | `check --schema` (emit) |
| `serve`        | `report`           | `report --serve` |
| `watch`        | `report`           | `report --watch` |
| `replay`       | `report`           | `report --history` |
| `diff`         | `report`           | `report --diff` |
| `program`      | `status`           | `status --program` |
| `brain run`    | `brain start`      | `brain start --auto-step` |
| `brain why`    | `brain status`     | `brain status --verbose` |
| `brain ledger` | `brain status`     | `brain status --ledger` |
| `brain compact/clear` | `brain checkpoint` | `brain checkpoint --compact` |
| `brain directive` | `brain step`    | `brain step --directive` |
| `pinky brief/heartbeat/progress/query/inbox/checkpoint` | `pinky status` + `pinky update` | merged sub-actions |

## Requirements

### REQ-001 — Behavior preservation under merge
**User story:** As a user relying on a merged command, I want its behavior reachable via the survivor's flag so that no capability is lost in consolidation.

- WHEN a command listed `merge` in `audit.csv` is invoked at its old path THE SYSTEM SHALL still perform the same effect via the absorbing survivor flag.
- THE SYSTEM SHALL NOT change exit-code semantics (0/1/2/3) for any merged behavior.
- THE SYSTEM SHALL route every merged flag through the existing handler of the absorbing command, not a duplicate.
- IF a merged behavior had a unique exit-code contract THEN THE SYSTEM SHALL preserve that contract on the new flag.

**Rationale:** Merging is consolidation, not removal; capability loss would force users back to reference lookups, defeating the memorizable-palette goal.

### REQ-002 — No new top-level commands
**User story:** As a maintainer, I want the merge to add zero top-level commands so that the §7 constraint holds and the surface only shrinks.

- THE SYSTEM SHALL add no new entry to the top-level command registry.
- THE SYSTEM SHALL express every merge as a flag or subcommand on an already-surviving command.
- WHERE a merge would require a new command THE SYSTEM SHALL instead deprecate the source (hand off to `cmd-deprecate`).

**Rationale:** A net-zero top-level count guarantees the optimization is strictly subtractive at the palette level.

### REQ-003 — Flag consolidation
**User story:** As an agent, I want overlapping flags to live on exactly one command so that I never guess which command owns a flag.

- THE SYSTEM SHALL place `--sandbox` and `--revert-on-fail` only on `verify`.
- THE SYSTEM SHALL place `--evidence` only on `task` and the record commands.
- THE SYSTEM SHALL place `--all` only on `next` and `status`, and `--format` only on `report`.
- THE SYSTEM SHALL support `--json` universally.

**Rationale:** One-home-per-flag removes the cross-command ambiguity that bloats agent working memory.

# Design — cmd-merge

## Overview
cmd-merge rewrites the command registry so that 16 merge-disposition commands/subcommands resolve through flags on surviving parents. It is the largest mutation spec. It changes registry wiring and flag ownership but preserves every behavior and exit code. Input contract: `audit.csv` rows where `disposition==merge`.

## Architecture
For each merge row: (a) add the target flag/subcommand to the absorbing command's `CommandMeta`, (b) point it at the existing handler, (c) remove the source command's top-level registry entry, (d) keep a thin alias only where `cmd-deprecate` schedules a grace period. The absorbing command's handler gains a switch on the new flag; no behavior is reimplemented.

## Components and interfaces
- **Registry editor** — mutates `internal/core/commands.go` `CommandMeta` entries.
- **Handler router** — in `internal/cmd/<survivor>.go`, dispatches on new flags to the pre-existing merged handler functions (moved, not rewritten).
- **Flag-ownership table** — single map asserting each consolidated flag has exactly one owning command; enforced by a Go test.
- Interface contract: merged handler signatures are unchanged; only their call site moves.

## Data models
No on-disk schema change. `state.json`, `config.yml`, `program.json` formats are untouched. The only model change is the in-memory `CommandMeta` registry shape (fewer top-level entries, more flags on survivors).

## Error handling
- Usage of a removed top-level command name → exit 2 (usage) with a one-line "moved to `<survivor> --<flag>`" hint, not a silent failure.
- Conflicting flag ownership at init → fail fast in a registry self-check test (`TestFlagSingleOwner`).
- Merged behavior that errors internally → propagate the original exit code unchanged.

## Verification strategy
- `specd check cmd-merge` — gate spec artifacts.
- `go test ./internal/core/ -run TestNoDuplicateCommands` — registry uniqueness.
- `go test ./internal/core/ -run TestFlagSingleOwner` — flag consolidation.
- `specd init --repair` / `specd next <slug> --dispatch` / `specd report <slug> --diff` — smoke each absorbed behavior.
- `specd verify cmd-merge <task>` — record evidence per task.

## Risks and open questions
- **Risk:** A merged behavior shares a flag name with the absorbing command's existing flag (e.g. `--reason`). Mitigation: namespace via subcommand where collision exists; `TestFlagSingleOwner` catches duplicates.
- **Risk:** `pinky` six-subcommand merge into `status`+`update` may lose telemetry granularity. Mitigation: preserve all fields as `--key=value` pairs on `pinky update`; verify field-completeness test.
- **Open question:** Does `brain run`'s driver loop have side effects beyond `start --auto-step`? Resolved during audit: `--auto-step` reuses the same poll-dispatch loop; confirm via behavior-parity smoke test.
