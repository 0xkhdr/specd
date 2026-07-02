# S15 Tasks — Fuzz & Parser Regression

Requirement coverage: R1–R5 robustness. Dependencies: none (Wave-1 root).

## Wave 1 — Baseline

- [ ] Confirm existing fuzz precedent and parser tests. Files:
  `internal/mcp/host_caps_fuzz_test.go`, `internal/core/tasksparser_test.go`,
  `state_test.go`, `ears_test.go`, `specfiles_test.go`.
- [ ] Collect seed inputs (valid + known-tricky) for each target.
- **Validation:** `go test ./internal/core/... -race -count=1`

## Wave 2 — Fuzz targets (depends on Wave 1)

- [ ] `FuzzParseTasks` for `tasksparser.go`. File:
  `internal/core/tasksparser_fuzz_test.go` (new).
- [ ] `FuzzLoadState` for `state.go` `LoadState`. File:
  `internal/core/state_fuzz_test.go` (new).
- [ ] `FuzzLintEars` for `ears.go` `LintEars`. File:
  `internal/core/ears_fuzz_test.go` (new).
- [ ] `FuzzSpecFiles` for `specfiles.go`. File:
  `internal/core/specfiles_fuzz_test.go` (new).
- **Validation:** `go test ./internal/core/... -run Fuzz -count=1`

## Wave 3 — Time-boxed fuzzing (depends on Wave 2)

- [ ] Run each target `-fuzztime 60s`; assert no crash. Triage findings.
- [ ] File issues for non-critical findings; fix any panic/corruption.
- **Validation:** `go test ./internal/core/ -fuzz FuzzParseTasks -fuzztime 60s`
  (repeat per target)

## Rollout & cleanup

- [ ] Commit discovered crash corpus under `testdata/fuzz/` (regression seeds).
- [ ] Confirm `internal/core` ≥80% floor still met (`make cover-check`).
- **Rollback:** delete new `*_fuzz_test.go`.
- **Completion evidence:** crash-free 60s fuzz per parser + committed corpus.
