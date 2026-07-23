# Design — workflow-12-reset-hygiene

- references: R1,R2,R3,R4,R5,R6,R7
- disposition: accepted
- owner: project maintainers

## Boundaries

- Lifecycle repair stays in the existing reopen planners and command transaction.
- Program projection stays in `internal/core/program.go`; no second state model is added.
- Refusal and path hardening stay at their existing shared sinks.
- Scope amendment extends the existing audited workflow/state transaction surface; it never edits evidence or task markers directly.
- MCP generation and Doctor reuse the bootstrap binary identity. No runtime dependency is added.

## Interfaces

- Reopen plans return task-status and marker updates with evidence invalidation, then one locked CAS write applies the plan.
- Approval-request identity includes the lifecycle cycle; reopening invalidates only current/future gate records for the new cycle while preserving prior records.
- Program status accepts loaded lifecycle states and derives completion, dependency satisfaction, and frontier from one predicate.
- `core.SpecDir(root, slug)` validates the slug before any per-spec child path is joined.
- Refusal construction rejects codes absent from the canonical recovery table in tests and conformance checks.
- A governed scope-amendment command accepts spec, task, path, reason, expected revision, session, and nonce; it appends the declared path and an audit event atomically.
- MCP config generation resolves the same executable identity used by bootstrap; Doctor compares configured command identity to the current handshake.

## Invariants

- Reopen never deletes history; it appends cycle/attempt records and invalidates current evidence.
- State writes remain locked, atomic, and CAS-guarded.
- Completed work cannot satisfy a reopened attempt.
- Program dependency and completion labels cannot disagree with direct spec status.
- Invalid slugs cannot reach a filesystem join.
- Scope expansion is explicit, narrow, audited, and cannot cover `.specd` runtime state.
- Generated hosting never silently claims a binary identity it does not execute.

## Failure

- Invalid lifecycle state, stale revision, live lease/session conflict, invalid slug, unknown refusal code, forbidden path, or binary mismatch fails closed with an actor-legal recovery.
- Partial reopen or amendment writes are prevented by planning before the single CAS mutation.
- Missing local executable falls back only to an explicitly resolved installed binary; Doctor reports mismatch instead of repairing silently.

## Integration

- Existing CLI and MCP routes share command metadata and refusal codes.
- Existing task parser and byte-stable marker rewrite are reused.
- Existing approval, evidence, session, diff-scope, and regression gates remain authoritative.
- Command-reference generation follows any verb or flag change.

## Alternatives

- Rejected: hand-editing `state.json`, task markers, or evidence.
- Rejected: a second program completion flag.
- Rejected: caller-by-caller slug validation.
- Rejected: deleting prior-cycle approvals.
- Deferred: automatic broad scope inference; amendments remain one explicit path at a time.

## Verification

- Focused table tests cover reopen task/spec cycles, approval rollover, program frontier, refusal-code conformance, slug traversal, scope amendment success/refusals, MCP pinning, and Doctor mismatch recovery.
- Full race, lint, vet, docs, and domain regression suites prove cross-surface compatibility.

## Deployment

- Ship in the static binary. Existing state remains readable; new cycle and amendment records are append-only.

## Rollback

- Revert the implementation commits. No migration deletes or rewrites historical records.
