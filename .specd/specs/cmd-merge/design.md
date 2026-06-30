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
