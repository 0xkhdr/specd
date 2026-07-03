# Tasks 09 — Orchestration (Brain/Pinky)

> **Build waves:** G (T9.1–T9.10). See `specs/progress.md`.
> **Depends on domains:** 02, 04, 05, 07, 08, 10. **Unblocks:** 11/12 observation (largely deferred).

## Wave 1 — pure core (no IO)

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.1 | craftsman | `internal/orchestration/decide.go` | — | `go test ./internal/orchestration -run TestDecidePure` | Decide is pure, deterministic, no IO |
| T9.2 | craftsman | `internal/orchestration/sense.go` | — | `go test ./internal/orchestration -run TestSense` | snapshot built from state+frontier+leases |
| T9.3 | craftsman | `internal/orchestration/brakes.go` | — | `go test ./internal/orchestration -run TestBrakes` | cost>limit and deadline→halt/timeout |

## Wave 2 — transport & leases

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.4 | craftsman | `internal/orchestration/acp.go` | T9.2 | `go test ./internal/orchestration -run TestACPRoundtrip` | append-only; restart-recoverable |
| T9.5 | craftsman | `internal/orchestration/lease.go` | T9.2 | `go test ./internal/orchestration -run TestLeaseReclaim` | expiry reclaim + retries + escalate |
| T9.6 | craftsman | `internal/orchestration/session.go` | T9.4 | `go test ./internal/orchestration -run TestSessionCAS` | session.json CAS under lock |

## Wave 3 — commands & integrity

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.7 | craftsman | `internal/cmd/brain.go`, `internal/orchestration/driver.go` | T9.1, T9.5, T9.6 | `go test ./internal/orchestration -run TestBrainDriverDispatchesFrontier` | driver loop dispatches frontier |
| T9.8 | craftsman | `internal/cmd/pinky.go`, `internal/cmd/brain_worker.go` | T9.4 | `go test ./internal/cmd -run TestReportRequiresVerify` | report rejected without passing record |
| T9.9 | craftsman | `internal/core/config_validate.go`, `internal/orchestration/authority.go` | T9.6 | `go test ./internal/orchestration -run TestFailClosedAuthority` | disabled by default; can't clear high/critical gates |
| T9.10 | validator | `internal/orchestration/decide_test.go` | T9.1 | `go test ./internal/orchestration -run TestNoLLM` | grep proves no model/network import in decision path |

## Traceability (task → requirement)
- T9.1 → R9.1 · T9.2 → R9.1 (snapshot) · T9.3 → R9.4, R9.5 · T9.4 → R9.8 · T9.5 → R9.3 · T9.6 → R9.8 · T9.7 → R9.6 · T9.8 → R9.2 · T9.9 → R9.7 · T9.10 → R9.1
