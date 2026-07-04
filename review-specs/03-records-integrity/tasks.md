# Tasks W3 — Make Records Mean Something

> Dogfooded. Runs parallel with W4 (disjoint files except `lifecycle.go` — coordinate).

## Wave 1 — record enrichment

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P3.1 | craftsman | `internal/core/state.go`, `internal/cmd/lifecycle.go` | — | `go test ./internal/core -run TestStampRecord && go test ./internal/cmd -run TestDecisionRequiresText` | `decision <slug>` without `--text` = usage error; `decision demo --text "…"` round-trips through `status --json`; all record kinds stamped timestamp/git_head/actor via one helper + injectable Clock |
| P3.2 | craftsman | `internal/core/task_complete.go`, `internal/cmd/registry.go` | — | `go test ./internal/core -run TestRejectUnknownHead` | verify in commitless repo warns; `task complete` there fails with clear message; pre-W3 unstamped records refused as completion evidence, remedy named |

## Wave 2 — regression

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P3.3 | validator | `internal/cmd/e2e_test.go` | P3.1, P3.2 | `go test ./internal/cmd -run TestE2E` | e2e: approval records name gate + artifact revision; ledger append-only (old records loadable, not completion-valid); high/critical midreq never auto-cleared |

## Traceability (task → requirement → finding)
- P3.1 → R3.1, R3.4 → F6 · P3.2 → R3.2, R3.3 → F14 · P3.3 → R3.1–R3.4
