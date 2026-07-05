# Tasks — 12-program-links

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/core/dag.go, internal/core/frontier.go, internal/core/lock.go, internal/core/paths.go | | `printf ok` | Confirms reusable acyclicity/frontier patterns, lock-ordering plan (spec lock before program lock), and completion predicate shared with submit |
| T2 | craftsman | internal/core/program.go, program_test.go | T1 | `go test ./internal/core -run TestProgramState -race -count=1` | `.specd/program.json` store: links list, schemaVersion per spec 02 discipline, AtomicWrite, own lock file with documented acquisition order; never touches spec state.json (R6) |
| T3 | craftsman | internal/core/program.go, program_test.go | T2 | `go test ./internal/core -run TestProgramGraph -race -count=1` | Cycle detection refusing with printed cycle path; program frontier = specs with all deps complete; completion predicate shared with submit's all-green check (R2, R4 core) |
| T4 | craftsman | internal/core/commands.go, internal/cmd/link.go, internal/cmd/registry.go + tests | T3 | `go test ./internal/cmd -run 'TestLink|TestUnlink' -race -count=1` | `link <from> <to>` / `unlink <from> <to>` verbs (phase metadata: any); nonexistent slug or nonexistent link exits 2; cycle exits 1 with path (R1, R2, R3) |
| T5 | craftsman | internal/cmd/status.go + tests | T3 | `go test ./internal/cmd -run TestStatusProgram -race -count=1` | `status --program`: all specs, links, phases, actionable frontier set (R4) |
| T6 | craftsman | internal/core/gates/approval.go + tests | T3 | `go test ./internal/core/gates -run TestApprovalProgramDeps -race -count=1` | Approval into execution refused while deps incomplete, blocking specs named; planning phases unblocked (R5) |
| T7 | craftsman | scripts/stress-program.sh | T4 | `./scripts/stress-program.sh` | Cross-process concurrent link/unlink leaves program.json consistent (no lost updates, no deadlock with spec locks) |
| T8 | craftsman | docs/command-reference.md, docs/CHEATSHEET.md, docs/concepts.md, docs/decisions/ (schedule/tick skip ADR) | T4,T5,T6 | `./scripts/docs-lint.sh` | Link semantics (execute-gated, planning-free), program frontier concept, schedule/tick skip ADR |
| T9 | validator | (read-only) | T6,T7 | `go test ./... -race -count=2` | Full suite green twice |
