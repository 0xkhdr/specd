# Tasks 02 — Spec Lifecycle & State Model

> **Build waves:** B (T2.1–T2.3), C (T2.4–T2.7). See `specs/progress.md`.
> **Depends on domains:** 10, 01. **Unblocks:** 03, 04, 05, 09, 11.

## Wave 1 — state core

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T2.1 | craftsman | `internal/core/state.go` | — | `go test ./internal/core -run TestStateCAS` | CAS refuses on stale revision; monotonic bump |
| T2.2 | craftsman | `internal/core/io.go` | — | `go test ./internal/core -run TestAtomicWrite` | temp→fsync→rename; partial write never replaces |
| T2.3 | craftsman | `internal/core/phases.go` | T2.1 | `go test ./internal/core -run TestPhaseRatchet` | forward-only; backward rejected |

## Wave 2 — lifecycle commands

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T2.4 | craftsman | `internal/cmd/new.go` | T2.1 | `go run . new demo && test -f .specd/specs/demo/state.json` | new spec at rev 0, mode simple, status requirements |
| T2.5 | craftsman | `internal/cmd/approve.go`, `internal/core/task_complete.go` | T2.3 | `go test ./internal/cmd -run TestApproveGates` | approve blocked when readiness fails; `--mode` auditable |
| T2.6 | craftsman | `internal/cmd/status.go` | T2.1 | `go run . status demo --json \| grep -q '"mode":"simple"'` | status is a pure state projection |
| T2.7 | validator | `internal/core/state_lock_test.go` | T2.1 | `go test ./internal/core -run TestSaveStateRequiresLock` | unlocked SaveState panics in test build |

## Traceability (task → requirement)
- T2.1 → R2.1, R2.2 · T2.2 → R2.3 · T2.3 → R2.5 · T2.4 → R2.1 · T2.5 → R2.4, R2.8 · T2.6 → R2.6 · T2.7 → R2.2
- R2.7 (records map) exercised by Spec 12 plugins.
