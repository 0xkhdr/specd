# Tasks W4 — Finish the Gate Engine, Wake the Constitution

> Dogfooded. Parallel with W3; shares no gate/manifest files with it.

## Wave 1 — content gates

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P4.1 | craftsman | `internal/core/gates/ears.go`, `internal/core/gates/core.go` | — | `go test ./internal/core/gates -run TestEARSGate` | fresh scaffold: `check` errors on placeholder; edited EARS file passes; non-EARS lines warn; registered via one registry call |
| P4.2 | craftsman | `internal/core/gates/{approval,sync}.go`, `internal/core/gates/core.go`, `internal/cmd/lifecycle.go` | — | `go test ./internal/core/gates -run 'TestApprovalGate|TestSyncGate' && go test ./internal/cmd -run TestApproveDesignStub` | task progress with unapproved reqs/design → error; `approve demo design` with empty design refused; checkbox↔state disagreement → error; severity floors pinned `error` |

## Wave 2 — constitution into context

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P4.3 | craftsman | `internal/context/manifest.go` | — | `go test ./internal/context -run TestSteeringInManifest` | `context demo T1 --json` lists steering (static-instructions mode) + memory (`reference-if-needed`) items within budget; over budget → memory drops before steering, deterministic; references only, no inlined content; all three surfaces get them (one engine) |

## Wave 3 — parity

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P4.4 | validator | `internal/core/gates/parity_test.go` | P4.1, P4.2, P4.3 | `go test ./internal/core/gates -run TestParity` | `check` output byte-identical when new gates off/green |

## Traceability (task → requirement → finding)
- P4.1 → R4.1 → F8 · P4.2 → R4.2 → F8 · P4.3 → R4.3 → F9 · P4.4 → R4.4
