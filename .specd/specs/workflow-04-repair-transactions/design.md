# Design — workflow-04-repair-transactions

- references: R1, R2, R3, R4, R5, R6
- boundaries: Pure impact planning, latest-event compensation, task attempts, artifact/spec revisions, and stale-descendant resolution for unreleased work.
- interfaces: `ImpactPlan`, `UndoPlan`, `ReopenPlan`, `specd undo`, `specd reopen`, content-addressed revision snapshots, and descendant-resolution operations.
- invariants: No history deletion, preview equals commit impact, fresh attempt evidence, immutable-consumption guard, CAS, and explicit stale resolution.
- failure: Revision races, snapshot failure, live lease, or external consumption refuses before partial repair and names re-preview/release/successor recovery.
- integration: Builds on workflow events, transition plans, task readiness, approval requests, evidence, program links, delivery ledgers, and spec locking.
- alternatives: Arbitrary historical undo and automatic descendant reset are rejected; Git-only snapshots are rejected because approved uncommitted authoring bytes need preservation.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- Pure impact graph and repair planners in `internal/core`.
- `undo` and `reopen` command handlers registered through command metadata.
- Attempt-aware evidence/scope/frontier projection.
- Content-addressed artifact snapshots under each spec.
- History, status, reports, docs, and black-box maintenance journeys.

Excluded: modifying released/deployed/archived revisions, deleting records, general event rollback, and
delegated repair authority in the initial slice.

## Impact interface

Caller assembles `ImpactInput` from current workflow events, artifacts, approvals, tasks, criteria,
evidence, reviews, missions/leases, submissions, releases, deployments, archives, and program links.
`BuildImpactPlan` is pure and returns:

- requested entity/version and expected state revision;
- deterministic impact digest;
- ordered entities classified current, stale, reopened, retained, superseded, cancelled, or forbidden;
- immutable-consumption references;
- required actor/authority, gates, snapshots, lease actions, and resolution choices.

Preview renders this plan. Commit accepts the expected state revision and impact digest, reloads inputs,
and refuses if recomputed plan differs.

## Undo

Initial `specd undo <slug> --expect-revision <n>` targets only the latest workflow event. Eligibility
requires a reversible transition and no child event, passing completion, submission/release/deployment,
archive, external adapter effect, or delegation consumption that depends on it.

Success appends a compensation event referencing the original and projects prior effective values at a
higher revision. Original event remains visible and immutable. There is no arbitrary event id flag.

## Task reopen

`specd reopen <slug> task <id> --reason <text> --expect-revision <n>` creates next attempt with:

- task id, attempt number, plan and scope revision;
- fresh current Git baseline and authority digest;
- pending activity plus derived readiness;
- optional bounded repair scope amendment approved in the same plan;
- links to prior attempt and impacted descendants.

Evidence identity adds attempt and plan/scope revision. Completion rejects prior-attempt evidence even
when command, files, and HEAD match. Live leases must be released/revoked inside the plan or block it.

## Artifact and spec reopen

Before artifact mutation, bytes are written with `AtomicWrite` to:

`.specd/specs/<slug>/revisions/<artifact>/<sha256>.md`

Existing matching snapshot is idempotent; differing bytes at the same digest are impossible and fail
closed. Snapshot creation and validation precede event append. New artifact version begins draft and
invalidates affected approval requests.

Spec reopen creates a new cycle referencing prior cycle. Released, deployed, or archived cycles are
successor-only. Submission is treated as immutable when its adapter contract says external consumption;
otherwise impact preview names the revocation/withdrawal requirement.

## Descendant resolution

Affected completed descendants project `completed + stale`; activity marker is not rewritten to pending.
Allowed resolution events:

- revalidate against current attempt/revision with fresh evidence;
- reopen into a new attempt;
- retain after explicit impact approval plus fresh evidence where behavior may change;
- supersede/cancel with acceptance and criterion coverage reassigned.

Parent readiness remains blocked while any stale descendant lacks resolution. Digest equality may reduce
review scope but never proves behavioral retention alone.

## Failure and recovery

- Snapshot failure: no workflow event or state mutation.
- CAS/impact drift: refuse and require preview.
- Active lease: exact release/revoke handoff.
- Cross-spec impact unavailable: conservative stale/blocked result.
- Immutable consumption: exact consuming record and successor command.
- Crash after event append: workflow-event replay finishes projection; snapshots already exist.

## Security and authority

Reopen and undo are operator-only by default. Agent may request them but cannot widen scope, choose
retention, or discard old authority. Paths are normalized within the spec revision directory. Reasons
are required and audit records store digests, not artifact contents.

## Verification

- Golden impact plans for every entity/state and cross-spec edge.
- Preview/commit digest equality and CAS race tests.
- Latest-event reversible/consumed matrix and no-deletion assertions.
- Attempt-2 cannot use attempt-1 evidence; scope amendment and fresh baseline tests.
- Snapshot crash/idempotency, active lease, released/archive successor, and stale resolution matrices.
- Black-box post-completion repair journey ending with current evidence and preserved prior history.

## Deployment and rollback

Land impact preview read-only, then task reopen, latest-event undo, artifact/spec reopen, and descendant
resolution. Feature commands stay hidden until their full route exists. New events/snapshots are
append-only; rollback may stop creating them but never deletes them.
