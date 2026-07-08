# SPEC-03: Packaging & Release Readiness

## Overview
- **Domain:** Performance & Scalability (Analysis Plan Domain 3) + Release packaging
- **Risk Level:** Low (local scale is small; single static binary, local FS)
- **Priority:** P1
- **Dependencies:** SPEC-01 (perf-gate restoration overlaps SPEC-01; release validated on green CI).

## Current State

- **Perf gate unrunnable:** `ci.yml:107` comment declares a "measured perf gate — disabled-mode
  context manifest does no work" (the A4 invariant), but the step invokes `make perf-gate` which
  cannot run (no root Makefile — B1). SPEC-01 restores a *minimal runnable* gate; SPEC-03 owns the
  *real* perf assertion and scale envelope.
- **No published scale envelope:** there are no documented limits (max tasks/spec, max
  specs/program) and no benchmark numbers.
- **Release workflow unaudited:** `.github/workflows/release.yml` presence is confirmed but its
  internals (build reproducibility, artifact integrity/signing) were **not** audited this session.
- **Version-doc drift (B4):** `README.md`/`CLAUDE.md` claim a "1.22+" Go minimum contradicting the
  `go 1.26` floor. SPEC-01 fixes the *matrix vs go.mod* consistency; SPEC-03 owns correcting the
  *published version claims* in release-facing docs.

## Target State

A runnable, meaningful perf gate; a documented scale envelope with benchmark numbers; a validated
release pipeline that produces integrity-verifiable artifacts; and version claims that match the
real floor.

## Scope Boundaries

- **In Scope:** perf benchmark(s) for `dag.go`/`frontier.go`/`phases.go`/`context/`; the perf-gate
  CI assertion (building on SPEC-01's minimal gate); a scale-envelope doc; audit + hardening of
  `.github/workflows/release.yml`; correction of Go-version claims in release-facing docs.
- **Out of Scope:** authoring the *missing CI scripts themselves* (SPEC-01); orchestration
  crash-safety stress (SPEC-06); general doc accuracy sweep (SPEC-07); anything under `reference/`.

## Technical Requirements

1. **Perf assertions:**
   - Disabled-mode context build performs **O(0)** work (A4 invariant) — assert measurably.
   - DAG frontier computation scales with task count **without quadratic blowup** — a benchmark
     across increasing task counts must show sub-quadratic growth.
   - No N+1 file reads during context-manifest assembly.
   - Deterministic resource cleanup: locks released and temp files removed on verify failure /
     `--revert-on-fail`.
2. **Benchmarks:** add Go benchmarks (`func Benchmark…`) covering DAG build and frontier recompute;
   record representative numbers.
3. **Scale envelope doc:** publish intended limits (max tasks/spec, max specs/program) and the
   measured numbers backing them.
4. **release.yml audit:** verify the build is reproducible/static, artifacts carry integrity
   (checksums and, if feasible, signatures), and the release triggers correctly. Fix gaps found.
5. **Version claims:** correct "1.22+" (and similar) in release-facing docs to the real floor
   settled by SPEC-01.

## Verification Strategy

- `go test -bench=. -benchmem ./…` runs the new benchmarks; frontier benchmark demonstrates
  sub-quadratic scaling across task counts.
- The perf-gate CI step runs (green) and fails if disabled-mode context build does any work.
- A cleanup test asserts locks/temp files are gone after an induced verify failure and after
  `--revert-on-fail`.
- release.yml produces artifacts with published checksums; a dry-run (or tag on a throwaway ref)
  succeeds.
- `grep` confirms no remaining incorrect Go-version claims in release-facing docs.
- No LLM in gate/DAG/report paths; no bypass flag; `reference/` untouched.

## References
- Analysis Plan: Domain 3; Cross-Cutting Concern 4 (supply chain / release signing);
  Recommended Spec Breakdown row SPEC-03.
- Related Specs: SPEC-01 (perf-gate mechanism, version floor), SPEC-04 (govulncheck / supply
  chain), SPEC-06 (crash-safety).
- Source Files: `internal/core/dag.go`, `frontier.go`, `phases.go`, `internal/context/`,
  `internal/orchestration/`, `.github/workflows/release.yml`, `.github/workflows/ci.yml`.
