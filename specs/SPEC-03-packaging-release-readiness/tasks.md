# SPEC-03 Tasks: Packaging & Release Readiness

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-03-01 | Disabled-mode perf assertion | Assert disabled-mode context-manifest build does O(0) work (A4). Build on SPEC-01's minimal perf-gate. | A test/gate fails if disabled-mode build does any measurable work; green on current HEAD. | Medium | pending |
| T-03-02 | DAG/frontier benchmarks | Add Go benchmarks for DAG build and frontier recompute across rising task counts; check for sub-quadratic scaling and no N+1 reads. | `go test -bench` runs; frontier scaling is sub-quadratic; numbers recorded. | Medium | pending |
| T-03-03 | Cleanup determinism test | Assert locks released and temp files removed on verify failure and on `--revert-on-fail`. | Test passes; fails if any lock/temp artifact leaks. | Small | pending |
| T-03-04 | Scale-envelope doc | Document max tasks/spec, max specs/program, with backing benchmark numbers. | Doc published with concrete limits + measured numbers; linked from docs index. | Small | pending |
| T-03-05 | Audit & harden release.yml | Verify reproducible/static build, artifact checksums (and signing if feasible), correct triggers; fix Go-version claims in release docs. | release.yml audited; artifacts carry integrity metadata; a dry-run/tag succeeds; no wrong version claims remain. | Medium | pending |

## Task Dependency Graph

```
T-03-01 (parallel)
T-03-02 ─→ T-03-04
T-03-03 (parallel)
T-03-05 (parallel)
```
T-03-04 needs the benchmark numbers from T-03-02. The rest are independent.
