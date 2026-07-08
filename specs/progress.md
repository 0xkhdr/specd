# Spec Progress Tracker

Phase 2 output of the specd production-readiness initiative. Source of truth for cross-domain
task status. Seven specs derived 1:1 from the approved `analysis-plan.md` "Recommended Spec
Breakdown" (Domains 3+release fold into SPEC-03; Domains 4+7 fold into SPEC-07).

| Spec ID | Domain | Priority | Status | Blocked By | Completed Tasks / Total |
|---------|--------|----------|--------|------------|------------------------|
| SPEC-01 | CI/CD & Build Tooling | P0 | completed | — | 7/7 |
| SPEC-02 | Feature ↔ Doc Regression | P1 | pending | SPEC-01 | 0/6 |
| SPEC-03 | Packaging & Release Readiness | P1 | pending | SPEC-01 | 0/5 |
| SPEC-04 | Security Tooling Hardening | P1 | pending | SPEC-01 | 0/6 |
| SPEC-05 | Test Coverage Formalization | P2 | pending | SPEC-01, SPEC-02 | 0/6 |
| SPEC-06 | Observability & Crash-Safety | P2 | in-progress | SPEC-01 | 1/5 |
| SPEC-07 | DX & Doc Accuracy | P2 | pending | SPEC-01, SPEC-02 | 0/6 |

Total: 8/41 tasks.

**SPEC-01 complete; BD-01 resolved.** All 7 SPEC-01 tasks are done. The double-dispatch race in
`brain resume` that blocked T-01-04/T-01-07 was fixed by fast-tracking SPEC-06 T-06-04 out of
wave order (the fix lives in SPEC-06's domain: `internal/cmd/brain_run.go` +
`internal/core/lock.go`). Two root causes — a non-atomic resume critical section and a
false-stale removal of a mid-write lock file — both closed; the five brain-resume stress scripts
now pass 30/30 and a new race test asserts exactly-one dispatch under `-race`. Details in
`SPEC-06…/tasks.md` → T-06-04 and `SPEC-01…/spec.md` → "Blockers Discovered" (resolved).
**One item remains for T-01-07:** the definitive all-green **real push/PR** CI run is not yet
recorded (all local gates are green); see `SPEC-01…/tasks.md` T-01-07 note.

## Wave Plan

- **Wave 0 (blocking):** SPEC-01. Nothing else can be trusted until CI is green — every other
  spec verifies its work through CI.
- **Wave 1 (parallel, after SPEC-01):** SPEC-02, SPEC-03, SPEC-04. Independent of each other.
- **Wave 2 (after Wave 1):** SPEC-05 (needs SPEC-02's verb map), SPEC-06, SPEC-07 (needs
  SPEC-02's verified feature map).

## Status Legend

- `pending` — Not started
- `in-progress` — Tasks actively being worked
- `blocked` — Waiting on dependency or external input
- `completed` — All tasks done, spec verified
- `verified` — Independently validated in a production-like environment (green CI on a real push/PR)

## Global Non-Negotiables (apply to every spec)

Copied into each spec's acceptance criteria; restated here so they can't be missed:

1. **No LLM** in any gate, DAG, or report path. No evidence-bypass flag may be added.
2. **`reference/`** is a frozen museum — no spec builds, imports, edits, copies from, or specs
   against it.
3. **Zero runtime dependencies** — `go.mod`/`go.sum` stay tidy (`go mod tidy` produces no diff).
4. **Path discipline** — `.specd/specs/` is runtime; top-level `specs/` (this tree) is planning.
   No task may confuse them (`regress-lint.sh` smell "A" guards this).
5. **Evidence integrity** — a task completes only against a passing verify record pinned to a
   real git HEAD.
