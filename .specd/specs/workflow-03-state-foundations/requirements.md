# Requirements — workflow-03-state-foundations

Release C supplies versioned state foundations needed by repair and unattended workflows. Source scope:
[implementation tasks T12–T16](../../../specd-workflow-improvements/implementation-tasks.md),
[workflow state management](../../../specd-workflow-improvements/workflow-state-management.md),
[pending and blocked states](../../../specd-workflow-improvements/pending-and-blocked-states.md),
[specification authoring](../../../specd-workflow-improvements/specification-authoring.md),
[approvals and unattended execution](../../../specd-workflow-improvements/approvals-and-unattended-execution.md), and
[migration and backward compatibility](../../../specd-workflow-improvements/migration-and-backward-compatibility.md).

## R1 — Append-only workflow events

owner: project maintainers
priority: must
risk: critical

- R1.1: When a governed workflow transition commits, the system shall append an event containing identity, entity version, actor and authority provenance, reason, input digests, before and after versions, and impacted entities.
- R1.2: When events replay from a valid baseline, the system shall rebuild current projection byte-for-field equivalent to saved state.
- R1.3: When event append or projection persistence is interrupted, the system shall recover idempotently without duplicate events, lost transitions, or projection advance without durable provenance.
- R1.4: When a future event or state schema is encountered, the system shall fail before mutation.

## R2 — Spec stage and condition

owner: project maintainers
priority: must
risk: critical

- R2.1: When spec state is represented, the system shall store lifecycle stage separately from active, waiting approval, waiting clarification, paused, blocked, cancelled, or complete condition.
- R2.2: When an invalid stage and condition combination is proposed or loaded, the system shall reject it through one canonical validator.
- R2.3: While legacy clients remain supported, the system shall project the existing status field deterministically from the new state without allowing direct legacy mutation.

## R3 — Task activity and readiness

owner: project maintainers
priority: must
risk: critical

- R3.1: When a task is accepted but has no active attempt or terminal disposition, the system shall report activity `pending` and a separate readiness value with structured reasons and references.
- R3.2: When frontier is computed, the system shall include only tasks whose activity is pending and readiness is ready.
- R3.3: When a task waits on dependencies, approval, clarification, or schedule, the system shall report every applicable cause in stable priority order.
- R3.4: When any task remains pending without an accepted terminal disposition, the system shall block parent completion.
- R3.5: When readiness depends only on DAG state, the system shall derive it instead of storing duplicate dependency truth.

## R4 — Clarification requests

owner: project maintainers
priority: must
risk: high

- R4.1: When an unresolved question affects authoring or execution, the system shall record a versioned clarification request linked to exact entities and whether it blocks readiness.
- R4.2: When a clarification is answered, withdrawn, or expires, the system shall append the resolution and recompute only affected readiness without editing prior records.
- R4.3: When artifact revision makes an answer stale, the system shall retain the answer as history and identify the new clarification or review needed.

## R5 — Approval request lifecycle

owner: project maintainers
priority: must
risk: critical

- R5.1: When an artifact or transition is submitted for approval, the system shall create an immutable request pinned to artifact digest, state revision, transition-plan digest, and config digest.
- R5.2: When an approval request changes state, the system shall allow only the closed draft, requested, approved, rejected, withdrawn, expired, revoked, and superseded transitions defined by the canonical model.
- R5.3: When request inputs drift, the system shall refuse stale approval and require a new or superseding request.
- R5.4: When interactive approval uses the compatibility command, the system shall preserve explicit request identity and immutable approval history.

## R6 — Lossless v1 migration

owner: project maintainers
priority: must
risk: critical

- R6.1: When a v1 project upgrades, the system shall map existing specs, tasks, approvals, and evidence to cycle or attempt 1 without changing their effective meaning.
- R6.2: When legacy blocked state cannot reveal its prior stage, the system shall refuse automatic mapping and provide a repair diagnostic.
- R6.3: When migration succeeds, the system shall preserve a permission-matched backup, validate replay, and atomically activate the new projection.
- R6.4: While compatibility remains active, the system shall prove old and new projections agree through conformance tests.

## R7 — Preserved invariants

owner: project maintainers
priority: must
risk: critical

- R7.1: While state foundations are implemented, the system shall preserve state CAS, atomic writes, reentrant locking, immutable evidence, deterministic reports, and zero runtime dependencies.
- R7.2: When event or migration records include sensitive configuration provenance, the system shall store digests and source identities without raw secret values.

## Edge and failure behavior

- Torn final events, duplicate event ids, checkpoint/projection divergence, empty task maps, and concurrent CAS conflicts recover or refuse deterministically.
- Completed plus paused, waiting approval without a request, and completed task without passing attempt evidence are invalid.
- Cancelled dependencies require an explicit coverage disposition before descendants become ready.

## Non-goals

- Undo, task reopen, artifact reopen, or stale-descendant resolution mutations.
- Scoped delegation grants or unattended approval consumption.
- Rewriting task Markdown markers into a new format.
