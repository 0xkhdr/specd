# SPEC-07 Tasks: DX & Doc Accuracy

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-07-01 | Runnable-example check | Author a check (script + CI step or documented cadence) running each documented command example verbatim against a fresh `specd init`'d project. Uses SPEC-02's map. | Every documented example runs green against a fresh init; check runs in CI or a cadence is documented. | Large | completed |
| T-07-02 | Dead-script sweep | Remove or wire-in scripts no workflow references (`stress-brain.sh`, `verify-progress.sh`, others); record per-script decision. Never touch `reference/`. | No orphan scripts remain unaddressed; each kept script is referenced; `reference/` untouched. | Small | completed |
| T-07-03 | CHANGELOG + versioning policy | Author a CHANGELOG and a short versioning-policy doc (cut process, Go floor). | Both files exist, accurate, linked from docs index. | Small | completed |
| T-07-04 | CONTRIBUTING quick-start | Add a lightweight CONTRIBUTING distinct from `contributor-guide.md`. | CONTRIBUTING exists with a fast onboarding path; linked from README. | Small | completed |
| T-07-05 | Drift-guard lint | Extend the `docs-lint.sh` pattern so gate count + Go-version string are lint-enforced from one authoritative source. | Lint fails on an intentional mismatch (proven with a temp edit), passes when consistent; wired into CI. | Medium | completed |
| T-07-06 | Invariant + version-claim sync | Confirm `contributor-guide.md` §3 matches post-SPEC-01 tooling; fix general doc-body "1.22+" claims to the real floor. | §3 accurate; `grep` finds no wrong Go-version claims in doc bodies. | Small | completed |

## Task Dependency Graph

```
T-07-01 ─→ (depends on SPEC-02 map)
T-07-02 (parallel)
T-07-03 (parallel)
T-07-04 (parallel)
T-07-05 ─→ (guards SPEC-02 T-02-06 gate-count fix)
T-07-06 ─→ (depends on SPEC-01 version-floor decision)
```
T-07-01 consumes SPEC-02's verified example set; T-07-06 needs SPEC-01's settled version floor;
T-07-05 makes SPEC-02's gate-count fix permanent. The rest are independent.

## Status Notes

- **All 6 tasks completed (Wave 2).** Verified against a real git HEAD; local gates green
  (`gofmt`/`vet`/`go mod tidy`/`golangci-lint` clean, `docs-lint.sh` ok incl. the new drift guard,
  `go test ./... -race` 268 pass).
  - **T-07-01** — runnable-example check: `cmd.TestDocumentedExamplesRun` parses every concrete
    (placeholder-free) example from the command palette (`core.Commands`, the SPEC-02 SOT) with the
    real `cli.ParseArgs` and runs the read-surface examples verbatim against a fresh, executing
    spec. Runs in CI as part of the Go suite. Mutating/lifecycle examples stay covered by
    `TestLifecycleE2E`.
  - **T-07-02** — dead-script sweep: `stress-brain.sh` and `verify-progress.sh` **removed**
    (redundant with the wired `stress-brain-recovery.sh` + `TestBrainResumeRaceDispatchesExactlyOnce`,
    and with the CI Go suite respectively). `regress-*.sh` kept, cadence-run. Per-script decision log
    in `scripts/README.md`. `reference/` untouched.
  - **T-07-03** — `CHANGELOG.md` (Keep-a-Changelog) + `docs/versioning-policy.md` (SemVer, Go floor,
    cut process) authored; both linked from the docs index.
  - **T-07-04** — `CONTRIBUTING.md` quick-start authored (setup, gate loop, house rules), distinct
    from the architecture-heavy `contributor-guide.md`; linked from README.
  - **T-07-05** — drift guard added to `scripts/docs-lint.sh`: gate count enforced from
    `internal/core/gates/core.go` (`registry.Register(` count = 14), Go floor from `go.mod` (`go`
    directive = 1.26). Proven: a temp "12 core gates" edit and a temp "Go 1.22+" edit each fail the
    lint; reverted → passes. `docs-lint.sh` already runs in the CI lint job.
  - **T-07-06** — `contributor-guide.md` §3 confirmed accurate against post-SPEC-01 tooling; the one
    remaining doc-body stale claim (`CLAUDE.md:12` "Go 1.22+ … toolchain pins 1.26" — doubly wrong:
    `go.mod` is `go 1.26`, no `toolchain` line) corrected to "Requires Go 1.26+". `grep` finds no
    wrong Go-version claims in doc bodies; the drift guard now prevents recurrence.
