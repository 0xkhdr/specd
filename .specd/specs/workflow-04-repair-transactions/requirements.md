# Requirements — workflow-04-repair-transactions

Release D makes unreleased repair legal without weakening immutable history. Source scope:
[implementation tasks T17–T21](../../../specd-workflow-improvements/implementation-tasks.md),
[undo and reopen](../../../specd-workflow-improvements/undo-and-reopen.md),
[task generation and execution](../../../specd-workflow-improvements/task-generation-and-execution.md),
[debugging and failure recovery](../../../specd-workflow-improvements/debugging-and-failure-recovery.md), and
[migration and backward compatibility](../../../specd-workflow-improvements/migration-and-backward-compatibility.md).

## R1 — Deterministic impact preview

owner: project maintainers
priority: must
risk: critical

- R1.1: When undo or reopen is requested, the system shall preview every affected artifact, approval, task, criterion, review, mission, submission, release, deployment, archive, and cross-spec link in deterministic order before mutation.
- R1.2: When preview classifies impact, the system shall distinguish current, stale, reopened, retained, superseded, cancelled, and forbidden entities with reasons.
- R1.3: When immutable external consumption prevents in-place repair, the system shall identify the consuming record and provide a linked-successor route.
- R1.4: When the expected state revision changes after preview, the system shall refuse commit and require a fresh preview.

## R2 — Narrow undo compensation

owner: project maintainers
priority: must
risk: critical

- R2.1: When the latest event is reversible and no child, evidence completion, external effect, release, deployment, archive, or delegation use consumed it, the system shall append a compensation event and project the prior effective state at a higher revision.
- R2.2: When an undo target is not the latest reversible event or has been consumed, the system shall refuse without deleting or rewriting history.
- R2.3: When undo persistence fails, the system shall leave event history and projection mutually recoverable and shall not partially restore prior state.

## R3 — Task reopen and attempt binding

owner: project maintainers
priority: must
risk: critical

- R3.1: When an eligible completed, failed, or cancelled task is reopened, the system shall create a new attempt with a fresh baseline, plan and scope revision, authority digest, and pending readiness projection.
- R3.2: When a reopened task verifies or completes, the system shall accept only evidence bound to its current attempt and resolvable current subject revision.
- R3.3: When repair spans original task boundaries, the system shall require an explicit bounded scope amendment approved in the reopen transaction.
- R3.4: When a live mission or lease owns the task, the system shall atomically release or revoke it as authorized or refuse reopen with the exact recovery.

## R4 — Artifact and spec reopen

owner: project maintainers
priority: must
risk: critical

- R4.1: When eligible unreleased requirements, design, or tasks are reopened, the system shall preserve a content-addressed prior revision and create a new draft version with new identity and digest.
- R4.2: When an eligible unreleased spec is reopened, the system shall create a new lifecycle cycle while retaining the complete reportable prior cycle.
- R4.3: When released, deployed, or archived work is targeted, the system shall refuse in-place reopen and propose a linked successor.
- R4.4: When artifact snapshot creation fails, the system shall perform no state mutation.

## R5 — Descendant staleness and resolution

owner: project maintainers
priority: must
risk: critical

- R5.1: When reopen affects completed descendants, the system shall mark them stale without silently resetting them to pending or current.
- R5.2: When stale descendants are resolved, the system shall require fresh revalidation, explicit reopen, approved retain with adequate evidence, or supersession or cancellation with acceptance coverage reassigned.
- R5.3: When affected behavior cannot be proved unchanged, the system shall reject digest-only retention and require fresh evidence.
- R5.4: When all impact is resolved, the system shall prove parent readiness from current revisions and attempts only.

## R6 — Audit and concurrency safety

owner: project maintainers
priority: must
risk: high

- R6.1: When any repair transaction commits, the system shall record reason, actor and authority, preview digest, affected identities, prior and new revisions, and consumption guards.
- R6.2: When concurrent repair or lifecycle mutation races, the system shall allow at most one CAS winner and preserve a legal re-preview route for the loser.
- R6.3: While repair features are implemented, the system shall preserve immutable evidence, atomic writes, event replay equivalence, and zero runtime dependencies.

## Edge and failure behavior

- Cross-spec descendants, malformed legacy cycles, rejected artifacts, deleted outputs, and stale reviews appear in impact preview.
- Failed verification retries do not require reopen unless the attempt was closed.
- Archived specs never restore in place.

## Non-goals

- Deleting events, approvals, evidence, or historical artifact revisions.
- Arbitrary historical-event undo.
- Reopening externally immutable releases, deployments, or archives in place.
