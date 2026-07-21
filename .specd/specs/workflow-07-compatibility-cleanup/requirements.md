# Requirements — workflow-07-compatibility-cleanup

Release G measures compatibility use, then removes expired paths only through a separate release decision. Source scope:
[implementation tasks T30–T31](../../../specd-workflow-improvements/implementation-tasks.md),
[migration and backward compatibility](../../../specd-workflow-improvements/migration-and-backward-compatibility.md),
[testing and observability](../../../specd-workflow-improvements/testing-and-observability.md),
[user experience and steering](../../../specd-workflow-improvements/user-experience-and-steering.md), and
[implementation roadmap](../../../specd-workflow-improvements/implementation-roadmap.md).

## R1 — Local compatibility inventory

owner: project maintainers
priority: must
risk: high

- R1.1: When compatibility diagnostics run, the system shall list every active use of legacy config paths, state schema, status projection, machine-output shape, unknown actor provenance, and deprecated task grammar with stable codes.
- R1.2: When a deprecated surface is migrated, the system shall stop reporting that use without deleting historical migration records.
- R1.3: When workflow metrics are reported, the system shall derive local counts from existing history and ledgers without a second mutable metrics store or outbound telemetry.
- R1.4: When reports include refusals, waits, retries, reopen cycles, delegated approvals, and zero-progress halts, the system shall expose aggregate identities without source content or secrets.

## R2 — Removal eligibility

owner: project maintainers
priority: must
risk: critical

- R2.1: When compatibility removal is proposed, the system shall require expiration of the published minimum two-minor-release window, passing upgrade fixtures, local usage audit, release-owner decision, and current generated documentation.
- R2.2: When any removal prerequisite is unmet, the system shall keep the compatibility route and report the unmet exit gate.
- R2.3: When archived or long-lived v1 work remains supported, the system shall preserve a documented inspection or upgrade route before removing ordinary legacy writes.

## R3 — Deliberate compatibility removal

owner: project maintainers
priority: must
risk: critical

- R3.1: When approved compatibility removal ships, the system shall remove expired legacy config reads, deprecated status projection writes, old machine-output routes, and stale examples as one explicit breaking release task.
- R3.2: When an old project uses a removed surface, the system shall fail before mutation with the exact supported upgrade command and preserved backup requirements.
- R3.3: When a future schema is encountered after cleanup, the system shall continue to fail closed before mutation.
- R3.4: When compatibility branches remain, the system shall document each branch and its removal condition or fail structural lint.

## R4 — Upgrade and archival safety

owner: project maintainers
priority: must
risk: critical

- R4.1: When config or state migration runs after cleanup, the system shall preserve evidence, history, permissions, effective policy, and idempotent recovery.
- R4.2: When an archived v1 spec is inspected, the system shall verify its immutable manifest without silently rewriting it.
- R4.3: When downgrade would let an old binary overwrite upgraded state, the system shall refuse through version preflight.

## R5 — Documentation and release proof

owner: project maintainers
priority: must
risk: medium

- R5.1: When CLI surfaces or compatibility routes are removed, the system shall regenerate command reference, upgrade guide, archival instructions, changelog, and examples in the same task.
- R5.2: When the cleanup release is tested, the system shall pass default and production journeys, migration and downgrade matrices, future-schema refusal, race checks, repeated-count checks, lint, and domain regressions.

## Edge and failure behavior

- Read-only filesystems, existing backups, nested legacy roots, archived v1 specs, and old JSON clients receive deterministic diagnostics.
- Repeated deprecation warnings remain bounded and machine-readable in CI.
- Removal never depends on network telemetry.

## Non-goals

- Removing compatibility before its published window and exit gates pass.
- Combining cleanup with new workflow features or schema redesign.
- Adding a daemon, outbound reporting, or a new mutable metrics database.
