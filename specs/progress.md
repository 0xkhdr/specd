# Spec Progress Tracker

Phase 2 output of the specd production-readiness initiative. Source of truth for cross-domain
task status. Seven specs derived 1:1 from the approved `analysis-plan.md` "Recommended Spec
Breakdown" (Domains 3+release fold into SPEC-03; Domains 4+7 fold into SPEC-07).

| Spec ID | Domain | Priority | Status | Blocked By | Completed Tasks / Total |
|---------|--------|----------|--------|------------|------------------------|
| SPEC-01 | CI/CD & Build Tooling | P0 | verified | — | 7/7 |
| SPEC-02 | Feature ↔ Doc Regression | P1 | completed | SPEC-01 | 6/6 |
| SPEC-03 | Packaging & Release Readiness | P1 | completed | SPEC-01 | 5/5 |
| SPEC-04 | Security Tooling Hardening | P1 | completed | SPEC-01 | 6/6 |
| SPEC-05 | Test Coverage Formalization | P2 | completed | SPEC-01, SPEC-02 | 6/6 |
| SPEC-06 | Observability & Crash-Safety | P2 | completed | SPEC-01 | 5/5 |
| SPEC-07 | DX & Doc Accuracy | P2 | completed | SPEC-01, SPEC-02 | 6/6 |

Total: 41/41 tasks. **Initiative fully closed and verified end-to-end.**

SPEC-01 verified 2026-07-09: the definitive all-green **real push/PR** hosted CI run is recorded —
PR #38 (`fresh-start → main`), run 28982515514 against commit
`d4d69a9e7e0821467cf8566419fe8ed761024149`, **16/16 checks green** (all matrix legs
ubuntu/macos/windows + every job). Two runner-only golangci-lint failures were fixed in-flight
(`golangci-lint-action@v6→v7`; `install-mode: goinstall` so the linter builds with the runner's
Go 1.26 instead of the go1.24-built release binary) — CI-toolchain fixes only, no code/invariant
change. SPEC-01 `completed → verified`; the initiative is now fully verified end-to-end (41/41).

Wave 2 fully closed 2026-07-09: SPEC-06 (5/5), SPEC-05 (6/6), SPEC-07 (6/6) landed. Observability
regression-tested (Prometheus validity, history ordering, HUD, exit-code/error-doc drift guard) and
documented (`docs/observability.md`); coverage floor set to a real policy target (75.0%, measured
75.7%) with `TESTING.md` authored and the `ci.yml` reference resolved; the doc-drift class closed
permanently — the gate count and Go floor are now lint-enforced from single sources, orphan scripts
swept, and `CHANGELOG.md`/`CONTRIBUTING.md`/`docs/versioning-policy.md` shipped. This closes the
production-readiness initiative (41/41). The final user-gated SPEC-01 real push/PR CI run is now
recorded (PR #38, run 28982515514, 16/16 green) and SPEC-01 is `verified` — the initiative is
closed end-to-end. No LLM in any gate/report path, no evidence-bypass, zero runtime deps,
`reference/` untouched.

Wave 1 fully closed 2026-07-09: SPEC-03 (5/5) and SPEC-04 (6/6) landed alongside the earlier
SPEC-02. SPEC-04 regression-hardened the opt-in security gate (scan-boundary, fail-closed
allowlist, slug traversal, verify env-scrub) and shipped `SECURITY.md`; the `govulncheck@v1.5.0`
pin was already applied by SPEC-01 T-01-06. SPEC-03 added perf benchmarks + a sub-quadratic
frontier assertion + O(0) disabled-mode proof, published `docs/scale-envelope.md`, and hardened
release packaging with a new `.goreleaser.yaml` (static/reproducible build, checksums, SBOM).
No evidence-bypass flag added; `reference/` untouched; zero runtime deps preserved.

SPEC-02 completed 2026-07-09: verified verb → handler → doc map (23 verbs, zero unmatched),
gate count normalized to 14 everywhere, deferred/fail-closed/slug-position invariants pinned by
tests. This satisfies SPEC-05 and SPEC-07's dependency on a verified verb/feature map. SPEC-07
still owns the durable "12 core" drift-guard lint (SPEC-02 fixed the number; SPEC-07 builds the
guard) and the dead-script sweep incl. `scripts/stress-brain.sh`.

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
