# SPEC-02 Tasks: Feature ↔ Doc Regression

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-02-01 | Build verb→handler→doc map | Enumerate all 23 verbs from `commands.go`; match each to a `registry.go` handler and a `command-reference.md` entry. | A complete table with zero unmatched rows; any mismatch flagged for T-02-05. | Medium | pending |
| T-02-02 | Deferred-verb regression | Add a test asserting `triage` prints a deferral notice and exits 0. | Test passes; fails if `triage` silently no-ops or exits non-zero. | Small | pending |
| T-02-03 | Fail-closed regression | Add tests asserting an unknown verb exits 2 and a bad flag value exits 2. | Both tests pass and fail if exit code changes. | Small | pending |
| T-02-04 | Slug-position regression | Assert each verb reads the slug from the correct argv index (`brain`→argAt(1), others→argAt(0)). | Test passes; fails if any verb's slug index is wrong. | Medium | pending |
| T-02-05 | Orphan sweep | Flag handlers with no doc entry and documented behavior with no handler (incl. brain_worker.go/dispatch.go sub-behaviors); resolve each. | Every handler documented or recorded as intentionally internal; no documented behavior lacks a handler. | Medium | pending |
| T-02-06 | Normalize gate count to 14 | Replace all "12 core gates" (README:15, README:74, elsewhere) with 14 per authoritative `validation-gates.md`; file a drift-guard request for SPEC-07. | `grep -rn "12 core"` returns nothing; README internally consistent at 14; docs-lint green. | Small | pending |

## Task Dependency Graph

```
T-02-01 ─→ T-02-05
T-02-02 (parallel)
T-02-03 (parallel)
T-02-04 (parallel)
T-02-06 (parallel)
```
T-02-01 must precede the orphan sweep (T-02-05). The four regression tests and the gate-count
fix are independent and can run concurrently.
