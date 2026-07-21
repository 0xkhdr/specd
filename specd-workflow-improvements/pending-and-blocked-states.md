# Pending and blocked states

## Domain definition

Defines precise semantics for work that has not started, is not yet eligible, is intentionally
paused, needs clarification, has failed, or cannot proceed.

## Current behavior

Tasks default to `pending`; frontier separately checks dependencies. Reports aggregate pending
without saying whether ready. `blocked` exists for tasks and specs, while escalation may describe a
task as blocked without storing the task marker. Waiting, pause, clarification, and deferment are
mostly prose.

## Evidence from feedback

- [Controller said frontier empty when a stale mission held a ready task](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--a-stale-unclaimed-mission-blocks-its-task-until-the-lease-ttl-expires).
- [Pending tasks did not block transition out of execution](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--executing--verifying-approves-with-tasks-still-pending-auto-approval-finding).
- [Telemetry halt exited as normal despite permanent block](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--brain-run-halts-at-step-0-on-telemetry-the-host-cannot-supply).

## Main problems

`pending` conflates accepted, ready, dependency-waiting, mission-held, and sometimes deferred work.
Blocked mixes objective failures with operator waits. Aggregate counts hide actionable cause.

## Root-cause analysis

Activity and eligibility are modeled in different functions but rendered as one status. New waiting
causes were added as controller messages rather than typed readiness records.

## Desired behavior

`pending` means only: accepted task, no active attempt, no terminal disposition. Readiness answers
whether it may start. Paused, blocked, failed, clarification, and deferred are separate facts.

## Recommended design

Task fields:

```text
activity: draft|pending|in_progress|paused|blocked|failed|completed|cancelled|superseded
readiness: ready|waiting_dependency|waiting_approval|waiting_clarification|waiting_schedule
reason_code, reason_refs, reason_text
attempt
```

Rules:

- Only `pending + ready` enters frontier.
- Dependency reason is derived and lists incomplete ids.
- Manual schedule/approval reason requires text, actor, and expiry/review time where applicable.
- `blocked` requires objective condition, owner, recovery, and recheck trigger.
- `paused` requires operator intent and resume authority, not a technical error.
- `failed` closes an attempt; retry creates a new attempt.
- Deferment is a governed disposition (`superseded` or deferred acceptance mapping), never pending.
- Any pending task blocks parent completion until completed or explicitly disposed with acceptance
  coverage resolved.

No generic spec `pending` state is recommended. Specs use `waiting_approval`,
`waiting_clarification`, `paused`, or `blocked`. Approval requests use `requested`; runs use `queued`.

## Workflow implications

Status and controller wait output state exact cause. Agents stop only for non-ready work and select
other ready frontier tasks when available. Parent readiness becomes auditable.

## Data-model implications

Add readiness projection and structured wait reason. Current marker remains activity view; readiness
stays machine state/projection, preserving byte-stable tasks Markdown.

## CLI implications

`status`, `next`, `drive`, and reports show activity plus readiness. Add filters such as
`status --tasks ready` only if real workflows need them; initial output is enough.

## Coding-agent implications

Agent acts only on pending-ready. It reports required actor and recovery for waits, never labels a
clarification or approval handoff as technical block.

## Compatibility implications

Existing pending maps unchanged. Readiness derives from DAG and approvals. Existing blocked markers
need a default `legacy_block` reason and doctor warning until clarified.

## Failure scenarios

Missing reason for stored non-ready state fails validation; circular wait is gate error; dependency
cancelled without disposition blocks with `dependency_terminal_unresolved`; expired pause becomes
review-required, not automatic resume.

## Edge cases

Task with no dependencies can be pending but waiting approval; completed-stale is a projection over
completed activity; multiple wait causes sort by priority and all remain visible.

## Testing strategy

Closed matrices for activity/readiness, frontier property tests, parent completion tests, stable
reason ordering, legacy mapping, and controller wait-message parity.

## Implementation recommendations

Add readiness as derived output before persisting it. Persist only non-derived wait records; avoid
duplicating dependency truth.

## Trade-offs

Two dimensions add output fields but prevent an ever-growing flat enum. Derived readiness avoids
state synchronization for dependencies.

## Risks

Clients reading only legacy status may miss readiness. Keep legacy `pending` but add `runnable`
during transition and deprecate ambiguous aggregates.

## Acceptance criteria

- Every pending task reports ready or named wait.
- Only pending-ready reaches frontier.
- Pending always blocks parent completion.
- Paused, blocked, clarification, approval, and deferred never render as pending alone.
- Non-ready manual state has reason and owner.

## Open questions

- Should schedule waits be first release or deferred until a real use case?
- Is `completed_stale` stored activity or derived freshness label? Recommendation: derived.
