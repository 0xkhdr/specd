# SPEC-07 Tasks: DX & Doc Accuracy

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-07-01 | Runnable-example check | Author a check (script + CI step or documented cadence) running each documented command example verbatim against a fresh `specd init`'d project. Uses SPEC-02's map. | Every documented example runs green against a fresh init; check runs in CI or a cadence is documented. | Large | pending |
| T-07-02 | Dead-script sweep | Remove or wire-in scripts no workflow references (`stress-brain.sh`, `verify-progress.sh`, others); record per-script decision. Never touch `reference/`. | No orphan scripts remain unaddressed; each kept script is referenced; `reference/` untouched. | Small | pending |
| T-07-03 | CHANGELOG + versioning policy | Author a CHANGELOG and a short versioning-policy doc (cut process, Go floor). | Both files exist, accurate, linked from docs index. | Small | pending |
| T-07-04 | CONTRIBUTING quick-start | Add a lightweight CONTRIBUTING distinct from `contributor-guide.md`. | CONTRIBUTING exists with a fast onboarding path; linked from README. | Small | pending |
| T-07-05 | Drift-guard lint | Extend the `docs-lint.sh` pattern so gate count + Go-version string are lint-enforced from one authoritative source. | Lint fails on an intentional mismatch (proven with a temp edit), passes when consistent; wired into CI. | Medium | pending |
| T-07-06 | Invariant + version-claim sync | Confirm `contributor-guide.md` §3 matches post-SPEC-01 tooling; fix general doc-body "1.22+" claims to the real floor. | §3 accurate; `grep` finds no wrong Go-version claims in doc bodies. | Small | pending |

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
