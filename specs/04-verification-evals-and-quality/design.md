# Design — Quality proof contracts

## Decision

Keep two planes separate:

```text
external adapter/CI/human/judge → versioned JSON/JSONL artifact → import/store
                                                           ↓
task declaration + policy + current subject → pure core validation/gates → completion/report
                                                           ↑
existing `specd verify` → append-only test evidence ──────┘
```

`verify` creates required `test` evidence. Adapter execution never happens in a gate. Import
validates schema/identity/digests; gates evaluate fixed local records. All maps sort explicitly
before serialization/findings. Stored references/digests replace large/sensitive bodies.

## Contract model

Use versioned envelope with common identity and class payload:

```text
EvidenceEnvelopeV1
  schema_version, evidence_id, evidence_class, spec_slug, task_id, run_id, attempt,
  subject_revision, diff_digest?, producer, producer_version, config_digest,
  check_id, verdict, score?, created_at, actor, artifact_ref, artifact_digest

OutputEvalPayload: dataset/rubric ids+versions+digests, cases[], scorer metadata,
                   output_ref/digest, repetitions, aggregation, threshold
TrajectoryPayload: trace_ref/digest, event_count, required_steps, forbidden_steps
ReviewPayload: review ref/digest, approval identity, dimensions?
```

Task declaration names required `class/check_id` pairs and quality policy refs. Prefer a companion
quality artifact when task-table extension risks byte stability; choose only after T03 migration
fixture. Legacy verify records are adapted as class `test` at read boundary, not rewritten.

## Subject and freshness

Freshness predicate compares required record identity to task/spec and current reachable HEAD.
Policy selects additional subject digests: diff for source work; output for output eval; dataset /
rubric for labelled eval; trace for trajectory. A digest mismatch produces `EVIDENCE_STALE`, does
not delete historical evidence. Completion requires passing `verify` plus all required pairs.

## Trace policy

Normalized trace is JSONL, one event per line, run/event ids unique, sequence strictly increasing.
Allowed data: tool/action id, bounded sanitized args/result class, paths/effects, timestamps,
actor/correlation. Reject reasoning/prompt/secret fields rather than attempting vague masking.
Trajectory evaluator checks required/forbidden identities and result predicates against normalized
events. Scope evidence comes from trusted diff contract when Domain 06 delivers it; no trust in
worker self-report.

## Risk coverage

Requirements/acceptance criteria receive stable ids. Quality policy maps each critical id to
required check/eval ids and risks. Production lint rejects no map, unknown refs, shallow verify
where declared risks require integration/failure/concurrency/rollback proof, or waiver without
scope/owner/expiry. It describes contract quality; it cannot infer semantic test adequacy.

## Adapter and governance

`eval import`/`eval status` are local CLI surfaces added only after core envelope/gates. Adapter
output must pin tool/config/scorer and policy inputs. LM judge record adds provider/model/prompt
digest/sampling; import does not contact it. Dataset/rubric manifests are immutable by digest;
case bodies remain external/redacted refs. Aggregation over fixed records is pure, deterministic,
critical-case first.

## Context/report and migration

Context gets compact labelled `quality_contract` entries and refs; Domain 02 owns selection/budget.
Reports project class/verdict/score/freshness separately. Start V1 behind compatibility fixture;
preserve plain CLI and old specs until docs/migration tests pass. Atomic write every artifact;
append-only evidence and CAS state remain unchanged.

## Validation ladder

1. Golden/schema/parser tests: ordering, byte stability, unknown fields/classes, legacy behavior.
2. Gate tests: bad identity/digest/current HEAD/test failure/refusal, offline only.
3. Trace tests: duplicate/order/secret/missing/forbidden/diff discrepancy.
4. Black-box fixtures: production coverage, stale dataset/rubric, external adapter unavailable.
5. Full race/vet/lint/regression proof after domain integration.
