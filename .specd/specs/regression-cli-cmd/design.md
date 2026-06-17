# Design — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

## Overview
Freeze the CLI contract with golden tests over help text, exit codes, and `--json` schemas,
plus an end-to-end lifecycle test. Existing tests (args_test.go, json_contract_test.go,
lifecycle_test.go, registry_test.go, commands_test.go) are the base; gaps are closed so that
every subcommand has a parse test and every `--json` command has a schema assertion.

## Architecture
```
internal/cli/args.go  --> flag parse / usage
internal/cmd/registry.go --> subcommand dispatch (~25 cmds)
  each cmd ─ human output ─┐
            └ --json output ┴─ json_contract_test.go (schema-stable)
lifecycle: new -> check -> approve -> task -> report (lifecycle_test.go)
exit codes: exit.go taxonomy (golden)
```

## Components and interfaces
- **args.go** — parse + usage. Contract: unknown flag => non-zero usage error.
- **registry.go** — dispatch table. Contract: every registered cmd is reachable + tested.
- **each cmd `--json`** — stable top-level shape, no ANSI. Contract: parseable by agents.
- **exit codes** — distinct, documented, golden-locked.

## Data models
Per-command `--json` payloads (status, program, next, dispatch, etc.). Top-level keys are
the contract; additive change OK, rename/removal is a breaking regression.

## Error handling
Bad flag -> usage + non-zero. Out-of-order lifecycle -> gate block with reason. Failures
under `--json` -> machine-readable error object, still non-zero exit.

## Verification strategy
- Parse: table per subcommand of {args -> ok|usage error} (R1).
- JSON: assert each `--json` command emits valid JSON with expected top-level keys, no ANSI (R2).
- Lifecycle: full new->report walk in a temp repo (R3).
- Exit codes: golden table {scenario -> code} (R4).

## Risks and open questions
- `--json` shapes are large; over-strict golden tests are brittle. Mitigation: assert
  presence/type of top-level keys, not full byte-equality. Open: is additive JSON change
  allowed without a version bump? Default yes (additive = non-breaking); confirm in decisions.
