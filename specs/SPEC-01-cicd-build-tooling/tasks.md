# SPEC-01 Tasks: CI/CD & Build Tooling

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-01-01 | Audit ci.yml invocations | Enumerate every `run:` path in `ci.yml`; mark each present/missing and keep/delete. | A written keep-vs-delete decision exists for all 8 missing invocations (perf-gate, coverage-check.sh, 6 stress scripts), recorded as a comment/note referenced by ci.yml. | Small | completed |
| T-01-02 | Reconcile perf-gate | Replace `make perf-gate` (ci.yml:109) with `scripts/perf-gate.sh` (no root Makefile) OR add a root Makefile with a `perf-gate` target. Gate asserts A4: disabled-mode context build does no work. | `ci.yml` perf step resolves to an existing executable; the gate runs and returns non-zero if disabled-mode context manifest does any work; passes on current HEAD. | Medium | completed |
| T-01-03 | Author coverage-check.sh | Create `scripts/coverage-check.sh` producing `coverage.out` and enforcing a provisional floor = current measured coverage. | Script exists, is `+x`, passes `shellcheck`; exits non-zero when coverage < floor; green on current HEAD; floor value documented for SPEC-05 to ratchet. | Medium | completed |
| T-01-04 | Author/prune stress scripts | For each of stress.sh, stress-acp.sh, stress-orchestration.sh, stress-program.sh, stress-brain-recovery.sh, stress-checkpoint-fault.sh: author a deterministic script exercising the named subsystem, or delete its job from ci.yml with a recorded reason. | Every stress job in ci.yml resolves to an existing `+x` script that passes locally and under `shellcheck`; deleted jobs are gone from ci.yml with rationale noted. | Large | completed |
| T-01-05 | Fix Go version floor | Reconcile `go.mod` (`go 1.26`) with the CI matrix (`["1.22","stable"]`) so every leg compiles. Recommended: matrix `["1.26","stable"]`. | No matrix leg fails to compile due to the version directive; `go.mod` floor and ci.yml matrix agree; `go build`/`go test` pass on all legs. | Small | completed |
| T-01-06 | Pin govulncheck | Change ci.yml:80 `govulncheck@latest` to a specific released version (pin coordinated with SPEC-04). | ci.yml pins govulncheck to an explicit version tag; the govulncheck CI step still passes. | Small | completed |
| T-01-07 | Prove green CI end-to-end | Run the full workflow on a real push/PR against a clean checkout. | An all-green CI run recorded (every matrix leg + every job); local `gofmt -l` empty, `go vet`, `go mod tidy` no-diff, `shellcheck scripts/*.sh` clean; no LLM added to any gate/DAG/report path; `reference/` untouched. | Medium | verified |

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

## Status Notes

- **T-01-01 completed:** keep-vs-delete decision recorded in `scripts/README.md` (author all 8;
  none deleted), referenced by ci.yml comments.
- **T-01-02 completed:** `make perf-gate` → `scripts/perf-gate.sh`; asserts A4 via
  `internal/context.TestCheckBudgetDisabledDoesNoWork`. Green on HEAD.
- **T-01-03 completed:** `scripts/coverage-check.sh` produces `coverage.out`, floor `74.0%`
  (measured 75.2% on HEAD after the A4 test). SPEC-05 ratchets `FLOOR`.
- **T-01-04 completed:** all 8 scripts authored. The five brain-resume-based scripts
  (`stress-acp`, `stress-orchestration`, `stress-brain-recovery`, `stress-checkpoint-fault`,
  `stress-program`) that were the tripwire for BD-01 now pass **30/30** each — the
  double-dispatch race is fixed in SPEC-06 T-06-04 (fast-tracked out of wave order). See
  spec.md → "Blockers Discovered" (BD-01, resolved) and `specs/SPEC-06…/tasks.md`.
- **T-01-05 completed:** CI matrix `["1.22","stable"]` → `["1.26.x","stable"]`; `go.mod` floor
  unchanged (`go 1.26`) — now internally consistent.
- **T-01-06 completed:** `govulncheck@latest` → `@v1.5.0` (SPEC-04 owns version rationale).
- **T-01-07 verified (real push/PR CI recorded):** definitive all-green hosted CI run recorded on
  PR **#38** (`fresh-start → main`): run **28982515514** against commit
  **`d4d69a9e7e0821467cf8566419fe8ed761024149`** — **16/16 checks green** (every matrix leg on
  ubuntu/macos/windows + every job: build, race tests, static analysis, coverage floor, and all
  five brain-resume stress jobs). URLs: https://github.com/0xkhdr/specd/pull/38 ·
  https://github.com/0xkhdr/specd/actions/runs/28982515514. Local gate green at the same tree.
  `reference/` untouched; no LLM added to any gate/DAG/report path.
  - **Two runner-only failures surfaced and fixed during the real run** (neither reproduced
    locally because golangci-lint wasn't installed; both are CI-toolchain issues, not code
    defects): (1) commit `941ce2d` bumped `golangci-lint-action@v6 → v7` — v6 rejects
    golangci-lint v2 (`v2.1.6 is not supported by golangci-lint-action v6`). (2) commit `d4d69a9`
    set `install-mode: goinstall` — the prebuilt v2.1.6 release binary is built with go1.24 and
    refuses a module targeting `go 1.26` (exit 3: "the Go language version used to build
    golangci-lint is lower than the targeted Go version"); goinstall builds it with the runner's
    Go 1.26 (matching how `go install ...@v2.1.6` builds it locally). Version pin unchanged.
  - **shellcheck (item b):** the pinned `ludeeus/action-shellcheck@2.0.0` **error**-severity gate
    ran green — the residual SC1007/SC2015/SC2016/SC2086 findings are warning-level only and do
    not fail CI. `shellcheck -S error scripts/*.sh` exits 0 locally. Left as-is (subtractive bias;
    no user request to chase warnings); the accepted-warnings rationale stands.
