# SPEC-01 Tasks: CI/CD & Build Tooling

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-01-01 | Audit ci.yml invocations | Enumerate every `run:` path in `ci.yml`; mark each present/missing and keep/delete. | A written keep-vs-delete decision exists for all 8 missing invocations (perf-gate, coverage-check.sh, 6 stress scripts), recorded as a comment/note referenced by ci.yml. | Small | pending |
| T-01-02 | Reconcile perf-gate | Replace `make perf-gate` (ci.yml:109) with `scripts/perf-gate.sh` (no root Makefile) OR add a root Makefile with a `perf-gate` target. Gate asserts A4: disabled-mode context build does no work. | `ci.yml` perf step resolves to an existing executable; the gate runs and returns non-zero if disabled-mode context manifest does any work; passes on current HEAD. | Medium | pending |
| T-01-03 | Author coverage-check.sh | Create `scripts/coverage-check.sh` producing `coverage.out` and enforcing a provisional floor = current measured coverage. | Script exists, is `+x`, passes `shellcheck`; exits non-zero when coverage < floor; green on current HEAD; floor value documented for SPEC-05 to ratchet. | Medium | pending |
| T-01-04 | Author/prune stress scripts | For each of stress.sh, stress-acp.sh, stress-orchestration.sh, stress-program.sh, stress-brain-recovery.sh, stress-checkpoint-fault.sh: author a deterministic script exercising the named subsystem, or delete its job from ci.yml with a recorded reason. | Every stress job in ci.yml resolves to an existing `+x` script that passes locally and under `shellcheck`; deleted jobs are gone from ci.yml with rationale noted. | Large | pending |
| T-01-05 | Fix Go version floor | Reconcile `go.mod` (`go 1.26`) with the CI matrix (`["1.22","stable"]`) so every leg compiles. Recommended: matrix `["1.26","stable"]`. | No matrix leg fails to compile due to the version directive; `go.mod` floor and ci.yml matrix agree; `go build`/`go test` pass on all legs. | Small | pending |
| T-01-06 | Pin govulncheck | Change ci.yml:80 `govulncheck@latest` to a specific released version (pin coordinated with SPEC-04). | ci.yml pins govulncheck to an explicit version tag; the govulncheck CI step still passes. | Small | pending |
| T-01-07 | Prove green CI end-to-end | Run the full workflow on a real push/PR against a clean checkout. | An all-green CI run recorded (every matrix leg + every job); local `gofmt -l` empty, `go vet`, `go mod tidy` no-diff, `shellcheck scripts/*.sh` clean; no LLM added to any gate/DAG/report path; `reference/` untouched. | Medium | pending |

## Task Dependency Graph

```
T-01-01 ──┬─→ T-01-02 ──┐
          ├─→ T-01-03 ──┤
          └─→ T-01-04 ──┤
T-01-05 ───────────────┤
T-01-06 ───────────────┴─→ T-01-07
```
T-01-01 gates the three authoring tasks (they need the keep/delete decision). T-01-05 and
T-01-06 are independent and run in parallel. T-01-07 is the final integration proof.
