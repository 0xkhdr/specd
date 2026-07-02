# S7 — Reporting Regression

## 1. Purpose and requirement coverage

Guarantee report generation produces valid, structurally-stable Markdown, HTML,
and PR summaries. Covers **R7**.

## 2. Verified current state

- Renderers: `internal/core/report.go` — `RenderMarkdown` (`report.go:254`),
  `RenderHTML(d, autoRefreshSeconds)` (`report.go:291`, emits `<!doctype html>`).
  `ReportData` bundles state + planning markdown (`report.go:36`).
- PR summary: `internal/core/prsummary.go`. Metrics section:
  `internal/core/report_metrics.go`.
- `report` command: `internal/cmd/report.go`, tested by `report_metrics_test.go`;
  core tests in `report_test.go`, `report_cov_test.go`, `render_test.go`,
  `render_cov_test.go`, `prsummary_*`.
- Dashboard docs: `docs/dashboard.md`; HTML auto-refresh fetch loop in
  `report.go:320`.

## 3. Proposed design and end-to-end flow

Golden-file tests render a fixed `ReportData` and diff against committed golden
Markdown/HTML/PR-summary outputs. Assert: valid HTML doctype + well-formed
structure; Markdown headings/sections present; PR summary carries pass/fail
counts; metrics section matches telemetry input. Run at `-count=2` for ordering
stability (map-key sort).

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** `ReportData` fields; output structure of all three formats; the
  `autoRefreshSeconds` HTML parameter.
- **Dependencies:** S3 (report consumes DAG/frontier state).

## 5. Invariants, security, errors, observability, compatibility, rollback

- HTML output must escape user content (XSS safety) — assert no raw injection.
- **Compatibility:** report formats stay structurally compatible for downstream
  parsers/dashboards.
- **Observability:** metrics section reflects `report_metrics.go` accurately.
- **Rollback:** golden files versioned; revert restores prior output.

## 6. Acceptance criteria and validation commands

- `go test ./internal/core/... -run 'Report|Render|PRSummary|Metrics' -race -count=1`
  passes.
- `go test ./internal/core/... -count=2` stable (golden order).
- HTML output parses as valid document; contains `<!doctype html>`.

## 7. Open decisions and deviations

- Plan cites `internal/core/report.go` + `internal/cmd/report.go` — verified.
  PR summary lives in `prsummary.go` (confirmed), not inlined in `report.go`.
