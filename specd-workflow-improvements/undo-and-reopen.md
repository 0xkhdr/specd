# Undo and reopen

## Domain definition

Defines safe compensation of mistakes and creation of new work revisions after rejection,
cancellation, failure, approval, task completion, or spec completion.

## Current behavior

Lifecycle only moves forward. Current operating-model contract calls a linked successor the only
sanctioned reopen. Task override clears only escalation. Mid-requirement records stale downstream
approvals but do not move lifecycle or task state. Completed-task repair has no command.

## Evidence from feedback

- [Post-completion repair landed outside the harness](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--no-verb-re-opens-a-completed-task-so-a-post-completion-fix-lands-outside-the-harness-entirely).
- [Approved design error was unreachable through task scope](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-approved-design-specifies-a-signature-that-cannot-satisfy-its-own-requirement-and-task-scoping-makes-the-fix-unreachable).
- [Context budget blocked an already completed task](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-context-budget-gate-fires-on-a-completed-task-and-its-only-remedy-is-unreachable).
- [No approval undo after premature verification transition](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--executing--verifying-approves-with-tasks-still-pending-auto-approval-finding).

## Main problems

- Construction immutability is applied to unreleased repair.
- Editing approved tasks is forbidden, but repair commonly spans original task boundaries.
- Old baselines remain attached to completed work.
- There is no descendant invalidation/revalidation model.
- “Undo” could dangerously imply record deletion or evidence erasure.

## Root-cause analysis

Current model equates completion with external immutability and treats task rows as permanent
ownership. Real maintenance discovers faults after completion but before release, and cross-cutting
rules rarely fit one original task.

## Desired behavior

Undo compensates a recent unconsumed reversible event. Reopen creates a new version or attempt.
Neither rewrites history. Released, deployed, submitted-as-immutable, or archived revisions remain
successor-only.

## Recommended design

### Undo

Allow only latest event when no child consumed it and no passing completion, external effect,
release, deployment, archive, or delegation consumption depends on it. Append `compensated_by` event
and restore projected prior state at a higher revision.

### Reopen

`specd reopen <slug> [task <id>|requirements|design|tasks|spec] --to <target> --reason <text>
--expect-revision <n>` first previews impact, then creates new cycle/version/attempt.

Task reopen increments attempt, issues fresh baseline, and may atomically approve a bounded repair
scope amendment. Old evidence remains attempt 1. Artifact reopen snapshots old content/digest and
creates draft revision. Spec reopen is allowed only when external-consumption guard is clear.

Completed descendants become `completed_stale`; user chooses fresh revalidation, reopen, approved
retain, cancellation/supersession with coverage reassignment. Never auto-reset them to pending.

Approval-specific language stays precise: revoke an approval; resubmit a rejected artifact; reopen
the governed entity.

## Workflow implications

Repair becomes legal managed work. Reopen pauses active missions, invalidates affected authority,
and forces new context/evidence. User sees exact blast radius before mutation.

## Data-model implications

Add lifecycle cycle, artifact version, task attempt, supersession/compensation links, impact set,
immutable-consumption refs, reopen reason/actor, and stale-descendant resolution records.

## CLI implications

Add `reopen`, narrow `undo`, `approval revoke`, impact preview, and successor suggestion. Reopen is
operator-only by default; delegated grants may enumerate allowed reopen targets separately.

## Coding-agent implications

Agent may request reopen but cannot widen scope itself. After reopen it discards old authority,
loads new attempt context, and treats old evidence only as history.

## Compatibility implications

Existing work becomes cycle/attempt 1. Current completed specs stay successor-only until v2 upgrade
can prove no immutable consumption. Existing midreq records seed stale dependencies conservatively.

## Failure scenarios

- Reopen races another state change: CAS refusal, re-preview required.
- Artifact snapshot fails: no state mutation.
- Active lease exists: revoke/release in same transaction or refuse.
- External consumption discovered: refuse in-place and print successor command.

## Edge cases

Rejected requirement can resubmit without “undo”; cancelled task can reopen to new attempt; failed
verify needs retry rather than reopen unless attempt was closed; archived spec never restores in
place; cross-spec descendant impact must be visible.

## Testing strategy

Matrix each entity/state, immutable-consumption guards, stale descendants, attempt evidence binding,
scope amendments, crash injection, replay, and concurrent reopen CAS.

## Implementation recommendations

Implement task reopen first: narrowest high-value slice. Add spec/artifact reopen only after event
and immutable-consumption models pass. Keep undo latest-event-only initially.

## Trade-offs

In-place unreleased reopen is easier for users but weakens simple “complete means immutable” rule.
Explicit external-consumption boundary restores correct immutability where it matters.

## Risks

Overbroad retain action could bless stale code. Require explicit impact approval plus fresh evidence
when affected behavior cannot be proved unchanged.

## Acceptance criteria

- No operation deletes old records/evidence.
- New task attempt cannot complete with prior attempt evidence.
- Descendants cannot remain silently current.
- Released/archived reopen refuses with successor route.
- Impact preview matches committed impact.

## Open questions

- Is submission alone immutable, or only release/deployment/archive?
- Can file-digest equality permit automatic descendant retention, or only reduce required review?

