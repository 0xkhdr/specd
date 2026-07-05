# Tasks — 10-telemetry

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/core/task_complete.go, internal/core/evidence.go, internal/orchestration/acp.go, internal/cmd/report.go | | `printf ok` | Confirms record shapes to extend, whether state.json schema bump needed (spec 02 discipline), and current ACP claim/report field set |
| T2 | craftsman | internal/core/telemetry.go, telemetry_test.go | T1 | `go test ./internal/core -run TestTelemetryFields -race -count=1` | Annotation type {tokens int, cost decimal-string, durationMs int}; validation rejects non-integer/negative/non-decimal exit-2 semantics; stored verbatim, zero computation paths (R1, R2) |
| T3 | craftsman | internal/core/task_complete.go, internal/cmd/task.go, internal/cmd/verify.go + tests | T2 | `go test ./internal/cmd -run TestTaskTelemetry -race -count=1` | `--tokens/--cost/--duration-ms` optional on task completion + verify record; records without annotations fully valid (R1, R5) |
| T4 | craftsman | internal/orchestration/acp.go + tests | T2 | `go test ./internal/orchestration -run TestACPRigor -race -count=1` | Claim/report records carry attempt number (derived: count of prior claims + 1 under spec lock), git HEAD, changed-files, verification ref, optional annotations (R3) |
| T5 | craftsman | internal/cmd/report.go + tests | T3,T4 | `go test ./internal/cmd -run TestReportMetrics -race -count=1` | Per-spec + per-task aggregation with per-attempt breakdown; decimal aggregation via big.Rat/scaled ints, float-poison test (0.1+0.2 accumulation); missing telemetry shown as absent, never imputed (R4, R6) |
| T6 | craftsman | .claude/agents/pinky-craftsman.md, pinky-validator.md, docs/agent-integration.md | T3 | `grep -l "duration-ms" .claude/agents/pinky-craftsman.md` | Pinky mission templates instruct workers to report tokens/cost/duration; agent-integration docs updated |
| T7 | craftsman | docs/command-reference.md, docs/CHEATSHEET.md | T3,T5 | `./scripts/docs-lint.sh` | New flags + stored-never-computed doctrine documented |
| T8 | validator | (read-only) | T5 | `go test ./... -race -count=1` | Full suite green including migrated fixtures if schema bumped |
