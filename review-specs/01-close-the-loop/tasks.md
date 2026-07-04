# Tasks W1 — Close the Loop

> Dogfood from here on: each task lives in a real spec under this repo's `.specd/specs/`,
> is verified via `specd verify`, and (once P1.1 lands) completed via `specd task complete`.
> P1.1 itself is the bootstrap: it is the first task ever closed by the verb it adds.

## Wave 1 — completion path

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P1.1 | craftsman | `internal/core/task_complete.go`, `internal/cmd/lifecycle.go`, `internal/cmd/registry.go` | — | `go test ./internal/core -run TestCompleteRequiresEvidence && go test ./internal/cmd -run TestTaskComplete` | complete-without-evidence exits non-zero; with passing record at HEAD, `state.json` updated under lock+CAS referencing record hash; `taskStatus` reads state not markers |
| P1.2 | craftsman | `internal/cmd/registry.go`, `internal/cmd/lifecycle.go` | — | `go test ./internal/cmd -run TestNextGatedOnApproval` | `next` on fresh spec: empty + reason naming missing approval; after requirements+design approved: frontier returned; `verify` refuses pre-approval |

## Wave 2 — mode & escape hatch

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P1.3 | craftsman | `internal/core/state.go`, `internal/cmd/lifecycle.go`, `internal/cmd/report.go` | P1.2 | `go test ./internal/core -run TestModeEnum && go test ./internal/cmd -run TestApproveModeTransition` | `new --mode` works, default `simple`; unknown mode = loud load error; `status --json` shows mode/phase/status (original T2.6 verify passes); `brain start` on simple spec refuses |
| P1.4 | craftsman | `internal/core/task_complete.go`, `internal/cmd/lifecycle.go` | P1.1 | `go test ./internal/core -run TestUnverifiedEscapeHatch` | craftsman task refuses `--unverified`; scout/auditor/validator accepted; ledger shows `unverified-attestation` kind; `--unverified` sans `--evidence` = usage error |

## Wave 3 — regression

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P1.5 | validator | `internal/cmd/e2e_test.go` | P1.1, P1.2, P1.3, P1.4 | `go test ./internal/cmd -run TestE2E` | full conductor lifecycle e2e: init → new → approve×2 → next → verify → task complete → approve; pre-approval dispatch impossible |

## Traceability (task → requirement → finding)
- P1.1 → R1.1 → F2 · P1.2 → R1.2 → F3 · P1.3 → R1.3 → F5 · P1.4 → R1.4 → F14 · P1.5 → R1.1–R1.4
