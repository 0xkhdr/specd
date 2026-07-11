# W0 T01 — Requirement-to-surface inventory (Domain 05)

Scout, read-only. Maps requirements R1–R6 (`requirements.md`) onto the existing
Brain/ACP/session/lease/config/MCP/report code, records the gap, and names the
boundary domain that owns the neighboring contract. `file:line` refs are to the
current tree on branch `sdlc-specs`.

## Requirement → surface → gap → boundary

| Requirement | Current code surface | Gap | Boundary domain |
|---|---|---|---|
| R1.1 truthful dispatch states (pending/delivered/claimed/active/reported/expired/cancelled/escalated/terminal); pending must not imply launch | ACP kinds are only `dispatch`/`claim`/`report` (`internal/orchestration/acp.go:17-21`); session lifecycle is `running/cancelled/complete/crashed` (`internal/orchestration/session.go:22-27`); worker state is only `active`/`expired` (`internal/orchestration/decide.go:22-25`); `Dispatch` writes a lease with `WorkerID:"brain"` (`internal/cmd/brain_run.go:205-208`) | No `pending`/`delivered`/`claimed`/`reported`/`escalated`/`cancelled` mission state enum; dispatch already writes a controller lease so "pending" is indistinguishable from "launched" — the exact untruthful-claim gap | 01 (task/phase semantics) |
| R1.2 versioned canonical mission envelope (protocol, identity, attempt, role, authority ref, declared files, acceptance, verify, context receipt, config/palette/policy digests, subject, limits, expiry, route class/reason) | No mission type exists; only `MissionID` string `sessionID.sN.taskID` (`internal/orchestration/checkpoint.go:70-76`); ACP dispatch event carries just `TaskID`+`MissionID` (`internal/cmd/brain_run.go:221-227`). Design names target `MissionV1` (`design.md:26-32`) | `MissionV1` struct absent; no protocol_version, no pinned digests, no acceptance/verify/context/config/palette/policy/subject/limits/route fields | 02 (context receipt/digest), 03 (dispatch envelope) |
| R1.3 unknown version/field/enum, duplicate mission, missing pin, noncanonical digest fail closed; stable canonical serialization | Duplicate-mission guard exists: `AppendDispatch` refuses reused `MissionID` (`internal/orchestration/acp.go:103-118`, `ErrDuplicateMission` :78); ACP decode rejects malformed JSON (`acp.go:150-171`) | No protocol_version/unknown-field/unknown-enum rejection; no canonical (stable-sorted) JSON digest; missing-pin refusal absent (no pins yet) | 03 (envelope schema) |
| R2.1 public local CLI/MCP claim taking worker id/host/capabilities/pin echoes/requested lease; atomically ≤1 live lease per task/attempt | `AppendClaim` + `NextAttempt` exist (`internal/orchestration/acp.go:46-73`) but no command calls them; `brain` verb exposes only `start|step|run|status|cancel|resume` (`internal/core/commands.go:311-312`); MCP exposes only `brain.next` (`internal/mcp/tools_brain.go:10`) | No public `brain claim`; no `ClaimV1` (host/capabilities/pin echoes/requested lease); atomic one-live-lease-per-task enforced only for controller `brain` lease, not registered worker | 06 (authority/capability contract) |
| R2.2 lease has opaque unique id, worker id, task/mission/attempt, issued/expiry, policy digest, revocation reason; controller ≠ worker identity | `Lease{TaskID,WorkerID,ExpiresAt,Retries}` (`internal/orchestration/lease.go:5-10`); holder hardcoded `"brain"` (`internal/cmd/brain_run.go:207`) | No `lease_id`, no `mission_id`/`attempt`/`issued_at`/`policy_digest`/`state`/`revocation_reason`; controller impersonates worker — violates "controller cannot satisfy worker identity" | 06 (authority/scope) |
| R2.3 role/capability/authority/task/context/config/palette/subject mismatch or expired authority or conflicting claim refuses before work authority; loser gets typed conflict | `requireLiveLease` checks task/worker/expiry match (`internal/cmd/brain_worker.go:29-50`) but at report time, not claim time; `Authority{Enabled bool}` is a bare flag (`internal/orchestration/authority.go:3-25`) | No pre-work claim validation of role/capability/pins/authority-expiry; no typed conflict/recovery result for the losing claimant | 06 (authority/sandbox/scope verdict) |
| R3.1 worker heartbeat + cancel-ack append bounded correlated events; only matching live lease renews within policy; heartbeat never extends authority or alters pins | No heartbeat kind or command exists; ACP kinds stop at claim/report (`internal/orchestration/acp.go:17-21`); lease TTL is a fixed 15m constant (`internal/cmd/brain_run.go:14`) | Entire heartbeat/renewal/cancel-ack surface absent; a slow live worker and a dead worker are indistinguishable | 06 (policy), 07 (bounded telemetry) |
| R3.2 report binds mission/lease/worker/attempt, observable subject facts, evidence+eval refs, declared route/provider facts, status/blocker/next, bounded telemetry | ACP report event carries `Attempt/GitHead/ChangedFiles/VerifyRef/Telemetry` (`internal/orchestration/acp.go:39-43`); `workerReport` binds task/worker/lease/HEAD (`internal/cmd/brain_worker.go:11-17`) | No `ReportV1` with `lease_id`/`mission_id`/`eval_refs`/`route_fact`/`status`/`blocker`/`next_action`; no public report command | 04 (evidence/eval freshness), 07 (telemetry) |
| R3.3 report acceptance requires matching live unrevoked lease, role/task/mission/pins, passing current evidence + HEAD; must call normal completion path, never bypass | `acceptWorkerReport` → `requireLiveLease` + `requirePassingVerify` (exit 0, pinned HEAD, report==evidence HEAD) (`internal/cmd/brain_worker.go:19-73`) | Helper exists but is not wired to any command and does NOT call the normal completion mutation; no role/mission/pin/revocation match (only task/worker/expiry) — the "validated-but-bypassed" risk from the alignment doc | 04 (evidence freshness), 01 (completion path) |
| R3.4 host server-computes git HEAD/diff where possible; worker-claimed files/cost stay audit facts; disagreement retained, never converted to completion proof | Verify evidence HEAD is server-pinned via `core.HeadPinned` (`internal/cmd/brain_worker.go:60-70`); `ChangedFiles` is stored as worker-reported only (`acp.go:41`) | No server-computed diff/changed-files; no scope-disagreement retention/refusal | 06 (scope gate), 04 (evidence) |
| R4.1 checkpoint before durable mission visibility; resume reconciles checkpoint/ACP/pending missions/leases/reports/status without duplicate mission/completion | Write-ahead checkpoint before ACP append (`internal/orchestration/checkpoint.go:14-52`, `brain_run.go:210-228`); `PlanResume`/`DeriveStatus`/`HasLiveLease` reconcile (`internal/orchestration/recover.go:25-66`); `brainResume` under one spec lock (`brain_run.go:277-333`) | Reconciliation covers dispatch missions only; no claim/report/pending-mission/worker-lease reconciliation (those states don't exist yet) | 07 (recovery/measurement) |
| R4.2 retry policy classifies retryable failure, attempt bound, timeout, expiration, revocation, escalation owner, recovery; stale/revoked report cannot complete later | `ReclaimExpired`/`Escalation` on `Retries<maxRetries` (`internal/orchestration/lease.go:18-41`); `Decide` emits escalate/timeout/halt (`internal/orchestration/decide.go:47-62`); `brain_run.go:124` hardwires `MaxRetries:1` | No failure classification, no revocation, no escalation-owner; no guard that a stale/revoked report cannot later complete | 06 (escalation owner), 07 (policy limits) |
| R4.3 concurrent claims only for frontier tasks with non-overlapping write scope, unless approved coordination pins ordering; claim races yield one winner | Frontier waves derived from DAG deps (`FrontierExcluding`, `brain_run.go:117`); live-leased/escalated tasks withheld (`brain_run.go:108-116`); session CAS gives one resume/step winner (`session.go:71-91`) | No write-scope overlap check across concurrent claims; no shared-file coordination policy; race resolution is only for controller steps, not worker claims | 06 (scope), 01 (task file scope) |
| R5.1 versioned project policy maps role/risk/complexity/capabilities → eligible class, token/cost/deadline/retry limits, ordered fallback; stable route/reason | `OrchestrationConfig{Enabled,Model}` only (`internal/core/config_loader.go:17,123-125`; `config_validate.go:41-46`); `DecisionLimits{MaxCost,Deadline,MaxRetries,AllowDispatch}` (`decide.go:40-45`) but run wires only `MaxRetries:1` | No routing policy artifact, no risk/complexity/capability→class mapping, no ordered fallback, no route class/reason output | 06 (policy approval), 10 (adapter/provider) |
| R5.2 high-risk task refuses class missing required review/eval/sandbox/context capability; policy human-approved with task metadata; model prose cannot alter | `Authority.CanClearGate` refuses high/critical (`internal/orchestration/authority.go:20-25`) | No task risk/complexity/capabilities metadata; no capability-class requirement matching; no human-approval binding of routing policy | 06 (approval/sandbox), 01 (task metadata) |
| R5.3 cost/tokens/deadline/health telemetry records unit/source/knownness; required-unknown / exceeded-budget / expired-deadline / unsupported-class / exhausted-retry brakes dispatch/escalates | `EvaluateBrakes` halts on cost/deadline (`internal/orchestration/brakes.go:3-11`); `Snapshot.Cost` defaults to 0 (`sense.go:20-33`) — unset reads as zero | Missing telemetry silently = 0 (violates "unknown, never zero"); no unit/source/knownness on telemetry; brakes not wired to real config budgets | 07 (trusted measurement), 06 (policy) |
| R5.4 optional adapter resolves class→provider/model and records actual fact; provider outage follows declared fallback/escalation; never makes core gate touch network or change semantics | No adapter surface; `OrchestrationConfig.Model` is an inert string (`config_loader.go:125`) | No provider/model resolution, no route-fact recording, no fallback/escalation on outage; must stay outside trusted core (non-goal `requirements.md:72`) | 10 (adapter/provider/A2A transport) |
| R6.1 ACP envelope retains mission/claim/heartbeat/cancel/report identity/pins + redacted refs; unknown protocol import/export fails closed | Append-only ordered ledger with mission id + rigor fields (`internal/orchestration/acp.go:23-44`); read numbers seq by position (:150-171); history projection reads ACP (`internal/cmd/report.go:131-144`) | No `protocol_version` on envelope; no heartbeat/cancel kinds; no pins; no redaction rule; no unknown-protocol import/export refusal | 10 (transport), 07 (export) |
| R6.2 local subprocess, MCP host, future A2A produce same semantic ACP stream for canonical fixture (minus transport metadata) | Single internal in-process path only; MCP exposes `brain.next` (`internal/mcp/tools_brain.go:10`), no claim/report tools | No adapter abstraction, no conformance fixture, no cross-transport equivalence | 10 (A2A/MCP adapter conformance) |
| R6.3 export contains ids/digests/status/reasons, not secrets/raw prompts/source body/hidden reasoning/unbounded tool output | Report/history projection is derived from on-disk records and stays bounded (`internal/cmd/report.go:18-147`); telemetry stored verbatim (`acp.go:43`) | No explicit redaction gate guaranteeing no secrets/prompts/source; telemetry bound unenforced | 07 (redacted export) |

## P0 gaps (align with alignment-doc P0 + README W0/W1/W2 program rules)

1. **Untruthful dispatch = launch.** `Dispatch` writes a live lease with
   `WorkerID:"brain"` (`internal/cmd/brain_run.go:205-208`) the moment it records
   a dispatch, so "pending mission" and "worker launched" are indistinguishable.
   Needs a `pending` mission state with no worker lease (R1.1, README rule 1).
2. **No versioned mission envelope / no pins.** There is no `MissionV1`
   (`design.md:26-32`); a dispatch ACP event carries only `TaskID`+`MissionID`
   (`brain_run.go:221-227`). Nothing pins role/files/acceptance/verify/context/
   config/palette/authority/subject, so stale claims/reports cannot fail closed
   (R1.2, R1.3, README rule 3).
3. **Controller impersonates the worker.** `Lease` has no `lease_id` and holder
   is hardcoded `"brain"` (`lease.go:5-10`, `brain_run.go:207`); requirement is a
   registered worker id + unique lease id, controller identity may never satisfy
   worker identity (R2.1, R2.2, README rule 2).
4. **No public claim/heartbeat/report lifecycle.** `AppendClaim` (`acp.go:63`)
   and `acceptWorkerReport` (`brain_worker.go:19`) exist but no command or MCP
   tool reaches them — `brain` verb is `start|step|run|status|cancel|resume`
   (`commands.go:311-312`), MCP is `brain.next` only. And the report helper never
   invokes the normal completion path, so it is validated-but-unwired (R2.1, R3.1,
   R3.3).
5. **Missing = zero.** `Snapshot.Cost` defaults to 0 and brakes compare against it
   (`sense.go:20-33`, `brakes.go:3-11`); `OrchestrationConfig` is `{Enabled,Model}`
   only (`config_loader.go:17,123-125`) with no budgets wired (run uses
   `MaxRetries:1`). Required-unknown telemetry must brake, never read as zero
   (R5.3, README rule 6). This is W4/W1 P1 in the doc but the zero-default is a
   correctness hazard worth flagging now.

## Boundary summary (README "Ownership" table confirmed against code)

- Domain 05 owns: pending-mission/claim/lease/heartbeat/report state machine,
  `MissionV1`/digest pinning/ACP lifecycle, correlated completion-input checks,
  deterministic eligible capability class + limits + route reason, telemetry fact
  envelope + unknown semantics.
- 01 = task/phase + completion semantics; 02 = context receipt/selection/digest
  (mission `context_ref`/`context_digest`); 03 = dispatch envelope (05b input);
  04 = evidence/eval freshness (report acceptance); 06 = authority/policy/sandbox
  + harness-derived scope verdict (claim + routing approval); 07 = trusted
  measurement + redacted export (telemetry/brakes); 10 = adapter/provider/model
  + A2A/MCP transport (R5.4, R6.2). 05 consumes these contracts, never
  reimplements them (`design.md:63-65`).
