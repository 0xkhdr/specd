# Design — Deterministic orchestration lifecycle

## Decision

Split controller policy from delivery. Brain makes a durable pending mission; host adapter may
deliver it. Worker then claims a lease. Core validates every later transition from pinned mission,
current local state, and evidence. Provider/model selection is adapter work after core selects only
eligible capability class.

```text
tasks + state + policy → Sense/Decide → checkpoint → ACP pending mission
                                                    ↓
                              adapter delivery (optional, external) → registered worker
                                                                    ↓
claim → CAS live lease → heartbeat → verify/eval evidence → report → normal completion gate
  ↑             ↓                                      ↓                   ↓
conflict     cancel/expiry/revoke                  typed finding       ACP audit projection
```

No adapter present: command reports `pending`/`not delivered`; no active worker lease and no
launch claim. Keep current documented `brain` lifecycle compatible while adding versioned
`brain claim|heartbeat|report` surfaces only after baseline test locks wording and mutations.

## Canonical contracts

```text
MissionV1
  protocol_version, session_id, mission_id, spec_slug, task_id, attempt,
  role, authority_ref, declared_files[], acceptance[], verify,
  context_ref/context_digest, config_digest, palette_digest, policy_digest,
  subject_head/diff_digest?, route_class/route_reason, limits, issued_at/expires_at

ClaimV1
  protocol_version, mission_id, task_id, attempt, worker_id, host, capabilities[],
  role, mission_digest, pinned_digests, requested_lease, actual_route?

LeaseV1
  lease_id, mission_id, task_id, attempt, worker_id, issued_at, expires_at,
  policy_digest, state, revocation_reason?

ReportV1
  protocol_version, mission_id, lease_id, task_id, attempt, worker_id, status,
  observed_head/diff_digest?, changed_files_claim[], evidence_refs[], eval_refs[],
  route_fact?, telemetry?, blocker?, next_action?
```

Canonical JSON only semantic fields; arrays stable-sort by identity. Store large data through
root-relative ref plus digest. Keep ledger payload bounded. Existing ACP entries read as legacy
until explicit migration; V1 mutable operation rejects unknown or ambiguous input.

## State transition rules

1. `pending`: controller writes checkpoint then exactly-one mission event. No lease.
2. `claim`: under `WithSpecLock`, load session/ACP, validate mission/pins/authority/capability;
   atomically append claim and CAS session lease. One live lease per task.
3. `heartbeat`: matching active lease only; append bounded event; renew only policy permits.
4. `report`: server observes subject/diff where local; validates lease/pins/current evidence;
   appends report before invoking standard task completion mutation under same authority boundary.
5. `expiry|cancel|revoke`: append causal event, make lease ineligible forever; policy decides retry
   versus escalation. Resume projects ledger/session/checkpoint to same result after any crash.

Pins include task semantic digest (role/files/acceptance/verify), context/config/palette/policy,
authority and subject. Domain 02 owns receipt production; Domain 04 owns evidence freshness;
Domain 06 owns authority, sandbox and harness-derived scope verdict. Domain 05 consumes their
stable contracts rather than reimplementing them.

## Routing policy

`project.yml` gains versioned optional routing section only after parser/config validation fixture:

```text
task metadata (risk, complexity, required capabilities)
  + approved routing policy + trusted limits
  → eligible capability classes ordered by policy → selected class + deterministic reason
  → adapter mapping → actual provider/model telemetry fact
```

Missing required telemetry is `unknown`; policy either explicitly allows unknown or dispatch
brakes. Cost uses declared integer micro-units or documented fixed unit—not float prose.
Adapters may fail/fallback only according to chosen policy; they cannot mutate mission pins.

## Verification ladder

1. Unit/golden: envelope canonicalization, version/refusal, transition legality, route order.
2. Race/CAS: simultaneous claims/reports/renewals; exactly one live lease/completion.
3. Black-box: no-adapter truthful status; valid fake-host lifecycle; stale pins; wrong role;
   expired/revoked lease; crash points checkpoint/append/CAS; parallel overlap conflict.
4. Conformance: local CLI/MCP/A2A canonical event equivalence and redaction.
5. Full race/vet/lint/regression after integrated domain.
