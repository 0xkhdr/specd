# Implementation roadmap

## Domain definition

Sequences improvements into safe, reviewable releases with dependency gates, migration windows, and
measurable exit criteria.

## Current behavior

Many features exist as individually sound components but production paths cross unresolved contract
boundaries. Fixes have sometimes landed after downstream features, leaving green tests around an
unusable route.

## Evidence from feedback

Production orchestration encountered evidence-policy, telemetry, authority, routing, baseline,
reporting, concurrency, and mission-release failures in sequence. Repair and review problems then
appeared after tasks completed. This argues for vertical journeys before breadth.

## Main problems

Large state/routing/delegation changes can compound risk. Implementing reopen before audit events or
canonical config before resolver would create new ambiguity. Compatibility and docs can lag behavior.

## Root-cause analysis

Feature ordering followed domains rather than executable user outcomes. Boundary proofs were absent
or allowed posture changes.

## Desired behavior

Each release slice leaves one complete user outcome better, keeps old projects usable, and does not
claim enforcement unsupported by hosts.

## Recommended design

### Release A — truthful observation

Deliver T01-T06: feedback index, real journeys, transition plan, readiness parity, route parity, typed
refusals. No state schema change. Exit gate: check/approve blockers match and production route fails
early/actionably or completes.

### Release B — explicit activation and canonical config

Deliver T07-T11: config resolver/scaffold/migrate and request router/guide. Exit gate: ordinary work
does not invoke Specd; managed/consult routes are explicit; both legacy config spellings migrate.

### Release C — state foundations

Deliver T12-T16: event ledger, stage/condition, task readiness, clarification, approval requests.
Exit gate: v1 upgrade replay is lossless; old output compatibility remains.

### Release D — repair

Deliver T17-T21: impact preview, narrow undo, task/artifact/spec reopen, descendant revalidation.
Exit gate: audit-found post-completion defect is repaired inside Specd with fresh evidence; released
history remains successor-only.

### Release E — executable orchestration

Deliver T22-T26: parser/scaffold contracts, context, evidence, mission/concurrency, review. Exit gate:
production dispatch-claim-verify-report-review-complete journey passes without profile change or
manual ledger work.

### Release F — unattended authority

Deliver T27-T29: actor enforcement, scoped grants, controller approval requests. Exit gate: governed
agent direct approve fails; delegated journey completes and audit distinguishes it.

### Release G — cleanup

Deliver T30-T31 after window: compatibility report, measured removal, docs/changelog.

## Workflow implications

Users receive truthful diagnosis before migration, explicit routing before stricter enforcement,
state history before reopen, and actor enforcement before delegation.

## Data-model implications

Schema v2 begins only in Release C. Release A/B add versioned read envelopes without mutating state.
Grant schema waits until approval request/event foundations exist.

## CLI implications

New commands arrive with their owning release: config migrate (B), clarification/approval request
(C), undo/reopen (D), mission release/review restamp (E), delegation (F).

## Coding-agent implications

Generated guides update in B; agent can continue old managed loop until then. Reopen and delegated
operations are never advertised before dispatch can enforce them.

## Compatibility implications

Two-minor-release minimum for config and machine-response deprecations. State migration ships with
backup and downgrade preflight. Removal is its own release.

## Failure scenarios

If journey gate fails, stop release breadth and fix route. If migration telemetry shows legacy use,
extend window. If no host can enforce actor/path contract, keep assurance advisory and defer required
enforcement.

## Edge cases

Partially migrated repos, archived v1 specs, long-running old controller session, old MCP client,
reopen requested before state migration, delegation created under changed policy.

## Testing strategy

Each release has one black-box exit journey plus upgrade fixtures. CI forbids enabling later feature
flag until prerequisite contract tests pass.

## Implementation recommendations

Follow [implementation tasks](implementation-tasks.md) order. Keep vertical slices small; do not land
a UI command whose authority/state path is deferred.

## Trade-offs

Sequencing delays headline unattended/reopen features, but avoids repeating current “surface exists,
route impossible” failure.

## Risks

Temporary dual projections increase maintenance. Assign removal release and diagnostic from first
landing.

## Acceptance criteria

- Every release exit gate passes before next feature set.
- No command is documented before executable end-to-end route exists.
- Migration window and removal owner are published.
- Production journey remains profile-pure.
- Core invariants and zero dependencies remain green.

## Open questions

- Minor release cadence defining deprecation dates.
- Whether task reopen can ship as an earlier emergency slice after event ledger only.
