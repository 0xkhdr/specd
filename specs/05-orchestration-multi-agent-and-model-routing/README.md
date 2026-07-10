# Domain 05 — Orchestration, multi-agent work, model routing

## Goal

Turn Brain from durable dispatch ledger into truthful, recoverable worker lifecycle.
Core computes frontier, authority, lease, scope, freshness, route eligibility. External host
adapters deliver missions or resolve provider/model. No model/network call enters core gate.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/05-orchestration-multi-agent-and-model-routing.md`.
Paper intent: conductor retains control; orchestrator delegates bounded work; MCP/A2A handoff
keeps identity/evidence; routing spends capable model only where policy requires it.

Current state: DAG/frontier, CAS session, ACP, checkpoint recovery, brakes, and report helper
exist. Gap: `brain run` records controller lease (`worker_id=brain`), launches nobody; public
claim/heartbeat/report flow absent; mission inputs not pinned; limits/routing not wired.

## Ownership

| Area | Domain 05 owns | Other domain owns |
|---|---|---|
| Controller | pending mission, claim/lease/heartbeat/report state machine | Domain 01 task/phase semantics |
| Mission | V1 envelope, digest pinning, ACP lifecycle | Domain 02 context receipt/selection |
| Completion input | correlated worker/lease/HEAD/evidence reference checks | Domain 04 evidence/eval freshness |
| Authority/scope | consume role, authority, actual-diff verdict | Domain 06 policy/sandbox/scope gate |
| Routing | deterministic eligible capability class, limits, route reason | Domain 10 adapter/provider/A2A transport |
| Telemetry | fact envelope and unknown semantics | Domain 07 trusted measurement/export |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `05a-orchestration-contract-baseline` | observed behavior, public wording, failing lifecycle fixtures | — |
| W1 | `05b-mission-and-pending-dispatch` | V1 mission + pending state; no fake worker lease | 05a, Domain 03 dispatch envelope |
| W2 | `05c-worker-claim-heartbeat-report` | registered identity, CAS lease, public lifecycle commands | 05b, Domain 04/06 contracts |
| W3 | `05d-recovery-and-host-conformance` | fake-host E2E, retries/cancel/recovery, parallel conflict policy | 05c |
| W4 | `05e-routing-limits-and-local-observation` | deterministic capability route and fail-closed brakes | 05c, Domain 06/07 policy |
| W5 | `05f-a2a-and-adapter-conformance` | canonical transport mapping and redacted conformance | 05d,05e, Domain 10 contract |

## DAG

```text
05a → 05b → 05c ─┬─> 05d ─┐
                  └─> 05e ─┼─> 05f

Domain 03 envelope ─> 05b
Domain 04 evidence + Domain 06 authority/scope ─> 05c,05e
Domain 10 transport ─> 05f
```

## Program rules

1. `dispatch` means pending mission only until adapter delivery is implemented and observed.
2. Active lease always names registered worker plus unique lease id; controller never impersonates worker.
3. Mission pins task/role/files/acceptance/verify/context/config/palette/authority/subject inputs.
   Any changed pin rejects stale claim/report before completion.
4. Every lifecycle transition runs under per-spec lock and CAS; ACP is append-only, ordered, versioned.
5. Core route chooses capability class/reason only. Adapter provider/model facts never authorize gate.
6. Cost/token/health absence = `unknown`, never zero/pass. Required unknown input brakes dispatch.
7. Preserve old CLI behavior until versioned command/envelope migration fixture passes. Stdlib-only;
   no `reference/` edits.

## Completion claim

Fresh fixture can initialize orchestration, create one pending mission, fake worker claim/heartbeat,
record passing evidence, submit report, complete once, and recover from crash/retry without duplicate
mission or stale result. Docs never claim worker launch where no adapter ran. Routing decisions are
stable and auditable; adapters cannot weaken authority, scope, evidence, or freshness gates.
