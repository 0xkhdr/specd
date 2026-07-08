# SPEC-04 Tasks: Security Tooling Hardening

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-04-01 | Per-scanner fixture regression | Confirm/add tests that each scanner (secrets, injection, slopsquat) fires on its fixture and stays silent outside the scan boundary. | All three scanners proven to fire on-fixture and respect the boundary (excludes lockfiles/testdata/.specd/reference/vendor/.git). | Medium | pending |
| T-04-02 | Fail-closed allowlist test | Prove a corrupt/unloadable fingerprint allowlist makes the gate fail closed. | Test passes; a bad allowlist causes non-zero/error, never a silent pass. | Small | pending |
| T-04-03 | Slug traversal test | Prove slug validation rejects `../`, absolute paths, and separators, preventing escape from `.specd/specs/<slug>/`. | Traversal-attempt inputs rejected; test fails if any escape succeeds. | Small | pending |
| T-04-04 | Verify sandbox isolation | Document + test `--sandbox`/bwrap isolation for shell-executed verify lines; prove `--revert-on-fail` restores state and leaks no temp files; no secrets in logs. | Sandbox behavior documented; test proves isolation + clean revert; log scan shows no secrets. | Medium | pending |
| T-04-05 | Pin govulncheck | Choose an explicit govulncheck version; hand to SPEC-01 (T-01-06) for application; confirm slopsquat + govulncheck both run in CI. | ci.yml pins govulncheck; both supply-chain checks run green. | Small | pending |
| T-04-06 | Author SECURITY.md | Write a threat model (hostile spec/tasks/verify-line/dependency-name attacker model) + vulnerability-disclosure policy. | `SECURITY.md` exists with threat model and disclosure policy; linked from README/docs. | Medium | pending |

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
