# Requirements — agent-protocol-clarity

> Phase A of docs/agent-specd-communication-analysis.md: correctness and clarity.
> No new protocol, no host contract. Removes the contradictions and unstated
> assurance boundaries that make a governed session read as stronger or weaker
> than it is. Prerequisite for agent-driver-protocol.

## R1 — No role instructs an agent to run a human-only command

owner: maintainer
priority: must
risk: high

- R1.1: When a role template instructs an agent to record a deviation, the system shall name an operation the agent authority permits.
- R1.2: When any shipped role or steering text references a command, the system shall fail a test if that command is denied to the role it is written for.

## R2 — Effects are named rather than implied by the words read-only

owner: maintainer
priority: must
risk: medium

- R2.1: When the system describes a role effect, it shall express it as an explicit set drawn from workspace-read, workspace-write, harness-evidence-write, harness-state-write, and external-write.
- R2.2: When the validator role runs a verification command, the system shall describe it as workspace-read plus harness-evidence-write instead of read-only.

## R3 — Assurance level is stated on every machine surface

owner: maintainer
priority: must
risk: high

- R3.1: When the system emits a machine-readable response, it shall carry an assurance level of advisory, gated, or sandboxed.
- R3.2: When the host advertises no sandbox support, the system shall report the session as advisory and shall not present it as fully governed.
- R3.3: When an operator reads command documentation, the system shall state the boundary between default-profile and production-profile enforcement explicitly.

## R4 — Refusals are typed and self-unblocking

owner: maintainer
priority: must
risk: high

- R4.1: When the system refuses an operation, it shall return a stable error code, the exact blocker, whether authority was consumed, whether retry is safe, the actor required to unblock, and the exact recovery command.
- R4.2: When a refusal is emitted in machine mode, the system shall use one structured shape on every refusal path so an agent never improvises a recovery.

## R5 — Machine output is self-locating

owner: maintainer
priority: should
risk: medium

- R5.1: When the system emits a machine-readable response, it shall include the active spec, phase and status, state revision, actor class, authority state, legal next operations, human-only boundary, and whether the response is advisory or authoritative.

## R6 — Prose never grants authority

owner: maintainer
priority: must
risk: critical

- R6.1: When a role file, steering file, requirement, skill, or agent summary requests a capability, the system shall ignore it as an authority source and derive authority only from machine-readable role capability contracts.
- R6.2: When a role Markdown file and its capability contract disagree, the system shall fail a conformance test instead of resolving the conflict at runtime.

## Edge and failure behavior

- When a role declares no effects, the contract shall default to the empty set and shall never default to workspace-write.
- When stored state carries an unknown assurance level, the system shall treat it as advisory and shall refuse to upgrade it implicitly.
- When a refusal is raised before authority is issued, the structured refusal shall report authority-consumed as false.
- When a legacy machine consumer parses a response, the system shall add fields additively so existing parses keep working.

## Non-goals

- Implementing drive, driver sessions, nonces, or context receipts (agent-driver-protocol).
- Host-side filesystem, shell, or network mediation (deferred to Phase C).
- Weakening any gate or adding any completion bypass.
