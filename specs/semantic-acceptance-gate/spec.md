# spec.md — Semantic Acceptance Gate (criteria ↔ test mapping)

**Status:** proposed
**Source:** specd-report.html §8 idea **B1** (impact: very high · effort: high · moat: very high) · §9 north-star item **#2**
**Date:** 2026-06-16
**Scope:** new Gate 8 in `internal/core/gates.go`; activates the existing `gates.acceptance` config stub and the `CriterionRecord`/`Acceptance` state fields.

---

## 1. Objective

Today `verify` proves "tests passed." It does **not** prove the tests cover the
acceptance criteria. Add a gate that maps each EARS acceptance criterion to ≥1
named test and fails completion if a criterion is untested. This closes the
deepest objection to AI code — "green tests, wrong behavior" — and makes "done"
mean **"the requirement is demonstrably satisfied,"** not "some test passed."
This is specd's strongest possible moat.

> **Hard invariant:** deterministic, stdlib-only, evidence-gated. The gate does
> **not** judge test *quality* with an LLM — it enforces a recorded,
> human/agent-authored mapping from each criterion to named tests, then verifies
> those named tests actually ran and passed. The judgment is the author's; the
> *enforcement* is the harness's.

## 2. Context (already half-built)

- `internal/core/state.go` already defines `CriterionRecord{ Status:"pass"|"fail",
  Evidence }` and `State.Acceptance map[string]CriterionRecord` — scaffolded,
  unused.
- `GatesCfg.Acceptance` defaults to `"off"` (`specfiles.go`) — an explicit stub
  "waiting for this" (per the report).
- EARS criteria are parsed today by `internal/core/ears.go` (`LintEars`).
- The gate pipeline is `CheckGates` in `gates.go`; verify records live in
  `VerificationRecord` (`state.go`).

## 3. Requirements (EARS)

- **R1 (H)** WHERE `gates.acceptance == "error"` (or `"warn"`), THE SYSTEM SHALL
  run an acceptance gate that requires every EARS acceptance criterion in
  `requirements.md` to map to ≥1 named test.
- **R2 (H)** THE SYSTEM SHALL source the criterion→test mapping from a declared,
  parseable location (an `acceptance:` block per task in `tasks.md`, listing
  criterion IDs and the test names that cover them) — not from inference.
- **R3 (H)** WHEN a criterion has no mapped test, the acceptance gate SHALL emit
  a violation (`error` when configured `"error"`, otherwise `warn`), naming the
  unmapped criterion.
- **R4 (H)** WHEN a task is marked complete, THE SYSTEM SHALL record each mapped
  criterion's outcome into `State.Acceptance` as a `CriterionRecord` whose
  `Evidence` references the passing `VerificationRecord` (exit 0 + named test).
- **R5 (M)** IF a mapped test name is absent from the verify output, the gate
  SHALL fail rather than silently pass (the named test must demonstrably run).
- **R6 (M)** WHERE `gates.acceptance == "off"` (default), behavior SHALL be
  unchanged — fully backward compatible.
- **R7 (M)** THE SYSTEM SHALL extend `specd check` and `specd report` to display
  per-criterion coverage status (covered / uncovered / failing).

## 4. Design / approach

1. **Parse the mapping** — extend the tasks parser (`tasksparser.go`) to read an
   optional `acceptance:` mapping (criterion ID → test names). Criterion IDs
   come from numbering EARS criteria in `ears.go`.
2. **Gate 8** — `GateAcceptance(CheckCtx)` in `gates.go`, appended to
   `CheckGates`, governed by `cfg.Gates.Acceptance` (off/warn/error), mirroring
   the existing `GateTraceability` warn/error pattern.
3. **Evidence binding** — on task completion (`internal/cmd/task.go` /
   `verify.go`), parse the verify output for the mapped test names; record
   `CriterionRecord{Status, Evidence: verifyRef}` into `State.Acceptance`.
4. **Surface** — `check`/`report` render the criterion coverage matrix.

## 5. Non-goals

- No LLM-based "does this test really test the criterion" judgment — mapping is
  author-declared; the gate enforces presence + execution, not semantics.
- No change to the existing 7 gates' behavior when acceptance is `off`.
- No new runtime dependency; no network.

## 6. Acceptance criteria

- With `gates.acceptance: error`, a spec with an unmapped criterion fails
  `specd check`; mapping it to a passing named test makes it green.
- Completing a task records `CriterionRecord`s in `state.json` referencing the
  passing verify record (exit 0, named test present in output).
- A mapped test that did not appear in verify output ⇒ gate failure.
- `gates.acceptance: off` ⇒ byte-identical to today (regression test).
- `check`/`report` show per-criterion coverage; `make ci` green; stdlib-only.
