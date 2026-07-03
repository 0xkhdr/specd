# Tasks 03 — Validation Gates Engine

> **Build waves:** D (T3.1–T3.3), E (T3.4–T3.6). See `specs/progress.md`.
> **Depends on domains:** 02, 04, 05 (+ 08 for context-budget gate). **Unblocks:** 12.

## Wave 1 — interface & core gates

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T3.1 | craftsman | `internal/core/gates/registry.go` | — | `go test ./internal/core/gates -run TestRegistryOrder` | deterministic ordered run |
| T3.2 | craftsman | `internal/core/gates/core.go`, `internal/core/ears.go`, `internal/core/dag.go` | T3.1 | `go test ./internal/core/gates -run TestCoreGates` | 7 core gates pass/fail correctly, no IO |
| T3.3 | craftsman | `internal/cmd/check.go` | T3.1, T3.2 | `go run . check demo` | check runs registry only; exit codes correct |

## Wave 2 — opt-in modules on the new interface

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T3.4 | craftsman | `internal/core/gates/security/*` | T3.1 | `go run . check demo --security` | scanners run as gates; allowlist reason mandatory |
| T3.5 | craftsman | `internal/core/gates/contextbudget.go` | T3.1 | `go test ./internal/core/gates -run TestContextBudgetGate` | fails when manifest over budget |
| T3.6 | validator | `internal/core/gates/parity_test.go` | T3.2 | `go test ./internal/core/gates -run TestByteIdenticalWhenOptInsOff` | output byte-identical with opt-ins off |

## Traceability (task → requirement)
- T3.1 → R3.1, R3.5 · T3.2 → R3.2, R3.4, R3.7 · T3.3 → R3.4 · T3.4 → R3.6 · T3.5 → R3.2 (budget gate, see Spec 08) · T3.6 → R3.3
