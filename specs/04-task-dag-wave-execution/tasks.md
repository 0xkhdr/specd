# Tasks 04 — Task DAG & Wave Execution

> **Build waves:** C (T4.1–T4.3), D (T4.4–T4.6). See `specs/progress.md`.
> **Depends on domains:** 02, 10. **Unblocks:** 03, 08, 09.

## Wave 1 — parser & round-trip

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T4.1 | craftsman | `internal/core/tasksparser.go`, `internal/core/md.go` | — | `go test ./internal/core -run TestTasksRoundTrip` | Serialize∘Parse is identity over fuzz corpus |
| T4.2 | craftsman | `internal/core/tasksparser.go` | T4.1 | `go test ./internal/core -run TestSingleLineRewrite` | status change rewrites one line only |

## Wave 2 — graph & scheduler

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T4.3 | craftsman | `internal/core/dag.go` | T4.1 | `go test ./internal/core -run TestDAG` | cycle/orphan/wave violations detected with id+line |
| T4.4 | craftsman | `internal/cmd/next.go`, `internal/core/frontier.go` | T4.3 | `go run . next demo --json` | frontier ordered by ordinal; correct terminal kinds |
| T4.5 | craftsman | `internal/cmd/next.go` | T4.4 | `go run . next demo --waves` | wave projection replaces `waves` command |
| T4.6 | validator | `internal/core/tasksparser_fuzz_test.go` | T4.1 | `go test ./internal/core -run FuzzTasks -fuzztime=30s` | no round-trip violation found |

## Traceability (task → requirement)
- T4.1 → R4.1 · T4.2 → R4.2 · T4.3 → R4.3 · T4.4 → R4.4, R4.5 · T4.5 → R4.6 · T4.6 → R4.1
