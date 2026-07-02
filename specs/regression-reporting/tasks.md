# S7 Tasks — Reporting Regression

Requirement coverage: R7. Dependencies: S3.

## Wave 1 — Baseline (after S3 green)

- [ ] Capture golden outputs for a fixed `ReportData`: Markdown, HTML, PR
  summary. Files: `internal/core/report.go`, `internal/core/testdata/` (new
  golden dir if absent).
- [ ] Inventory existing report tests: `report_test.go`, `render_test.go`,
  `prsummary_*`.
- **Validation:** `go test ./internal/core/... -run 'Report|Render' -race -count=1`

## Wave 2 — Golden-file regression (depends on Wave 1)

- [ ] Markdown golden diff via `RenderMarkdown`. File:
  `internal/core/report_test.go` (extend).
- [ ] HTML golden diff via `RenderHTML`; assert `<!doctype html>` + escaping.
  File: `internal/core/report_test.go`.
- [ ] PR summary golden diff via `prsummary.go`. File:
  `internal/core/prsummary_test.go` (extend or add).
- [ ] Metrics section matches telemetry input. File:
  `internal/cmd/report_metrics_test.go` (extend).
- **Validation:** `go test ./internal/core/... -run 'Report|Render|PRSummary|Metrics' -race -count=1`

## Wave 3 — Determinism (depends on Wave 2)

- [ ] Run `-count=2` to catch map-key ordering drift in rendered output.
- **Validation:** `go test ./internal/core/... -count=2`

## Rollout & cleanup

- [ ] Confirm `internal/core` ≥80% (`make cover-check`).
- **Rollback:** revert goldens + test extensions.
- **Completion evidence:** green golden diffs at count=2.
