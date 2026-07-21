# Workflow state management

## Domain definition

Owns specification, artifact, task, approval, execution-run, and clarification states; validates
transitions; projects current state from immutable events.

## Current behavior

`internal/core/phases.go` defines a linear ratchet from requirements through complete plus a
standalone blocked status. Task state is a four-value map mirrored into Markdown markers.
Approval records are map entries, and orchestration has separate mission, lease, and session state
machines. No shared transition object connects them.

## Evidence from feedback

- [`executing -> verifying` accepted pending tasks](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--executing--verifying-approves-with-tasks-still-pending-auto-approval-finding).
- [`check` stayed green while work was unshippable](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--specd-check-stays-silent-and-exit-0-while-the-spec-is-unshippable).
- [Approved artifacts changed without gate reaction](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--approved-artifacts-and-steering-can-be-edited-with-no-gate-reaction-at-all).
- [Session/checkpoint divergence produced hidden durable effects](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--correction--what-actually-re-pinned-the-baseline-was-a-failed-brain-run-not-brain-resume).

## Main problems

- One status tries to represent stage and condition.
- Readiness is recomputed differently by commands.
- Approval, evidence, task, and controller transitions lack one dependency graph.
- Current map-shaped records overwrite effective slots and obscure revision chains.
- Failed commands can leave recoverable controller state with no common audit semantics.

## Root-cause analysis

State models grew per feature. Each is locally deterministic, but cross-domain meaning is implicit.
The lifecycle assumes construction only; orchestration assumes attempts; maintenance assumes
successors. Without a shared transition plan, each command chooses which gates and descendants
matter.

## Desired behavior

Use orthogonal closed sets and one transition engine:

- Spec: `stage` plus `condition`.
- Artifact: version plus review state.
- Task: activity plus readiness plus attempt.
- Approval, run, and clarification: independent closed state machines.
- Append-only events are history; `state.json` is CAS-protected current projection.

## Recommended design

`TransitionInput` contains current entity versions, actor/authority, config/source digests, git head,
and explicit requested transition. Pure planning returns mutations, stale descendants, gates,
refusals, and recoveries. Commit appends event, applies projection, and atomically advances revision.

Keep old `status` as a compatibility projection during migration. Never allow code to mutate it
directly after v2. One validator owns valid stage/condition pairs and generates docs/tests.

## Workflow implications

Every stage gains explicit authoring, waiting approval, pause, clarification, block, and cancellation
behavior. `check --readiness`, approve, route, and reports consume the same plan. Completion checks
all active obligations rather than inferring readiness from phase name.

## Data-model implications

Add `schema_version: 2`, `cycle`, `stage`, `condition`, entity version maps, `last_event_id`, and event
ledger. Events include before/after version, reason, actor, authority, config digest, source digests,
git head, and impacted entities. Existing evidence remains separate and immutable.

## CLI implications

Add transition preview JSON to check/status. State-changing verbs accept `--expect-revision`.
Status renders stage, condition, task activity/readiness, stale descendants, and current request ids.

## Coding-agent implications

Agent receives one authoritative transition plan. It may act only when mode, entity, readiness, and
authority all agree. A condition change never grants new file scope or evidence authority.

## Compatibility implications

Map v1 statuses directly to v2 stages/active condition; `complete` maps complete, and blocked needs
prior-stage inference or a repair diagnostic. Existing task markers remain byte-stable. Existing
records get baseline event references without being rewritten.

## Failure scenarios

- Event append succeeds but projection fails: replay on next load, do not append duplicate.
- Projection advances without durable event: reject load and recover from write-ahead record.
- Future event schema: fail before projection.
- Invalid legacy blocked state: doctor names manual target selection.

## Edge cases

Complete plus paused is invalid; waiting approval with no request is invalid; active execution with
no approved task plan is invalid; completed task with no passing attempt evidence remains invalid.

## Testing strategy

Exhaustive transition matrices, property tests for replay/projection equivalence, crash injection at
every write boundary, race/CAS tests, v1 fixtures, and black-box lifecycle journeys.

## Implementation recommendations

Land read-only transition planning first, then event persistence, then migrate one mutation path at
a time. Keep transition validation in core and filesystem assembly in command layer.

## Trade-offs

More explicit fields and events increase schema size and recovery code. They remove duplicated
command reasoning and make reopen/audit possible; this is justified complexity at the state boundary.

## Risks

Dual old/new projections may drift during rollout. A conformance test must compare them until old
fields are removed.

## Acceptance criteria

- Same input yields same plan and event digest.
- Invalid combinations cannot save.
- Replayed state equals stored projection.
- Approval and check share blocker set.
- History explains every effective transition.

## Open questions

- Store event ledger inside state records or dedicated `events.jsonl`?
- Which uncommitted artifact snapshot mechanism is required before Git commit?

