# Spec — Testing & Reliability (S4)

## Introduction

specd's test strategy is more mature than the analysis plan assumed: race
detection is mandatory (`go test ./... -race -count=1`), order-dependence is
guarded by a `-count=2` rerun, and coverage enforcement is an 11-package
regression-only ratchet (floors from 70% `internal/cmd` to 95%
`internal/spec`) via `scripts/coverage-check.sh`, not a single blanket number.
Live evidence confirmed one genuine, unguarded gap (D15): the six
`stress*` Makefile targets invoke their scripts with no `ulimit`/timeout
bounds, and none of the `scripts/stress*.sh` files set any resource limit
internally. This spec adds resource bounds to stress targets and raises
specific package coverage floors where evidence supports it, rather than a
blanket floor increase.

## Requirement 1 — Bounded stress targets

**User story:** As a contributor running `make stress` locally, I want a
runaway fault-injection scenario to fail loudly within a bounded time/resource
budget instead of exhausting my machine, so a bug in the stress harness
doesn't turn into a frozen laptop.

**Acceptance criteria:**
1. WHEN any of `stress`, `stress-acp`, `stress-orchestration`, `stress-program`,
   `stress-brain-recovery`, `stress-checkpoint-fault` runs THE SYSTEM SHALL
   enforce a wall-clock timeout (via `timeout` or equivalent) that fails the
   target with a clear "stress target exceeded budget" message rather than
   hanging indefinitely.
2. THE SYSTEM SHALL enforce a process-count and/or open-file-descriptor
   `ulimit` on each stress target sized to comfortably exceed legitimate
   usage (determined from the existing stress script's expected concurrency)
   but catch a goroutine/process leak before it exhausts the host.
3. WHEN a stress target is intentionally testing a long-running scenario
   (if any are) THE SYSTEM SHALL document the chosen timeout's rationale in
   the Makefile target or the script itself — not pick an arbitrary number
   silently.
4. THE SYSTEM SHALL NOT change any stress scenario's actual fault-injection
   logic or assertions — this requirement only wraps execution with resource
   bounds.

## Requirement 2 — Resource leak detection

**User story:** As a maintainer, I want goroutine and file-descriptor counts
checked before/after stress runs, so a slow leak is caught by CI instead of
surfacing as a production hang months later.

**Acceptance criteria:**
1. THE SYSTEM SHALL record goroutine count (`runtime.NumGoroutine()`) before
   and after each stress scenario completes, failing the target if the count
   grows beyond a documented tolerance after a settling period.
2. WHERE a stress scenario opens files/sockets, THE SYSTEM SHALL verify they
   are closed after the scenario completes (e.g., via `lsof`-style check or
   an in-process open-handle counter, whichever fits the existing harness).

## Requirement 3 — Targeted coverage floor increases

**User story:** As a maintainer, I want coverage floors raised specifically
where this review found a real gap, not uniformly, so the ratchet stays
meaningful rather than becoming a number nobody trusts.

**Acceptance criteria:**
1. THE SYSTEM SHALL identify, via `go tool cover -func`, which of the 11
   floor-enforced packages has the largest gap between current coverage and
   its floor's stated target ceiling (e.g. `internal/cmd`'s floor is 70% with
   a stated target of 80% per `TESTING.md`).
2. THE SYSTEM SHALL raise that package's floor in
   `scripts/coverage-check.sh` only after adding tests that durably clear
   the new floor (not by lowering the bar to match accidental coverage).
3. THE SYSTEM SHALL NOT lower any existing floor (ratchet property,
   preserved per `CONTRIBUTING.md`'s "never lower coverage floor").

## Design

### Overview
Three independent workstreams: stress-target resource bounding (shell-level,
low risk), leak detection (additive instrumentation in stress scripts), and
one targeted coverage floor raise (requires new tests, highest effort).

### Architecture
No production code architecture change. `Makefile` stress targets and
`scripts/stress*.sh` gain wrapping (`timeout`, `ulimit`) and
before/after goroutine accounting. `scripts/coverage-check.sh` gains one
updated floor constant once its package clears the new bar.

### Components and interfaces
- `Makefile:50-66` — `stress*` targets wrapped with `timeout`/`ulimit`.
- `scripts/stress.sh` and the five other `scripts/stress-*.sh` — goroutine
  count check added around the scenario invocation.
- `scripts/coverage-check.sh` — one floor constant updated (package TBD by
  T1's investigation).

### Data models
No changes.

### Error handling
A timeout-exceeded or leak-detected stress run must fail with a message
identifying which check failed (timeout vs. ulimit vs. goroutine leak) —
not a generic non-zero exit that requires log-spelunking to diagnose.

### Verification strategy
- `make stress` and the five other stress targets, confirmed to still pass
  under the new bounds with the existing fault-injection scenarios.
- `scripts/coverage-check.sh` after the floor raise, confirmed green and
  confirmed to fail if the floor is reverted (sanity check, not committed).

### Risks and open questions
- Risk: an overly tight `ulimit`/timeout could make a legitimately slow
  (but correct) stress scenario flaky in CI. Mitigation: size bounds from
  the existing scripts' observed runtime/concurrency (T1's job), not a
  guess, and document the margin chosen.
- Open question: which package most needs a coverage floor raise — must be
  decided from actual `go tool cover -func` output (T4), not assumed.
