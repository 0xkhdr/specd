# Approvals and unattended execution

## Domain definition

Owns approval requests, actor authority, interactive decisions, delegated unattended approval,
revocation, and controller handoffs.

## Current behavior

`approve` runs readiness gates and advances one step. Command metadata says human-only and hides it
from some agent palettes, but direct CLI dispatch does not prove actor class. Records store free-form
actor. No unattended configuration exists; an agent can already call approve invisibly by convention.

## Evidence from feedback

- [Handshake omitted approve but next_commands included it](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--approve-is-absent-from-the-handshake-tool-palette-but-present-in-next_commands).
- [Nothing enforced human-only W10](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--nothing-in-the-harness-enforces-w10-an-agent-can-approve).
- [Dedicated unattended analysis recommends declared delegation](../unattended-approval-analysis.md#5-recommended-design).
- [Agent authored and approved insufficient artifacts](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-design-gate-counts-requirement-mentions-not-requirement-coverage-auto-approval-finding).

## Main problems

Human-only is advisory metadata. Current records cannot distinguish delegated approval reliably.
Automation either breaks stated policy or needs repeated manual intervention. Controller approval
would collapse author/check/approve separation if it can self-authorize.

## Root-cause analysis

CLI cannot distinguish human from agent sharing process credentials. Operation metadata was used for
tool presentation, not runtime actor enforcement. Delegation was treated as “skip approval” instead
of transferring bounded authority.

## Desired behavior

Interactive approval is an operator action on an explicit request and gate plan. Unattended approval
consumes pre-issued scoped delegation. Both run identical gates and leave distinct audit records.

## Recommended design

Approval request states: draft, requested, approved, rejected, withdrawn, expired; approved can be
revoked or superseded. Request pins artifact, state revision, gate plan digest, and config digest.

Host supplies actor class and hides operator tools. Unknown actor stays advisory. Do not use username,
TTY, or `SPECD_ACTOR_CLASS` as security proof; they may annotate CLI-only records.

Delegation grant binds:

- project and allowed spec ids/patterns;
- exact allowed transitions and maximum uses;
- issuer, issued/expiry times, policy/config digest;
- whether production lifecycle is allowed;
- mandatory reason and optional review-after time;
- prohibited operations (evidence, security exception, release/deploy/archive by default).

Repository stores grant id/hash and audit events; bearer secret stays in host secret storage. Approve
validates and consumes one use only after gates pass, preventing lost use on a gate failure. Grant
revocation blocks future use, never alters prior approvals.

Brain may emit waiting approval and consume a supplied grant; it never creates one. Without grant it
hands off to human and exits/waits distinctly, not success-halt.

## Workflow implications

Interactive flow gains submit/review clarity. Unattended flow can progress safely within operator
scope, and post-run review can identify all delegated boundaries.

## Data-model implications

Approval request/version, actor class/source, delegation id/use, gate plan/result digest, reason,
revocation/supersession, and assurance level. Config contains only delegation policy `off|scoped`.

## CLI implications

Add approval request/reject/revoke inspection and delegation create/revoke/inspect commands. Preserve
`approve <slug>` convenience for interactive operator. JSON separates `legal_operations` from
`awaiting_operator` and `delegated_operations`.

## Coding-agent implications

Agent never invokes hidden operator approve. With delegated approval exposed as legal, it supplies
reason and grant reference through host. It cannot weaken config to expand grant.

## Compatibility implications

Old approvals become actor_class unknown, delegated false/unknown. Existing `SPECD_ACTOR` remains
display provenance, not authority. Feature defaults off.

## Failure scenarios

Expired/revoked/exhausted/stale-policy grant refuses with operator recovery; gate failure consumes no
use; concurrent use CAS allows one; missing host enforcement lowers assurance; revocation during run
makes next transition wait.

## Edge cases

Later human approval may supersede delegated approval without deleting it. Delegation across multiple
specs must enumerate bounded pattern. Production release remains separate even if lifecycle approval
is delegated.

## Testing strategy

Actor-operation matrix, grant scope/expiry/use/replay, concurrency, redaction, gate equivalence,
controller wait/consume, host advisory behavior, and audit projection.

## Implementation recommendations

Enforce actor metadata before grants. Use stdlib crypto and constant-time comparison. Keep grants
small; no general policy language.

## Trade-offs

CLI-only use cannot prove human origin. Honest advisory records plus host-backed enforcement are
better than false security. Scoped grants add setup but remove repeated approval turns.

## Risks

Users may interpret delegation as approval-quality equivalence. Reports must highlight delegated
transitions and fit-review risk. Token leakage is authority leakage.

## Acceptance criteria

- Governed agent direct approve fails.
- Valid grant approval runs same gate plan.
- Gate failure neither advances nor consumes grant.
- Delegated records are unmistakable.
- Brain never mints delegation.
- No grant bypasses evidence or release controls.

## Open questions

- CLI host mechanism for bearer delivery and actor attestation.
- Whether production lifecycle delegation is permitted at all in initial release.

