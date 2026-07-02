# S1 Tasks — CLI Command Regression

Requirement coverage: R1, R12. Dependencies: none (Wave-1 root).

## Wave 1 — Baseline

- [ ] Snapshot current exit codes: for each `Registry` command, record the exit
  code on valid input, bad-flag input, and missing-root input. Likely file:
  `internal/cmd/exit_matrix_test.go` (new).
- [ ] Enumerate `--json` top-level keys per command from existing
  `json_contract_test.go`; note any command lacking a JSON contract test.
- **Validation:** `go test ./internal/cmd/... -race -count=1`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] Add table-driven exit-code test covering all 17 registry commands
  (valid → documented code; bad flag → `ExitUsage`; missing root/slug →
  `ExitNotFound`). File: `internal/cmd/exit_matrix_test.go`.
- [ ] Add `main.go` routing tests for `mcp`, `help`, `version` (incl.
  `version --json`) and unknown-command → help+error. File: `main_test.go`
  (extend existing).
- [ ] Assert `--json` outputs parse and carry documented keys for every command
  that supports `--json`. File: `internal/cmd/json_contract_test.go` (extend).
- **Validation:** `go test ./internal/cmd/... -race -count=1 && go test . -run Version`

## Wave 3 — Determinism & integration (depends on Wave 2)

- [ ] Run full command suite under `-count=2` and fix any order dependence.
- [ ] Add lifecycle assertion `new → check → approve → verify → task complete`
  exit codes (extend `internal/cmd/lifecycle_test.go`).
- **Validation:** `go test ./internal/cmd/... -count=2`

## Rollout & cleanup

- [ ] Confirm `internal/cmd` coverage still ≥71% (`make cover-check`).
- [ ] Remove any temporary fixtures under `internal/cmd/testdata/`.
- **Rollback:** delete `internal/cmd/exit_matrix_test.go`; revert extensions.
- **Completion evidence:** green `go test ./internal/cmd/... -count=2` +
  passing `make cover-check`.
