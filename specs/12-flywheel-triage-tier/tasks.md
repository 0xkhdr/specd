# Tasks 12 — Extended Loop / Flywheel (triage tier)

> **Build waves:** H (T12.1–T12.4). See `specs/progress.md`.
> **Depends on domains:** 03 (gate interface), 02 (`state.records`). **Unblocks:** none (leaf).

## Wave 1 — security gate only (the sole v1 deliverable of this domain)

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T12.1 | craftsman | `internal/core/gates/security/scanners.go` | T3.1 (Spec 03) | `go test ./internal/core/gates/security -run TestScanners` | secrets/injection/slopsquat deterministic |
| T12.2 | craftsman | `internal/core/gates/security/allow.go` | T12.1 | `go test ./... -run TestAllowlistReasonRequired` | reasonless allowlist entry is a hard error |
| T12.3 | craftsman | `docs/deferred-flywheel.md` | — | `grep -q DeployApproval docs/deferred-flywheel.md` | evidence shapes documented for v2 |
| T12.4 | validator | `internal/core/gates/security/off_test.go` | T12.1 | `go test -run TestSecurityOffByDefault` | check output unaffected when security off |

## Traceability (task → requirement)
- T12.1 → R12.2 · T12.2 → R12.4 · T12.3 → R12.3, R12.5 · T12.4 → R12.1 (byte-identical when off), R12.2
