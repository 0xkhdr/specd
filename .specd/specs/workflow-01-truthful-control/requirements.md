# Requirements — workflow-01-truthful-control

Release A restores truthful observation before changing persisted workflow semantics. Source scope:
[implementation tasks T01–T06](../../../specd-workflow-improvements/implementation-tasks.md),
[testing and observability](../../../specd-workflow-improvements/testing-and-observability.md),
[workflow state management](../../../specd-workflow-improvements/workflow-state-management.md),
[debugging and failure recovery](../../../specd-workflow-improvements/debugging-and-failure-recovery.md), and
[user experience and steering](../../../specd-workflow-improvements/user-experience-and-steering.md).

## R1 — Feedback regression inventory

owner: project maintainers
priority: must
risk: medium

- R1.1: When maintained workflow feedback is inspected, the system shall associate every feedback heading with a current regression test, an explicit deferral, or a resolved or superseded disposition.
- R1.2: When a feedback disposition lacks an owner or executable regression reference, the system shall fail the inventory lint with the affected feedback identity.
- R1.3: When historical feedback describes behavior already fixed, the system shall retain an invariant regression and label the historical disposition without reproducing obsolete behavior.

## R2 — Executable workflow journeys

owner: project maintainers
priority: must
risk: high

- R2.1: When the default-profile black-box journey runs from fresh initialization, the system shall complete authoring, execution, evidence, review when armed, and final lifecycle checks without manual state or ledger edits.
- R2.2: When the production-profile journey runs, the system shall preserve the production profile for the entire journey and either complete under declared host capabilities or refuse before work with an exact supported recovery.
- R2.3: When workflow journeys run twice, the system shall produce stable state and ordered output independent of map iteration order.

## R3 — Deterministic transition plan

owner: project maintainers
priority: must
risk: critical

- R3.1: When identical transition inputs are planned, the system shall produce a byte-stable plan containing target, actor, authority, armed gates, inputs, blockers, recoveries, state revision, and relevant digests.
- R3.2: When a transition plan is built, the system shall perform no filesystem or workflow-state mutation.
- R3.3: When current state has no legal successor, the system shall report that terminal condition instead of advertising a nominal transition.

## R4 — Readiness and approval parity

owner: project maintainers
priority: must
risk: critical

- R4.1: When readiness check and approval inspect the same state revision and inputs, the system shall return the same blocker set from the same transition plan.
- R4.2: When readiness passes, the system shall report the checked transition, state revision, armed gates, config digest, artifact digests, and that readiness was checked.
- R4.3: When machine output changes from the legacy check array, the system shall expose a versioned envelope and retain an explicit compatibility route for the published window.

## R5 — Guidance and dispatch parity

owner: project maintainers
priority: must
risk: high

- R5.1: When status, handshake, drive, or MCP advertises an operation, the system shall prove that the current transport can satisfy that operation's dispatch, issuer, actor, and authority preconditions.
- R5.2: When an operation requires unavailable human or host authority, the system shall render a separate handoff with the missing authority and shall not present the operation as agent-executable.
- R5.3: When command metadata, help, guidance, or dispatch routes drift, the system shall fail a parity test before release.

## R6 — Typed refusals and recoveries

owner: project maintainers
priority: must
risk: high

- R6.1: When a governed operation refuses, the system shall identify the code, category, entity, observed and expected values, inspected input digests, mutation status, retryability, required actor, and legal recovery operations.
- R6.2: When no in-place recovery exists, the system shall say so and provide the supported successor or escalation route instead of an unreachable command.
- R6.3: When evidence is present but failing or stale, the system shall distinguish that state from missing evidence.

## R7 — Preserved invariants

owner: project maintainers
priority: must
risk: critical

- R7.1: While this release is implemented, the system shall preserve deterministic gates, current-HEAD evidence integrity, atomic writes, CAS state mutation, reentrant spec locking, byte-stable task parsing, and zero runtime dependencies.
- R7.2: When CLI verbs, flags, machine envelopes, or refusal contracts change, the system shall update generated command documentation and boundary regression tests in the same task.

## Edge and failure behavior

- A warning-only readiness plan remains distinguishable from a blocking plan.
- A partially written controller checkpoint reports its durable effect and exact recovery.
- No telemetry producer, missing host containment, stale leases, and malformed artifacts receive typed outcomes.

## Non-goals

- Persisting workflow schema v2 or append-only workflow events.
- Moving project configuration or changing request activation policy.
- Implementing undo, reopen, delegation, or unattended approval.
