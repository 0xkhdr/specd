# Design — workflow-03-state-foundations

- references: R1, R2, R3, R4, R5, R6, R7
- boundaries: Workflow-event ledger, schema-v2 projection, orthogonal spec/task state, clarification records, approval requests, and v1 migration; no reopen mutations.
- interfaces: `WorkflowEventV1`, `TransitionCommit`, stage/condition validator, readiness projection, clarification and approval request state machines, and state migration preview.
- invariants: Event-first recoverable commit, replay equals projection, one canonical transition validator, derived dependency readiness, immutable requests, and lossless evidence history.
- failure: Torn append, unapplied event, future schema, invalid legacy state, or CAS race fails closed and recovers without duplicate transitions.
- integration: Reuses spec lock, fsynced append, atomic state CAS, existing task markers, transition plans, history reports, and config provenance.
- alternatives: Overwriting map slots and storing every readiness cause are rejected; workflow events stay separate from telemetry `EventV1` to avoid semantic collision.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- New `internal/core/workflow_event.go`: workflow event schema, validation, canonical digest, append, read, replay, and recovery. Existing `event.go` remains adapter telemetry.
- `internal/core/state.go` and phase/frontier projections: schema-v2 fields and compatibility status.
- Clarification and approval-request core records plus small command handlers.
- State migration preview/commit, history/status/report projections, and version documentation.

Excluded: undo, reopen, scope amendments, delegation grants, release mutation, and task Markdown rewrite.

## On-disk contracts

Each spec gains `workflow-events.jsonl`, append-only and fsynced. `state.json` remains the compact current
projection and gains:

- `schema_version: 2`, `cycle`, `stage`, `condition`, `last_event_id`;
- artifact version/current request identities;
- task activity and attempt projections plus persisted manual wait records;
- compatibility `status` during the migration window.

`WorkflowEventV1` contains schema, deterministic event id, expected and resulting state revision,
entity kind/id, before/after entity version, transition kind, actor and authority provenance, reason,
config/policy/source digests, Git head when relevant, impacted identities, and timestamp. Source bytes,
secrets, prompts, and raw output are forbidden.

## Commit and recovery protocol

Every mutating command runs under the existing per-spec reentrant lock:

1. Load and validate events and state projection.
2. Replay any valid event after `last_event_id`; persist recovered projection by CAS.
3. Build a pure transition plan from explicit inputs.
4. Validate expected revision and generate deterministic next event identity.
5. Append and fsync one workflow event.
6. Apply event to projection, set `last_event_id`, and atomically save by expected revision.

Crash after step 5 leaves an unapplied durable event; next load replays it. Projection cannot legally
advance without its event. Duplicate event id or mismatched expected revision refuses. A torn final
JSONL line is recoverable only when it is the final incomplete append; valid prior events remain.

## State models

Spec stores lifecycle stage independently from condition. One table owns valid combinations and
generates tests/docs. Complete cannot combine with paused; waiting approval requires a current request;
active execution requires an approved plan.

Task stores activity:

`draft | pending | in_progress | paused | blocked | failed | completed | cancelled | superseded`

Readiness is projected as:

`ready | waiting_dependency | waiting_approval | waiting_clarification | waiting_schedule`

Dependency readiness is derived from DAG/task state. Only manual approval, clarification, schedule,
pause, and block facts persist. Reasons have stable codes, refs, owner, recovery, and review/expiry when
applicable. Only pending plus ready reaches frontier. Legacy task marker remains the activity view.

## Clarification and approval requests

Clarification records are immutable transitions over open, answered, withdrawn, and expired. They pin
affected entity/version and whether blocking. Answer stores a new resolution event; changing a question
creates a new id.

Approval requests are immutable versions over draft, requested, approved, rejected, withdrawn,
expired, revoked, and superseded. Request pins entity/version, artifact digest, state revision,
transition-plan digest, config digest, requester, and expiry. Approval never edits a request; it appends
a transition. Existing `approve` may create-and-approve a request for interactive compatibility.

## v1 migration

Dry-run maps each v1 spec to cycle 1, artifact version 1, task attempt 1, and a synthetic baseline
event. Existing approvals become approved requests with unknown actor assurance. Existing evidence and
Markdown bytes remain untouched. Ambiguous legacy blocked state refuses with an explicit operator choice.

Commit preserves `state.v1.json.bak` with original permissions, writes baseline ledger, replays to a
candidate v2 projection, compares effective status/task/evidence meaning, then atomically activates v2.
Archived v1 specs remain immutable and inspectable without in-place rewrite.

## Failure and compatibility

- Future event/state version: refuse before projection or append.
- Event appended, state absent/stale: replay and CAS.
- Projection ahead of ledger: corruption refusal; restore from backup/ledger, never invent event.
- Cancelled dependency without coverage disposition: structured unresolved dependency.
- Legacy clients read compatibility status; only v2 core mutators write canonical fields.

## Verification

- Exhaustive stage/condition, activity/readiness, clarification, and approval transition matrices.
- Replay/property tests: ledger projection equals stored state.
- Crash injection at append, fsync, CAS, backup, and activation boundaries.
- Race tests, duplicate id, future schema, torn final line, and idempotent recovery.
- Golden v1 fixtures including complete, blocked, empty tasks, approvals, evidence, and archives.
- Parent completion/frontier tests and compatibility projection conformance.

## Deployment and rollback

Land read-only v2 projection and dual-output first, then ledger, migration command, and one mutator at a
time. Keep v1 backup and downgrade preflight. Rollback before migration removes additive reads; after
migration requires explicit restore from validated backup, never silent downgrade.
