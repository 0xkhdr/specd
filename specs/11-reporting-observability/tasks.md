# Tasks 11 — Reporting & Observability

> **Build waves:** F (T11.1–T11.5). See `specs/progress.md`.
> **Depends on domains:** 02, 05. **Unblocks:** none (leaf).

## Wave 1 — projection & renderers

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T11.1 | craftsman | `internal/core/report.go` | — | `go test ./internal/core -run TestReportModel` | one model built from state |
| T11.2 | craftsman | `internal/cmd/status.go` | T11.1 | `go run . status demo --json` | pure state projection |
| T11.3 | craftsman | `internal/core/prsummary.go`, `internal/core/commitlink.go` | T11.1 | `go test ./internal/core -run TestPRSummaryGolden` | byte-stable PR summary with commit links |
| T11.4 | craftsman | `internal/core/report_metrics.go` | T11.1 | `go test ./internal/core -run TestMetricsGolden` | deterministic Prometheus textfile |
| T11.5 | validator | `internal/core/report_purity_test.go` | T11.1 | `go test ./internal/core -run TestNoLLMInRender` | no model/network import in render path |

## Traceability (task → requirement)
- T11.1 → R11.1 · T11.2 → R11.1, R11.6 · T11.3 → R11.2, R11.3 · T11.4 → R11.4 · T11.5 → R11.1, R11.5
