# S14 — Documentation Accuracy Regression

## 1. Purpose and requirement coverage

Guarantee docs match actual command behavior, flags, and config keys. Covers
**R12**.

## 2. Verified current state

- Doc-lint gate: `make docs-lint` → `scripts/docs-lint.sh`, which cross-checks
  the cheat-sheet in `docs/command-reference.md` against its canonical spec copy
  so the two never drift (Makefile:60). Runs in CI `lint:` job.
- Config-doc test: `internal/core/config_docs_test.go` asserts documented config
  keys match code.
- Key docs: `docs/command-reference.md`, `docs/user-guide.md`,
  `docs/mcp-guide.md`, `docs/custom-gates.md`, `docs/validation-gates.md`,
  `SECURITY.md`, `README.md`, `TESTING.md`, `AGENTS.md`.
- Command source of truth: `internal/cmd/registry.go` (17 commands) + `main.go`
  (`mcp`/`help`/`version`).

## 3. Proposed design and end-to-end flow

Regression = `make docs-lint` green + `config_docs_test.go` green + a spot-check
that every `registry.go` command appears in `docs/command-reference.md` with
correct flags. Add a test asserting the documented command set equals the actual
registry set (catches added/removed commands like `migrate`/`doctor`).

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** cheat-sheet ↔ canonical copy parity; config-key documentation.
- **Dependencies:** S1 (command set is the doc's source of truth).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Invariant:** docs never describe a removed command (`doctor`, `migrate`) or
  omit a present one (`handshake`, `brain`, `pinky`, `context`, `waves`,
  `decision`, `memory`).
- **Rollback:** doc edits are git-revertible.

## 6. Acceptance criteria and validation commands

- `make docs-lint` passes.
- `go test ./internal/core/... -run 'ConfigDocs' -race -count=1` passes.
- Every `registry.go` command is documented in `docs/command-reference.md`;
  no doc references a non-existent command.

## 7. Open decisions and deviations

- Deviation D1: docs must be checked for stale `doctor`/`migrate` references and
  for coverage of the newer commands. This spec adds a registry↔docs parity
  assertion the analysis plan did not specify.
