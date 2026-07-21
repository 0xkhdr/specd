# Requirements — workflow-06-unattended-authority

Release F enforces actor boundaries and permits bounded unattended approval without gate bypass. Source scope:
[implementation tasks T27–T29](../../../specd-workflow-improvements/implementation-tasks.md),
[approvals and unattended execution](../../../specd-workflow-improvements/approvals-and-unattended-execution.md),
[coding-agent routing](../../../specd-workflow-improvements/coding-agent-routing.md),
[context management and enforcement](../../../specd-workflow-improvements/context-management-and-enforcement.md), and
[task generation and execution](../../../specd-workflow-improvements/task-generation-and-execution.md).

## R1 — Actor-aware dispatch

owner: project maintainers
priority: must
risk: critical

- R1.1: When a trusted host supplies actor class, the system shall enforce command `human_only`, agent, validator, auditor, and delegated-operation metadata before handler mutation.
- R1.2: When a governed agent invokes an operator-only operation directly, the system shall refuse and report the required actor and legal handoff.
- R1.3: When actor origin is unknown or supplied only by OS username, TTY, or ordinary environment text, the system shall record advisory provenance and shall not claim human proof.
- R1.4: When actor context crosses CLI, MCP, or controller transport, the system shall preserve its source and assurance without allowing repository prose to widen it.

## R2 — Scoped delegation grants

owner: project maintainers
priority: must
risk: critical

- R2.1: When an authorized operator creates a delegation grant, the system shall bind it to project, bounded spec identities, exact transitions, maximum uses, issuer, issue and expiry times, config and policy digests, required reason, and explicit prohibitions.
- R2.2: When a grant bearer secret is persisted, the system shall store only a secure digest in the repository and keep the bearer value in host secret storage.
- R2.3: When grant scope, expiry, revocation, remaining uses, policy digest, or production permission does not authorize a transition, the system shall refuse before gates or mutation with operator recovery.
- R2.4: When grants are compared or consumed, the system shall use standard-library cryptography, constant-time secret comparison, replay resistance, and CAS-safe use accounting.

## R3 — Gate-equivalent delegated approval

owner: project maintainers
priority: must
risk: critical

- R3.1: When delegated approval is attempted, the system shall execute the same transition plan and readiness gates as interactive approval.
- R3.2: When readiness gates fail, the system shall neither advance state nor consume a grant use.
- R3.3: When concurrent consumers use the same final grant use, the system shall permit at most one successful transition.
- R3.4: When delegated approval succeeds, the system shall record request identity, grant identity and use, actor source, reason, plan digest, result digest, and unmistakable delegated assurance.

## R4 — Controller approval handoff

owner: project maintainers
priority: must
risk: critical

- R4.1: When the controller reaches approval without a valid grant, the system shall enter explicit waiting approval, expose the request and human handoff, and return a distinct non-complete outcome.
- R4.2: When a valid supplied grant authorizes the request, the system shall consume it only after gates pass and continue the same controller session.
- R4.3: When a grant expires or is revoked while the controller waits or runs, the system shall preserve progress and require a new operator-authorized route for the next transition.
- R4.4: When controller code runs, the system shall never create, widen, or self-authorize a delegation grant.

## R5 — Safety boundaries

owner: project maintainers
priority: must
risk: critical

- R5.1: While delegation is enabled, the system shall prohibit grants from bypassing verify evidence, security exceptions, release, deployment, archive, or any operation excluded by policy.
- R5.2: When bearer or audit data is rendered, the system shall redact secrets and expose sufficient grant identity and scope for review.
- R5.3: When host enforcement is unavailable, the system shall lower assurance or refuse governed production policy instead of pretending actor or path containment.

## R6 — Compatibility and audit

owner: project maintainers
priority: must
risk: high

- R6.1: When old approval records load, the system shall map actor class to unknown and shall not reinterpret them as human or delegated proof.
- R6.2: When delegation is unconfigured, the system shall preserve interactive behavior and keep the feature off.
- R6.3: When reports render approvals, the system shall distinguish interactive, delegated, revoked, superseded, expired, and unknown-authority records.

## Edge and failure behavior

- Exhausted, expired, revoked, replayed, stale-policy, wrong-spec, and wrong-transition grants refuse with stable codes.
- Revocation affects future uses only and never rewrites prior approvals.
- Later human approval may supersede delegated approval while both remain auditable.

## Non-goals

- Inferring human identity from username, TTY, or an untrusted environment variable.
- General-purpose policy language or repository storage of bearer secrets.
- Delegating evidence manufacture, security exceptions, release, deploy, or archive in the initial release.
