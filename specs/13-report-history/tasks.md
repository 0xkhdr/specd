# Tasks — 13-report-history

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/core/ (evidence, state, ledgers), internal/orchestration/acp.go, internal/cmd/report.go | | `printf ok` | Inventory of every record source with its timestamp field + actor field availability; confirms all history inputs already exist on disk |
| T2 | craftsman | internal/core/history.go, history_test.go | T1 | `go test ./internal/core -run TestHistoryMerge -race -count=1` | Pure merge-sort over record sources into event stream; deterministic tie-break (documented: timestamp, then record-type, then id); zero writes during read (fs snapshot test); repeated runs byte-identical (R1, R2, R3) |
| T3 | craftsman | internal/cmd/report.go + tests | T2 | `go test ./internal/cmd -run TestReportHistory -race -count=1` | `report --history` line format (timestamp, actor, event, reference); `--history --json` emits JSON lines of same events (R1, R6) |
| T4 | craftsman | internal/core/promtext.go, promtext_test.go | T1 | `go test ./internal/core -run TestPromText -race -count=1` | Exposition-format writer: HELP/TYPE lines, `specd_` prefix, snake_case + unit suffixes, label escaping, duplicate-series detection; internal lint tests (R4, R5) |
| T5 | craftsman | internal/cmd/report.go + tests | T4 | `go test ./internal/cmd -run TestReportPrometheus -race -count=1` | `--format prometheus`: tasks by status, verify attempts/failures, escalated count, criteria coverage, telemetry totals where stored; absent features absent (no placeholder spam); `{spec="<slug>"}` labels (R4) |
| T6 | craftsman | docs/command-reference.md, docs/CHEATSHEET.md, docs/user-guide.md, docs/decisions/ (--diff deferral ADR) | T3,T5 | `./scripts/docs-lint.sh` | Metric-name contract table (names are API); textfile-collector usage note; --diff deferral ADR |
| T7 | validator | (read-only) | T3,T5 | `go test ./... -race -count=1` | Full suite green; e2e fixture replay matches golden history output |
