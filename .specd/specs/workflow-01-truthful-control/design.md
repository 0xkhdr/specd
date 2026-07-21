# Design — workflow-01-truthful-control

- references: R1, R2, R3, R4, R5, R6, R7
- boundaries: Pure transition planning, readiness projection, route parity, refusal contracts, regression inventory, and executable workflow journeys; no persisted lifecycle redesign.
- interfaces: `TransitionInput`, `TransitionPlan`, versioned check envelope, typed refusal envelope, command-operation metadata, and feedback-regression index.
- invariants: Same inputs yield same ordered plan; planning is read-only; approval cannot discover a same-revision blocker absent from readiness; no evidence or authority bypass.
- failure: Invalid or incomplete inputs produce typed blockers and actor-legal recovery; durable checkpoint effects remain visible.
- integration: Existing gates, lifecycle handlers, status, drive, handshake, MCP, and docs consume shared core projections without changing legacy state schema.
- alternatives: Fixing each command independently is rejected because it preserves contract drift; replacing all handlers at once is deferred because shared read paths deliver the safety gain first.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- `internal/core`: transition-plan model and builder, refusal registry, command-route validation.
- `internal/cmd`: explicit input assembly and rendering for check, approve, status, drive, handshake, and MCP execution.
- `internal/core/gates`: existing gate registry remains the only gate evaluator.
- `internal/cmd/*_test.go`, `scripts/production-smoke.sh`, and a feedback index: boundary and journey proof.
- Generated command reference, troubleshooting, host, and MCP documentation.

Excluded: state schema v2, config relocation, request-mode persistence, reopen, and delegated approval.

## Interfaces

`TransitionInput` is a caller-assembled value containing current status and revision, requested target,
actor class and assurance, transport capabilities, config and policy digests, artifact digests, gate
inputs, current authority, and terminal/external-consumption facts. It contains no filesystem handles.

`BuildTransitionPlan(TransitionInput)` returns an ordered `TransitionPlan`:

- schema version and deterministic plan digest;
- current and target state;
- actor, authority, and transport requirements;
- armed gate ids and input identities/digests;
- ordered blockers and warnings;
- legal recovery operations and required actors;
- `readiness_checked` and mutation intent.

Check JSON gains a versioned envelope containing the plan and findings. Legacy bare-array output stays
behind an explicit compatibility route during the published window. Text success prints the minimum
auditable identity: slug, target, revision, plan digest, config digest, and gate count.

`Refusal` remains the shared error type and gains stable entity, observed, expected, inspected inputs,
state-changed/checkpoint identity, retryability, required actor, and structured recovery operations.
Text rendering uses the shortest decisive message; JSON retains all fields.

## Data flow

1. Command layer loads existing state, config, artifacts, command metadata, and host context once.
2. Command layer constructs explicit `TransitionInput` and gate `CheckCtx`.
3. Core builds the plan and runs existing pure gates in registry order.
4. Check renders only. Approval compares the expected revision and consumes the same plan before the existing locked CAS mutation.
5. Status, drive, handshake, and MCP project next actions only from routes whose dispatch preconditions are satisfiable on the current transport.

No planner reads disk, clock, environment, or Git. Callers resolve those inputs first.

## Regression inventory and journeys

Maintain one table mapping every feedback heading to a named regression, explicit deferral, or resolved
and retained invariant. A small structural test rejects missing disposition, owner, or reference.

Black-box journeys build an installed-style binary and cover:

- default profile, one and two tasks, failed then passing verify;
- production profile without changing profile mid-test;
- authority unavailable, telemetry unavailable, and host containment unavailable;
- check/approve blocker equality at one revision;
- dispatch-claim-context-verify-report-review-complete where capabilities permit.

Production may refuse early, but refusal must be typed and executable. It may not silently downgrade.

## Failure and recovery

- Revision drift: `REVISION_CONFLICT`, no mutation, rerun readiness.
- Missing issuer or control route: blocker in plan, explicit human/host handoff.
- Evidence present but failing or stale: preserve its real classification.
- Checkpoint written before later failure: refusal reports checkpoint id, state change, and resume/cancel operation.
- No legal successor: terminal plan with no fake command.

Every registered recovery is checked against command metadata and dispatcher preconditions.

## Compatibility

State remains schema v1. Existing plain check behavior remains available during the declared response
window; new consumers request the versioned envelope. Existing error text may remain as detail while
stable refusal codes become normative. New success output has an explicit quiet flag if scripts need silence.

## Invariants

- Existing evidence, CAS, atomic-write, lock, task-parser, and no-LLM gate rules stay unchanged.
- Maps are sorted before hashing or rendering.
- Plans store identities and digests, never source bytes or secrets.
- Approval recomputes or validates the plan at the expected revision; a stale plan cannot authorize.

## Verification

- Pure matrix tests across status, profile, actor, assurance, transport, and terminal state.
- Equality test: readiness blockers equal immediate approval blockers at one revision.
- Route parity test: every advertised agent operation reaches handler preconditions.
- Golden JSON/text tests and refusal recovery validation.
- Default and production journeys, `-race`, repeated count, lint, docs lint, and domain regression script.

## Deployment and rollback

Land feedback inventory and journeys first, then planner, then check/approval, routes, and refusals.
Each slice keeps old state readable. Rollback removes additive projections and restores old renderers;
no persisted schema needs reversal.
