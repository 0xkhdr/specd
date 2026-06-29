# Spec — Fail-Loud State Validation (A10)

**Priority:** P2 · **Wave:** 2 · **Domain:** fail-loud state validation.

## Introduction

Two fail-loud gaps remain. (R-3) `cross-spec-recovery` resumes the program DAG;
a resume where a child spec's on-disk state was hand-edited to an impossible
status must be rejected with a clear error, not silently coerced. (CMD-2)
`pinky-brain-console disable` "warns active sessions may need cancel" — that
warning must be emitted on the actual disable path and tested, not only
specified.

## Current-state grounding

- `specs/resilience/cross-spec-recovery` — resumes program DAG from on-disk state
  (`program-state.json`, child `state.json`).
- `internal/spec/status.go`, `internal/spec/phase.go` — status/phase enums.
- `specs/commands/pinky-brain-console` — disable path; warning specified.

## Requirements

### Requirement 1 — Reject impossible hand-edited child status
**User story:** As an operator, I want resume to refuse a corrupt/impossible
child status, so I am not silently running on coerced state.

**Acceptance criteria:**
1. A resume where a child spec's on-disk status is an impossible/unknown value
   SHALL be rejected with a clear, actionable error.
2. The error SHALL name the offending spec and the invalid value.
3. The resume SHALL NOT silently coerce the value to a default.

### Requirement 2 — Disable warning emitted on the real path
**User story:** As an operator, I want the "active sessions may need cancel"
warning on the actual `pinky-brain-console disable`, so I am not surprised.

**Acceptance criteria:**
1. `pinky-brain-console disable` SHALL emit the warning when active sessions
   exist.
2. A test SHALL assert the warning is emitted on the real disable path.
3. No active sessions → no spurious warning.

## Design

- Add strict status/phase parsing on the resume path that errors on unknown
  values (reuse `internal/spec` enums; add an `IsValid`/parse-error path).
- Add a resume test feeding a hand-edited impossible child status and asserting
  rejection with spec name + value.
- Confirm the disable handler emits the warning; add a test exercising the path
  with active sessions present and absent.

## Out of scope

- Auto-repairing corrupt state (reject, don't coerce — by design).
- Redesigning the status enum.

## Risks

- **Over-strict rejection:** ensure legitimate in-progress/transitional statuses
  are not falsely rejected — cover valid statuses in the same test table.
