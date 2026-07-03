# Tasks 05 — Evidence & Verification

> **Build waves:** C (T5.1–T5.3), D (T5.4–T5.7). See `specs/progress.md`.
> **Depends on domains:** 02, 10. **Unblocks:** 03, 09.

## Wave 1 — runner & evidence

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T5.1 | craftsman | `internal/core/verify/exec.go`, `internal/core/customgate.go` | — | `go test ./internal/core/verify -run TestScrubbedEnv` | env allowlisted; NUL rejected; command printed |
| T5.2 | craftsman | `internal/core/evidence/ledger.go` | — | `go test ./internal/core/evidence -run TestAppendOnly` | records immutable; hash-referenced |
| T5.3 | craftsman | `internal/core/verify/capture.go` | T5.1 | `go test ./internal/core/verify -run TestChangedFiles` | changed files captured for scope gate |

## Wave 2 — completion integrity

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T5.4 | craftsman | `internal/core/task_complete.go` | T5.2 | `go test ./internal/core -run TestCompleteRequiresEvidence` | no complete without passing record |
| T5.5 | craftsman | `internal/cmd/verify.go` | T5.1, T5.3 | `go run . verify demo T1` | thin wiring; sandbox fail-closed |
| T5.6 | craftsman | `internal/cmd/verify.go` | T5.5 | `go test ./internal/cmd -run TestRevertOnFail` | working tree restored on failure |
| T5.7 | validator | `internal/core/verify/sandbox_test.go` | T5.5 | `go test ./internal/core/verify -run TestSandboxFailClosed` | missing sandbox binary → fail closed |

## Traceability (task → requirement)
- T5.1 → R5.2, R5.3 · T5.2 → R5.5 · T5.3 → R5.2 (capture) · T5.4 → R5.1, R5.6 · T5.5 → R5.4 · T5.6 → R5.7 · T5.7 → R5.4
