# Spec — Context-Manifest Measured Zero-Overhead Gate (A4)

**Priority:** P1 · **Wave:** 1 · **Domain:** perf-regression gating.

## Introduction

`context-manifest-zero-overhead` asserts zero/false overhead when the manifest
is disabled. Today that claim is guarded structurally (the disabled path
short-circuits), but there is no **measured** guard to catch a silent regression
where disabled-mode starts doing work (allocations, file reads, token cost).

This spec adds a deterministic, measured guard so the "zero overhead when
disabled" claim cannot regress unnoticed.

## Current-state grounding

- `specs/fusion/context-manifest-zero-overhead` — claims zero/false overhead when
  disabled; structural guards only.
- `internal/context/` — context/manifest assembly lives here (no per-package
  coverage floor today).
- No `make perf-gate` target exists; CI has stress jobs but no overhead micro-gate.

## Requirements

### Requirement 1 — Disabled-mode does no manifest work
**User story:** As a maintainer, I want proof that disabled-mode performs no
manifest assembly, so the zero-overhead claim is measured not asserted.

**Acceptance criteria:**
1. A deterministic test SHALL assert that, with the manifest disabled, the
   manifest builder is not invoked / performs no file reads.
2. The assertion SHALL be deterministic (counter/spy or `testing.AllocsPerRun`),
   NOT a wall-clock threshold.

### Requirement 2 — Allocation/work budget guard
**User story:** As a maintainer, I want a budget that fails CI if disabled-mode
starts allocating.

**Acceptance criteria:**
1. `testing.AllocsPerRun` (or equivalent) SHALL bound disabled-mode allocations
   at the documented budget (target: 0 manifest-attributable allocs).
2. The guard SHALL fail if a future change reintroduces work on the disabled path.

### Requirement 3 — Wired into the gate path
**User story:** As a maintainer, I want this guard to run in CI like other gates.

**Acceptance criteria:**
1. A `make perf-gate` (or equivalent existing target) SHALL run the guard.
2. CI SHALL invoke it so regressions block merge.

## Design

- Add a spy/counter around the manifest builder (or a hook the test can read) to
  observe invocation count and file-read count on the disabled path.
- Use `testing.AllocsPerRun(N, fn)` to bound allocations deterministically.
- Add `perf-gate` target to the Makefile that runs the focused test; wire into
  CI alongside existing gates.

## Out of scope

- Benchmarking the *enabled* manifest path's cost.
- Wall-clock latency SLAs (non-deterministic; rejected on purpose).

## Risks

- **Flaky perf gate:** wall-clock is banned; allocation/invocation counters are
  deterministic and CI-safe.
- **Spy overhead skews result:** keep the spy out of the measured closure.
