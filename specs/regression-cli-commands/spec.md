# S1 — CLI Command Regression

## 1. Purpose and requirement coverage

Guarantee every dispatchable CLI command produces a **deterministic exit code**
and stable stdout/`--json` output before v1.0.0. Covers **R1** (deterministic
exit codes) and contributes to **R12** (help/usage text matches docs).

## 2. Verified current state

- Exit codes are the four constants in `internal/core/exit.go`:
  `ExitOK=0`, `ExitGate=1`, `ExitUsage=2`, `ExitNotFound=3`. Errors carry a code
  via `core.SpecdError` (`GateError`/`UsageError`/`NotFoundError`).
- The command table lives in `internal/cmd/registry.go` (`Registry`). Verified
  dispatchable commands: `init`, `handshake`, `new`, `approve`, `decision`,
  `midreq`, `memory`, `brain`, `pinky`, `next`, `verify`, `task`, `status`,
  `context`, `check`, `report`, `waves`.
- `main.go` handles three commands *before* dispatch: `mcp` (long-lived stdio
  transport, `main.go:107`), `--help|-h|help` (`main.go:51`), and
  `--version|-v|version` (`main.go:91`, supports `--json`). Unknown commands
  print help and error out (`main.go:138`).
- Existing coverage: `internal/cmd/commands_test.go`, `json_contract_test.go`,
  `lifecycle_test.go`, plus per-command `*_test.go` files. `internal/cmd` has a
  coverage floor of **71%** (`scripts/coverage-check.sh`).

> Deviation D1: the analysis plan lists 13 commands including `migrate` and
> `doctor`. Neither exists in `registry.go` or `main.go`. New commands present
> that the plan omits: `handshake`, `decision`, `memory`, `brain`, `pinky`,
> `context`, `waves`. This spec uses the verified registry, not the plan list.

## 3. Proposed design and end-to-end flow

Table-driven regression tests that, for each command, assert: (a) exit code for
valid invocation, (b) exit code `2` for malformed flags/missing args, (c) exit
code `3` when the `.specd` root or slug is absent, (d) `--json` output parses
and carries the documented top-level keys, (e) `help`/`version` text is stable.
Drive commands through `cmd.Dispatch` and `main` entry so routing is exercised
end-to-end. Assert byte-stability by running the suite under `-count=2`.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** the `Registry` command names and their exit-code contract; the
  `--json` top-level shape per command; `mcp`/`help`/`version` pre-dispatch
  routing in `main.go`.
- **Config:** commands resolve `.specd` root and honor `SPECD_*` env overrides.
- **Dependencies:** none (Wave 1 root spec). stdlib-only (`go.mod`).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **INV3** (deterministic exit codes): same input → same code.
- **Errors:** every error path returns one of the four codes; no bare `os.Exit`
  with undocumented codes.
- **Observability:** command paths must not panic; `obs.Record*` calls no-op
  safely.
- **Compatibility:** flag surfaces are backward-compatible; no command renamed
  without a migration note.
- **Rollback:** tests are additive; revert by deleting the new `*_test.go`.

## 6. Acceptance criteria and validation commands

- `go test ./internal/cmd/... -race -count=1` passes.
- `go test ./internal/cmd/... -count=2` passes (order/iteration stability).
- Every registry command has at least one valid-path and one error-path
  exit-code assertion.
- `go build -o specd . && ./specd --version --json` emits `{"version": "..."}`.

## 7. Open decisions and deviations

- D1 (above): command list corrected to the verified registry.
- Open: whether `dispatch`/`serve`/`watch` (present as source files but not in
  `Registry`) are internal-only; treat as out of scope until confirmed.
