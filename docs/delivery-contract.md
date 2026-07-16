# specd — Delivery contract (draft)

Delivery is a **parallel, additive ledger domain**. It never adds a value to the six lifecycle
status states and never satisfies a task evidence gate. Everything below is a pure function of
on-disk `.specd/` ledgers plus validated adapter envelopes — no LLM, no network in core, no
timeout-default health. This document drafts the envelope fields and the closed state machine that
Domain 08 waves 08d/08g/08h implement; the canonical offline fixtures live under
`testdata/delivery/` (see `internal/core/delivery_fixtures_test.go`).

Status: **draft** (08a baseline). Field names here are the contract the later waves pin.

## Envelopes

Every envelope is versioned (`schema` field, e.g. `ReleaseCandidateV1`) so an older binary rejects
an unknown schema before mutation rather than silently reinterpreting it.

### ReleaseCandidateV1 → `releases.jsonl`

Immutable, reproducible identity frozen by `specd release candidate`; builds and uploads nothing.

| field | meaning |
|---|---|
| `release_id` | immutable, unique; derived from `artifact_digest` + human version, **never** from an environment name |
| `spec_id`, `spec_revision` | source spec and its approved revision |
| `git_head` | resolvable commit the evidence set is pinned to |
| `task_evidence_set_digest` | digest over the passing verify records (completion authority stays Domain 04) |
| `artifact_digest` | digest of the built artifact; substitution after candidacy fails a digest check |
| `sbom_ref`, `provenance_ref` | bounded references, not inlined payloads |
| `bootstrap_digest` | the agent-bootstrap packet digest (08b) |
| `state_schema` | state/delivery schema version this candidate was frozen under |
| `created_at` | RFC3339 timestamp |

### DeploymentV1 → `deployments.jsonl`

One attempt appended under `WithSpecLock`; a duplicate `idempotency_key` is a no-op or a
fail-closed conflict, never a second logical deployment.

| field | meaning |
|---|---|
| `deployment_id` | adapter-provided identity |
| `attempt` | harness monotonic per `release_id`+`environment` |
| `release_id`, `environment` | must match an allowlisted, closed environment name |
| `status` | one of the closed set below |
| `strategy`, `population`, `window` | canary/rollout policy in effect |
| `adapter`, `authority`, `actor` | trust source + authority + who acted (all visible) |
| `idempotency_key` | dedupe key for adapter callbacks |
| `started_at`, `finished_at` | attempt bounds |
| `telemetry_source`, `evidence_ref`, `attestation_ref` | bounded references to observation/attestation |

### HealthObservationV1

| field | meaning |
|---|---|
| `deployment_id`, `criterion_id` | which attempt + which required criterion |
| `health_check`, `threshold`, `observation` | the check and its measured value |
| `freshness` | `observed_at` + `max_age`; a stale or missing observation fails closed |
| `release_identity` | release/artifact/environment identity the observation is bound to |
| `source` | trust source label |

### RollbackV1

| field | meaning |
|---|---|
| `deployment_id` | the attempt being rolled back |
| `rollback_target` | the `release_id` to restore (required; a `failed` without a target is rejected) |
| `reason` | why |
| `adapter`, `action_result` | who performed it and the raw result label |
| `post_rollback_health` | rollback is complete **only** after target health passes |
| `capability_class` | reversibility classification |
| `human_required` | true where reversal is not demonstrably safe |

## Closed state machine

```text
requested → started → observing → healthy
                              └──→ failed → rolling_back → rolled_back
```

Transitions are a table, not free code. Rejected **before any append**:

- an unknown status value,
- a jump (e.g. `requested → healthy`, skipping `observing`),
- a mismatched `git_head` / `artifact_digest`,
- a stale or missing observation on a transition that requires fresh evidence,
- a `failed` transition without a `rollback_target`.

A canary is never `healthy` by timeout default and cannot promote before its full fresh window.
"command issued" is not "rolled_back".

## Additivity invariant

The evidence gate and `complete` behave **identically** whether these ledgers are present or
absent. No delivery record retroactively changes a task's `complete`. Source completion
(`submit`) and environment health are distinct states that reports label separately.

## Fixture plan — 15 production validation scenarios

Each production validation scenario gets a deterministic offline fixture; the four canonical
envelope fixtures are landed, and each remaining scenario fixture lands with the rule it
exercises.

| # | scenario | planned fixture | landing wave |
|---|---|---|---|
| 1 | Fresh production-like install | `scripts/production-smoke.sh` lifecycle | 08e |
| 2 | Agent guide conformance | `handshake` bind fixture | 08b |
| 3 | Wrong workspace/spec | `handshake` mismatch fixture | 08b |
| 4 | Unauthorized production attempt | `deployment_unauthorized.json` | 08h |
| 5 | Artifact substitution | `release_candidate.json` + swapped-digest fixture | 08d/08h |
| 6 | Canary success | `deployment.json` + `health_observation.json` | 08j |
| 7 | Canary failure and rollback | `rollback.json` | 08j |
| 8 | Monitoring outage/stale data | `health_stale.json` | 08j |
| 9 | Duplicate/racing callbacks | `deployment_idempotent.json` | 08i |
| 10 | Agent/controller crash | ledger torn-line fixture | 08g |
| 11 | N-1 upgrade | `scripts/upgrade-matrix.sh` fixtures | 08l |
| 12 | Future-schema/downgrade attempt | future-schema state fixture | 08f |
| 13 | Corrupt installed binary | staged-binary fixture | 08f |
| 14 | Air-gapped CI | offline candidate/report fixture | 08d |
| 15 | Secret/prompt injection in production output | `envelope_hostile.json` | 08i |

The four canonical envelope fixtures landed by 08a — `release_candidate.json`, `deployment.json`,
`health_observation.json`, `rollback.json` — pin the field and state definitions above so the later
waves extend rather than redefine them.
