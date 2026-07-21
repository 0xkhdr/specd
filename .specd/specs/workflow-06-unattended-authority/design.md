# Design — workflow-06-unattended-authority

- references: R1, R2, R3, R4, R5, R6
- boundaries: Trusted actor context, operator-only dispatch enforcement, scoped delegation storage/consumption, and controller approval handoff.
- interfaces: `ActorContext`, `DelegationGrantV1`, authority ledger/lock, delegated approval transaction, and waiting-approval controller state.
- invariants: Unknown is never human, grants never widen themselves, gates are identical, failed gates consume no use, secrets never persist or render, and one use has one winner.
- failure: Missing assurance, stale/revoked/exhausted grant, policy drift, transaction interruption, or controller wait preserves progress and names operator recovery.
- integration: Extends command metadata/dispatch, host/MCP context, workflow approval requests/events, controller sessions, reports, and stdlib cryptography.
- alternatives: Username/TTY/environment attestation, repository bearer storage, generic policy language, and controller-minted grants are rejected.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- Core actor context and command authorization before handler execution.
- Project authority ledger/projection under `.specd/authority/` with a dedicated lock.
- Human-only delegation create/revoke/inspect operations and delegated approval path.
- Controller waiting/consume/handoff behavior, reports, threat model, and end-to-end tests.

Excluded: evidence creation authority, security exceptions, release/deploy/archive delegation, remote
identity provider implementation, and general policy language.

## Actor context

`ActorContext` contains class (`operator`, `agent`, `service`, `unknown`), stable subject, transport,
attestation source, assurance, and expiry. Only a configured host transport can mark it governed.
OS username, TTY, `SPECD_ACTOR`, repository files, task prose, and ordinary environment variables remain
display provenance and cannot raise assurance.

Dispatcher resolves operation metadata before handler invocation. Human-only operation plus governed
non-operator actor refuses. Unknown stays advisory during compatibility window and is clearly reported;
production policy may require governed context and fail closed.

## Grant contract and storage

`DelegationGrantV1` contains schema/id, project identity, bounded spec ids/pattern, exact transitions,
maximum uses, issuer actor/authority, issued/expiry/review times, config/policy digests, production flag,
required reason policy, explicit prohibitions, token digest, status, and use projection.

Metadata and random bearer-token digest live in `.specd/authority/grants.jsonl` plus compact projection.
Bearer token is shown once to the operator and retained only by host secret storage. Token comparison uses
SHA-256 over high-entropy bytes plus `crypto/subtle` constant-time comparison. Rendering always redacts it.

Project-wide grants require `WithAuthorityLock`, implemented with the existing lock pattern. Any operation
needing both locks acquires authority lock before spec lock; tests enforce order to prevent deadlock.

## Delegated approval transaction

1. Build readiness plan without mutating and reject failed gates before grant reservation.
2. Acquire authority lock then spec lock; reload grant, policy, request, state, and plan inputs.
3. Re-run or validate identical plan at expected revision.
4. Append a prepared grant-use event keyed by approval request/event id.
5. Append and apply the workflow approval event.
6. Append consumed grant-use event and update projection.

Concurrent preparation sees the reservation and cannot exceed maximum uses. Recovery examines prepared
records: matching committed approval finalizes consumption; absent approval releases the reservation.
Thus gate failure consumes nothing, one successful approval consumes once, and crash recovery is idempotent.

Grant revocation appends a new event and blocks future preparation. It never changes prior approval.

## Controller integration

Approval decision returns either:

- `waiting_approval` with request id, plan digest, required operator, and resume command;
- delegated operation with supplied grant reference and reason;
- typed refusal for invalid grant/policy.

Brain never creates or widens grants. Without valid grant it checkpoints progress, enters explicit wait,
and returns a non-complete outcome. A later human approval or valid grant resumes the same session after
digest/revision refresh.

## Safety and audit

Default delegation policy is off. Initial grants cannot authorize evidence, exceptions, release,
deployment, archive, or repair scope widening. Production transitions require explicit grant permission
and governed host assurance. Token, prompts, source bytes, and secrets never enter events or errors.

Approval reports distinguish interactive, delegated, unknown, revoked, and superseded provenance and
show grant id/use, issuer, scope, reason, and plan/result digests.

## Failure and recovery

- Expired/revoked/exhausted/wrong-scope/stale-policy: no reservation or state mutation; operator route.
- Gate drift after initial check: recheck fails, reservation absent.
- Crash after preparation: deterministic finalize or release on recovery.
- Grant revoked while controller waits: request remains; next transition needs new authority.
- Unknown host assurance: advisory record or production refusal, never fabricated operator identity.

## Verification

- Actor/operation matrix across CLI, MCP, and controller.
- Grant schema, scope, expiry, revocation, use, replay, redaction, and constant-time comparison tests.
- Concurrent final-use race and crash injection at every transaction boundary.
- Interactive/delegated gate-plan equality and failed-gate no-consumption proof.
- Controller wait, human resume, delegated resume, mid-run expiry, and no-self-grant assertions.
- Audit/history golden tests and threat-model docs lint.

## Deployment and rollback

Land actor context in report-only mode, then enforcement, then grant storage, then controller use. Feature
stays off unless operator config enables scoped delegation. Rollback disables new grant use but preserves
ledger/history; outstanding grants can be revoked by supported compatibility binary.
