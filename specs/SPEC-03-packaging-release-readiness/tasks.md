# SPEC-03 Tasks: Packaging & Release Readiness

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-03-01 | Disabled-mode perf assertion | Assert disabled-mode context-manifest build does O(0) work (A4). Build on SPEC-01's minimal perf-gate. | A test/gate fails if disabled-mode build does any measurable work; green on current HEAD. | Medium | completed |
| T-03-02 | DAG/frontier benchmarks | Add Go benchmarks for DAG build and frontier recompute across rising task counts; check for sub-quadratic scaling and no N+1 reads. | `go test -bench` runs; frontier scaling is sub-quadratic; numbers recorded. | Medium | completed |
| T-03-03 | Cleanup determinism test | Assert locks released and temp files removed on verify failure and on `--revert-on-fail`. | Test passes; fails if any lock/temp artifact leaks. | Small | completed |
| T-03-04 | Scale-envelope doc | Document max tasks/spec, max specs/program, with backing benchmark numbers. | Doc published with concrete limits + measured numbers; linked from docs index. | Small | completed |
| T-03-05 | Audit & harden release.yml | Verify reproducible/static build, artifact checksums (and signing if feasible), correct triggers; fix Go-version claims in release docs. | release.yml audited; artifacts carry integrity metadata; a dry-run/tag succeeds; no wrong version claims remain. | Medium | completed |

## Task Dependency Graph

```
T-03-01 (parallel)
T-03-02 ─→ T-03-04
T-03-03 (parallel)
T-03-05 (parallel)
```
T-03-04 needs the benchmark numbers from T-03-02. The rest are independent.

## Completion (2026-07-09)

All 5 tasks completed. Verify: `go test -bench=. -benchmem ./internal/core/... ./internal/context/...`
runs; `go test ./... -race -count=1` green; version claims corrected. Additions:
- **T-03-01** — `TestCheckBudgetDisabledZeroAllocs` (`internal/context/perf_test.go`): disabled-mode
  budget check is 0 allocs/op (measurable O(0)/A4). Behavioural pin already in `budget_test.go`.
- **T-03-02** — benchmarks `BenchmarkFrontier`, `BenchmarkProjectWaves` (`internal/core/bench_test.go`),
  `BenchmarkBuildManifest` (`internal/context/perf_test.go`). `TestFrontierScalesSubQuadratically`
  asserts 4× tasks ⇒ < 9× time (measured ~4.8×). `TestBuildManifestNoN1FileReads` proves manifest
  item count is independent of task count (no N+1). Numbers recorded in the scale-envelope doc.
- **T-03-03** — `TestVerifyFailureLeavesCleanTree` (`internal/cmd/verify_test.go`): failed verify
  under `--revert-on-fail` restores tracked state and leaks no `.orig`/`.rej`/tmp/`specd.lock`.
- **T-03-04** — published `docs/scale-envelope.md` (limits: ≤500 tasks/spec, ≤100 specs/program)
  with the measured benchmark numbers; linked from `docs/README.md`.
- **T-03-05** — audited `release.yml`; found the missing GoReleaser config (release would fail).
  Added `.goreleaser.yaml`: static (`CGO_ENABLED=0`) + `-trimpath -s -w`, `mod_timestamp`
  reproducibility, sha256 `checksums.txt`, per-archive SBOM. Corrected stale "1.22+" Go floor to
  "1.26+" (SPEC-01's authoritative floor) in `docs/user-guide.md` + `docs/concepts.md`.
