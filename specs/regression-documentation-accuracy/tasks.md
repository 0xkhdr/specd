# S14 Tasks — Documentation Accuracy Regression

Requirement coverage: R12. Dependencies: S1.

## Wave 1 — Baseline (after S1 green)

- [x] Run `make docs-lint` and `config_docs_test.go`; record baseline. Files:
  `scripts/docs-lint.sh`, `internal/core/config_docs_test.go`.
- [x] Extract the documented command list from `docs/command-reference.md`.
- **Validation:** `make docs-lint`

## Wave 2 — Registry↔docs parity (depends on Wave 1)

- [x] Add a test asserting documented commands == `registry.go` set (+ `mcp`,
  `help`, `version`). File: `internal/cmd/agents_cli_drift_test.go` (extend) or
  a new `docs_parity_test.go`.
- [x] Grep docs for stale `doctor`/`migrate` references and remove them.
- [x] Confirm newer commands (`handshake`, `brain`, `pinky`, `context`, `waves`,
  `decision`, `memory`) are documented with flags.
- **Validation:** `make docs-lint && go test ./internal/cmd/... -run Drift -race -count=1`

## Wave 3 — Config keys (depends on Wave 2)

- [x] Ensure `config_docs_test.go` covers all documented config keys.
- **Validation:** `go test ./internal/core/... -run ConfigDocs -race -count=1`

## Rollout & cleanup

- [x] Ensure cheat-sheet ↔ canonical copy stay in sync (`docs-lint`).
- **Rollback:** revert doc edits.
- **Completion evidence:** green `docs-lint` + parity test; no stale commands.
