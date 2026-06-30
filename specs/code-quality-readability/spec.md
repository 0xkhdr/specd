# Spec — Code Quality & Readability (S3)

## Introduction

Live evidence (see `../discrepancies.md` D9–D11) refines this from a generic
"improve code quality" ask into three specific, evidence-backed items:
`.golangci.yml` is already on a current v2 schema with `gosec` enabled (the
plan's premise that it's stale was wrong), but it has no complexity linter and
`revive` is absent; five specific functions show high branch density and are
named refactor targets; and six of twelve `internal/` packages — including the
two largest, `internal/core` and `internal/cmd` — have no package-level doc
comment.

## Requirement 1 — Complexity linting

**User story:** As a maintainer, I want a complexity linter enforced in CI so
that new high-branch-density functions are caught at review time instead of
discovered by a future audit.

**Acceptance criteria:**
1. THE SYSTEM SHALL enable a cyclomatic or cognitive complexity linter
   (`gocyclo` or `gocognit`) in `.golangci.yml` with a documented threshold.
2. WHEN the linter is first enabled AND existing functions exceed the
   threshold THE SYSTEM SHALL either fix those functions (Requirement 2) or
   add an explicit, commented exclusion in `.golangci.yml` naming the
   function and a tracking rationale — THE SYSTEM SHALL NOT set the
   threshold high enough to silently pass all existing code without review.
3. THE SYSTEM SHALL evaluate enabling `revive` and record the decision
   (enable with a specific rule set, or explicitly skip with rationale) — do
   not enable it with default settings without checking it doesn't conflict
   with existing `staticcheck`/`gocritic` findings.

## Requirement 2 — Refactor named complexity hotspots

**User story:** As a contributor reading `internal/cmd/pinky.go` for the
first time, I want `RunPinky` broken into named sub-steps so I can understand
Pinky's dispatch logic without holding 171 lines and 49 branches in my head
at once.

**Acceptance criteria:**
1. WHEN `internal/cmd/pinky.go:14` `RunPinky` is refactored THE SYSTEM SHALL
   preserve its exact external behavior (CLI flags, exit codes, stdout/stderr
   contract) — verified by the existing `cmd/lifecycle_test.go` and
   `json_contract_test.go` passing unmodified.
2. THE SAME preservation bar (Acceptance Criterion 1, generalized) SHALL
   apply to refactors of `internal/core/orchestration_driver.go:98`
   `DriveOrchestration`, `internal/cmd/init.go:136` `runInitWithRuntime`,
   `internal/cmd/doctor.go:67` `runDoctor`, and `internal/core/acp.go:283`
   `validateACPPayload`.
3. THE SYSTEM SHALL extract named helper functions (not just reformat) such
   that the new complexity-linter threshold (Requirement 1) passes for all
   five functions without an exclusion.
4. IF a function's complexity is inherent to a genuinely irreducible decision
   table (e.g., `validateACPPayload`'s large switch over a closed enum) THEN
   an explicit linter exclusion with rationale IS acceptable in place of a
   forced refactor — do not contort a switch-over-enum into artificial
   indirection just to satisfy a line-count metric.

## Requirement 3 — Package-level documentation

**User story:** As a new contributor running `go doc ./internal/core`, I want
a package overview instead of an empty summary, so I can orient myself before
reading individual function signatures.

**Acceptance criteria:**
1. THE SYSTEM SHALL add a `// Package x ...` doc comment to each of:
   `internal/core`, `internal/cmd`, `internal/cli`, `internal/runner`,
   `internal/pack`, `internal/schema`.
2. EACH package doc comment SHALL describe the package's responsibility in
   the layered architecture (consistent with `docs/contributor-guide.md`'s
   description) in 2-5 sentences — not a one-line restatement of the package
   name.
3. THE SYSTEM SHALL add doc comments to the specific exported symbols found
   missing them during this review: `internal/core/state.go:250`
   `LoadState`, `internal/core/orchestration.go:91` `OrchestrationPolicy`,
   `internal/cmd/next.go:26` `RunNext` — and any other exported symbol in the
   six packages from Acceptance Criterion 1 found undocumented during
   implementation (the three above are a confirmed sample, not an exhaustive
   list).

## Design

### Overview
Three independent workstreams: linter config (low risk, fast), package docs
(zero behavior risk), and function refactors (highest risk — behavior must be
preserved exactly, verified by existing tests before any new ones are
needed).

### Architecture
No architectural change. Refactors extract helper functions within the same
file/package; no package boundaries move.

### Components and interfaces
- `.golangci.yml` — add complexity linter section, evaluate `revive`.
- `internal/cmd/pinky.go`, `internal/core/orchestration_driver.go`,
  `internal/cmd/init.go`, `internal/cmd/doctor.go`, `internal/core/acp.go` —
  refactor targets.
- New `doc.go` or top-of-file package comments in the six packages from
  Requirement 3.

### Data models
No changes.

### Error handling
No changes — refactors must not alter error wrapping/messages observable by
existing tests (`cmd/json_contract_test.go` likely asserts exact error JSON
shapes; preserve them).

### Verification strategy
- Linter: `make lint` after `.golangci.yml` changes, with the new threshold.
- Refactors: existing test suite (`cmd/lifecycle_test.go`,
  `cmd/json_contract_test.go`, `core/dag_test.go` where `DriveOrchestration`
  is exercised) must pass unmodified before any new tests are added — this
  is the behavior-preservation gate.
- Docs: `go doc ./internal/...` for each of the six packages shows non-empty
  package summaries.

### Risks and open questions
- Risk: refactoring `RunPinky`/`runInitWithRuntime` touches Brain/Pinky
  dispatch and init flow respectively — both are high-blast-radius paths.
  Mitigation: refactor in small, separately-reviewable commits per function,
  run full `make test -race` after each, not just at the end.
- Open question: does extracting helpers from `validateACPPayload`'s switch
  change anything observable in ACP wire behavior? Builder must diff
  generated/serialized output before/after, not just rely on unit test pass.
