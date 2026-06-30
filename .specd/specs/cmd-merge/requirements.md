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

## Requirement 1 — Behavior preservation under merge
**User story:** As a user relying on a merged command, I want its behavior reachable via the survivor's flag so that no capability is lost in consolidation.

**Acceptance criteria:**

1. WHEN a command listed `merge` in `audit.csv` is invoked at its old path THE SYSTEM SHALL still perform the same effect via the absorbing survivor flag.
2. THE SYSTEM SHALL NOT change exit-code semantics (0/1/2/3) for any merged behavior.
3. THE SYSTEM SHALL route every merged flag through the existing handler of the absorbing command, not a duplicate.
4. IF a merged behavior had a unique exit-code contract THEN THE SYSTEM SHALL preserve that contract on the new flag.

**Rationale:** Merging is consolidation, not removal; capability loss would force users back to reference lookups, defeating the memorizable-palette goal.

## Requirement 2 — No new top-level commands
**User story:** As a maintainer, I want the merge to add zero top-level commands so that the §7 constraint holds and the surface only shrinks.

**Acceptance criteria:**

1. THE SYSTEM SHALL add no new entry to the top-level command registry.
2. THE SYSTEM SHALL express every merge as a flag or subcommand on an already-surviving command.
3. WHERE a merge would require a new command THE SYSTEM SHALL instead deprecate the source (hand off to `cmd-deprecate`).

**Rationale:** A net-zero top-level count guarantees the optimization is strictly subtractive at the palette level.

## Requirement 3 — Flag consolidation
**User story:** As an agent, I want overlapping flags to live on exactly one command so that I never guess which command owns a flag.

**Acceptance criteria:**

1. THE SYSTEM SHALL place `--sandbox` and `--revert-on-fail` only on `verify`.
2. THE SYSTEM SHALL place `--evidence` only on `task` and the record commands.
3. THE SYSTEM SHALL place `--all` only on `next` and `status`, and `--format` only on `report`.
4. THE SYSTEM SHALL support `--json` universally.

**Rationale:** One-home-per-flag removes the cross-command ambiguity that bloats agent working memory.
