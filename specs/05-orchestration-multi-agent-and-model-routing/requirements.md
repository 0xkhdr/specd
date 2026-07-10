# Requirements — Orchestration, multi-agent work, model routing

## Scope

Create deterministic worker lifecycle and routing contracts around existing Brain foundation.
Preserve atomic writes, state CAS, reentrant spec lock, append-only evidence, no-bypass verify,
human approval, byte-stable task parser, offline/stdlb-only core, and compatibility migration.

### R1 — Truthful dispatch and mission

- R1.1: Brain shall distinguish `pending`, `delivered`, `claimed`, `active`, `reported`,
  `expired`, `cancelled`, `escalated`, and terminal task state. Pending dispatch shall not claim a
  process/model executed.
- R1.2: Every mission shall use a versioned canonical envelope with protocol, session/mission/spec/
  task identity, attempt, role, authority ref, declared files, acceptance, verify, context receipt,
  policy/config/palette digests, subject identity, limits, creation/expiry, and route class/reason.
- R1.3: Unknown version/field/enum, duplicate mission, missing required pin, or noncanonical digest
  shall fail closed with stable code; same semantic input shall serialize and digest identically.

### R2 — Worker claim and lease

- R2.1: Public local CLI/MCP-equivalent claim shall accept registered stable worker id, host,
  capabilities, mission/task/role/pin echoes, requested lease; it atomically grants at most one
  live lease per task/attempt.
- R2.2: Lease shall have opaque unique id, worker id, task/mission/attempt, issued/expiry, policy
  digest and revocation reason. Controller identity cannot satisfy worker identity.
- R2.3: Role/capability/authority/task/context/config/palette/subject mismatch, expired authority,
  or conflicting claim shall refuse before work authority. Loser gets typed conflict/recovery.

### R3 — Progress, report, completion

- R3.1: Worker heartbeat and cancellation acknowledgement shall append bounded correlated events;
  only matching live lease may renew within policy. Heartbeat does not extend authority or alter pins.
- R3.2: Report shall bind mission/lease/worker/attempt, actual observable subject facts, evidence and
  eval refs, declared route/provider facts, status/blocker/next action, bounded telemetry.
- R3.3: Report acceptance shall require matching live unrevoked lease, role/task/mission/pins,
  passing current evidence and HEAD. It shall call normal completion path, never bypass it.
- R3.4: Local host shall server-compute git HEAD/diff where possible. Worker claimed files/cost stay
  audit facts; disagreement/refusal is retained, never converted to completion proof.

### R4 — Recovery, retries, parallel work

- R4.1: Checkpoint before durable mission visibility; resume reconciles checkpoint, ACP, pending
  missions, leases, reports, and task status without duplicate mission/completion.
- R4.2: Retry policy shall classify retryable failure, attempt bound, timeout, expiration, revocation,
  escalation owner and recovery action. Stale/revoked report cannot complete later.
- R4.3: Concurrent claims allowed only for frontier tasks with no overlapping write scope, unless
  approved coordination policy pins shared-file ordering. Claim races yield one winner.

### R5 — Deterministic routing and brakes

- R5.1: Versioned project policy shall map role/risk/complexity/capabilities to eligible capability
  class, token/cost/deadline/retry limits and ordered fallback. Inputs + policy yield stable route/reason.
- R5.2: High-risk task shall refuse class missing required review/eval/sandbox/context capability.
  Policy must be human-approved with task metadata; model prose cannot alter it.
- R5.3: Cost/tokens/deadline/health telemetry shall record unit/source/knownness. Required unknown,
  exceeded budget, expired deadline, unsupported class, or exhausted retry brakes dispatch/escalates.
- R5.4: Optional adapter resolves class to provider/model and records actual fact. Provider outage
  follows declared fallback/escalation; it never makes core gate contact network or change semantics.

### R6 — Interoperability and observability

- R6.1: ACP event envelope shall retain mission, claim, heartbeat, cancel, report identity/pins and
  redacted references. Unknown protocol import/export fails closed.
- R6.2: Local subprocess, MCP host, and future A2A adapter shall produce same semantic ACP stream
  for canonical fixture, excluding declared transport metadata.
- R6.3: Export contains ids/digests/status/reasons, not secrets, raw prompts, source body, hidden
  reasoning, or unbounded tool output.

## Non-goals

- No embedded provider SDK/model call/network client/secret transport in trusted core.
- No claim that Brain launches worker before real adapter delivery exists.
- No autonomous approval, route override, evidence bypass, self-reported-scope trust, or hidden
  chain-of-thought capture.
