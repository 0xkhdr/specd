# SPEC-04 Tasks: Security Tooling Hardening

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-04-01 | Per-scanner fixture regression | Confirm/add tests that each scanner (secrets, injection, slopsquat) fires on its fixture and stays silent outside the scan boundary. | All three scanners proven to fire on-fixture and respect the boundary (excludes lockfiles/testdata/.specd/reference/vendor/.git). | Medium | completed |
| T-04-02 | Fail-closed allowlist test | Prove a corrupt/unloadable fingerprint allowlist makes the gate fail closed. | Test passes; a bad allowlist causes non-zero/error, never a silent pass. | Small | completed |
| T-04-03 | Slug traversal test | Prove slug validation rejects `../`, absolute paths, and separators, preventing escape from `.specd/specs/<slug>/`. | Traversal-attempt inputs rejected; test fails if any escape succeeds. | Small | completed |
| T-04-04 | Verify sandbox isolation | Document + test `--sandbox`/bwrap isolation for shell-executed verify lines; prove `--revert-on-fail` restores state and leaks no temp files; no secrets in logs. | Sandbox behavior documented; test proves isolation + clean revert; log scan shows no secrets. | Medium | completed |
| T-04-05 | Pin govulncheck | DONE: version `v1.5.0` chosen; applied by SPEC-01 T-01-06 (commit `a5e3935`) at `ci.yml:86`; corrected stale `@latest` claim in spec.md §Current State; confirmed slopsquat + govulncheck both run. | ci.yml pins govulncheck@v1.5.0; both supply-chain checks run green. | Small | completed |
| T-04-06 | Author SECURITY.md | Write a threat model (hostile spec/tasks/verify-line/dependency-name attacker model) + vulnerability-disclosure policy. | `SECURITY.md` exists with threat model and disclosure policy; linked from README/docs. | Medium | completed |

## Task Dependency Graph

```
T-04-01 (parallel)
T-04-02 (parallel)
T-04-03 (parallel)
T-04-04 (parallel)
T-04-05 ─→ (SPEC-01 T-01-06)
T-04-06 (parallel)
```
All authoring/test tasks are independent; T-04-05 feeds SPEC-01's govulncheck-pin task.

## Completion (2026-07-09)

All 6 tasks completed. Verify: `go test ./internal/core/gates/security/... -race -count=1` +
`go test ./... -race -count=1` green; `grep -n govulncheck .github/workflows/ci.yml` shows the
`@v1.5.0` pin (line 86) and the security gate runs `slopsquat`; `test -f SECURITY.md` passes.
Much of the trust-boundary coverage pre-existed (df76d4c: `traversal_test.go`, `sandbox_test.go`).
This spec's additions:
- **T-04-01** — extended the scan-boundary test (`scanner_test.go`) to every lockfile + every
  excluded dir (`testdata/`, `.specd/`, `reference/`, `vendor/`, `.git/`) at root/nested depth,
  plus true-negatives; per-scanner fire-on-fixture tests already present.
- **T-04-02** — added `corrupt_allowlist_fails_gate_closed` (`gate_test.go`): end-to-end proof
  that a corrupt allowlist surfaces an error-severity gate finding (never a silent pass); the
  `loadAllowlist` unit fail-closed cases already existed.
- **T-04-03** — extended `TestSlug` (`paths_test.go`) with explicit traversal inputs (`..`,
  `../x`, absolute paths, separators); verb-level `TestSlugTraversalRejected` already present.
- **T-04-04** — added `TestScrubbedEnvDropsSecrets` (`sandbox_test.go`) proving verify subprocess
  env is HOME/PATH/TMPDIR-only (no secret leak into logs); wrapArgv + fail-closed already pinned.
  Isolation contract documented in `SECURITY.md`.
- **T-04-05** — DONE (pin `@v1.5.0` applied by SPEC-01 T-01-06, commit `a5e3935`); corrected the
  stale `@latest` claim in `spec.md` §Current State + §Technical Requirements.
- **T-04-06** — authored `SECURITY.md` (threat model for hostile spec/verify/dependency content +
  isolation contract + disclosure policy), linked from `README.md`.
