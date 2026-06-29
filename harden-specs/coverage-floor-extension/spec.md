# Spec — Coverage-Floor Extension (A8)

**Priority:** P2 · **Wave:** 3 · **Domain:** test-coverage ratchet.

## Introduction

`scripts/coverage-check.sh` floors OVERALL (77), `core` (86), `cmd` (71),
`worker` (88), `mcp` (88), `testharness` (80). The measured low spots sit
**outside** the per-package floors: `internal/spec` (~50%, only 4 tests;
`role.go` is the substantive untested file), plus `internal/runner`,
`internal/pack`, `internal/obs`, `internal/context`, `internal/schema` have no
individual floor. A regression in an unfloored package can pass CI as long as the
77% overall holds.

This spec extends the ratchet to the unguarded packages and raises
`internal/spec`, advancing toward the 85/90/95 targets in `TESTING.md`.

## Current-state grounding

- `scripts/coverage-check.sh` — current per-package floors listed above.
- `internal/spec/role.go` (~6.2K, substantive, undertested); `internal/spec`
  ~50% with 4 tests.
- `internal/runner`, `internal/pack`, `internal/schema`, `internal/obs`,
  `internal/context` — no individual floor.
- `docs/TESTING.md` — documents 85/90/95 end-state targets and the ratchet.

## Requirements

### Requirement 1 — Floor the unguarded packages
**User story:** As a maintainer, I want every substantive package floored, so a
regression cannot hide under the overall number.

**Acceptance criteria:**
1. `coverage-check.sh` SHALL add floors for `internal/spec`, `internal/runner`,
   `internal/pack`, `internal/schema` (modest, at/below current measured).
2. Floors SHALL be set so they pass today (ratchet, not a cliff).

### Requirement 2 — Raise internal/spec coverage
**User story:** As a maintainer, I want `role.go`/phase/status logic tested, so
fail-loud state logic is covered.

**Acceptance criteria:**
1. `internal/spec` coverage SHALL be raised (target a meaningful step above ~50%,
   covering `role.go`).
2. Its floor SHALL be ratcheted to the new measured level.

### Requirement 3 — Stay on the documented ratchet
**User story:** As a maintainer, I want the new floors consistent with the
85/90/95 path in `TESTING.md`.

**Acceptance criteria:**
1. New floors SHALL be reflected/described in `TESTING.md` as ratchet steps.
2. The change SHALL not lower any existing floor.

## Design

- Add new package entries to `coverage-check.sh` set just below current measured
  coverage for each.
- Write tests for `internal/spec` (role/phase/status), then ratchet its floor up.
- Note the additions in `TESTING.md` as ratchet steps toward 85/90/95.

## Out of scope

- Hitting the final 85/90/95 targets in this spec (rolling ratchet).
- Floors for trivial/generated packages.

## Risks

- **Cliff floors:** set each new floor at/just-below measured so CI stays green;
  ratchet up in follow-ups.
