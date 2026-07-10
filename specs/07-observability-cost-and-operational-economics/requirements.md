# Requirements — Observability, cost, and operational economics

## Scope

Add deterministic run/span correlation, honest telemetry trust, correct context accounting, an
active-or-removed cost brake, privacy/cardinality policy, trace/event exports, and economic
roll-ups around existing evidence/report/orchestration surfaces. Preserve atomic writes, state CAS,
reentrant spec lock, append-only evidence, no-bypass verify, exact-rational cost, byte-stable
tasks parser, offline/stdlib-only core, `go:embed` templates, and backward-compatible decoding.
No LLM in any measurement, gate, or report path. No new runtime dependency.

### R1 — Versioned run/telemetry envelope and trust semantics

- R1.1: A versioned envelope shall wrap telemetry, evidence, and ACP records. Old evidence/ACP
  fixtures shall decode unchanged; canonical new records shall round-trip byte-stably. Legacy
  `Annotations` shall be preserved.
- R1.2: Malformed decimals, negative units, cost-present-without-currency, and unknown *required*
  schema versions shall fail closed. A missing optional field shall stay explicitly absent, never
  defaulted to zero.
- R1.3: Every measured value shall carry a `telemetry_source` provenance (`worker`,
  `provider_adapter`, or `operator`). Reports shall render worker-reported values as reported, never
  as independently measured. Attestation reference stays external and optional.

### R2 — Run correlation and attempt identity

- R2.1: Core shall allocate a deterministic `run_id` and monotonic `attempt` under the spec lock,
  stable through retry and recovery. A provider trace ID shall never be the primary key.
- R2.2: Manual and Brain task runs shall produce one stable run identity per loop; two failures and
  a pass shall remain three attempts on one task/run chain. Racing writers shall not duplicate an
  attempt.
- R2.3: The run ledger (`runs.jsonl`) shall be additive; the evidence gate shall pass or fail
  identically whether the ledger is present or absent. Completion authority stays verify/eval.
- R2.4: A crash during append shall yield either the prior complete record or one complete new
  record — never a partial line or duplicate run/mission.

### R3 — Context accounting and sufficiency

- R3.1: The manifest estimate shall cover the bytes the contract instructs the host to load —
  including `design.md` and the task's declared source files — not only reference-string length.
- R3.2: Required items (approved requirements, relevant design, exact task row, role constitution,
  declared files, directly needed steering) shall not be silently dropped. An over-budget required
  set shall fail with a concise remediation, not a starved agent.
- R3.3: Estimated footprint, host-reported actual input tokens, and provider-billed tokens shall be
  distinct fields; a passing `context-budget` gate shall not be presented as proof the model saw
  every item. The host shall acknowledge loaded item digests; the harness records the ack.
- R3.4: Repeated manifests over the same tree shall be byte-identical and carry a canonical
  `context_manifest_digest` over manifest + effective config + role + steering versions, recording
  supplied vs. omitted items and reasons.

### R4 — Honest cost brake

- R4.1: The cost brake shall be either wired end to end from accepted telemetry or the dormant
  public implication removed/deferred. No test shall populate `Snapshot.Cost` in a way production
  cannot.
- R4.2: A configured threshold shall halt only *subsequent* dispatch, record the exact reason, take
  no worker lease, and undo no completed call. Without configured limits, behavior shall match today.
- R4.3: When production policy requires trusted/attested telemetry and it is missing or malformed,
  dispatch shall fail closed with one actionable message; a brake based on untrusted data shall say
  so.

### R5 — Privacy and metric-cardinality policy

- R5.1: A static contract test shall reject Prometheus label additions outside an allowlist. No
  run/mission/SHA/path/model/actor/error-text label may enter metrics.
- R5.2: Default trace/run fixtures shall contain no prompt, model response, chain-of-thought, file
  contents, raw command output, secrets, or absolute home paths. Detail lives in bounded local
  artifacts referenced by hash.
- R5.3: `evidence_ref` shall be workspace-relative or content-addressed only; traversal and URLs are
  rejected in the core schema unless an adapter resolves them.
- R5.4: Policy shall be documented in `docs/telemetry-schema.md`, `docs/observability.md`, and
  `SECURITY.md`; central redaction applies before any display or export.

### R6 — Metadata-only run spans and trace export

- R6.1: Metadata-only spans shall cover context load, model boundary, tool call, edit, verify, eval,
  approval, and dispatch. `kind` is a versioned closed enum with an extension field so unknown
  *critical* kinds fail parsing rather than silently disappear.
- R6.2: `report --trace --json` shall be a pure projection: parent references resolve, span IDs are
  unique within a run, ordering is stable on equal or missing timestamps, and two exports over the
  same tree are byte-identical.
- R6.3: A span claiming code effects or completion shall carry `git_head`. Timestamps are
  informational; no gate shall derive an outcome from wall-clock ordering alone.

### R7 — Provider-neutral annotation expansion

- R7.1: Optional additive fields shall carry `input_tokens`, `output_tokens`, `cached_tokens`,
  `provider`, `model`, `currency`, `pricing_ref`, `telemetry_source`, and `attestation_ref`. All
  remain optional; the legacy `tokens` total is retained for compatibility.
- R7.2: Exact-decimal cost aggregation shall have golden tests; the legacy total and new category
  totals shall never silently contradict each other. `currency`/unit and `pricing_ref` are mandatory
  when a cost is present.
- R7.3: `provider`/`model` shall be optional bounded identifiers from an adapter, never required for
  valid deterministic evidence, and never used as a metric label.

### R8 — Neutral event schema and context efficiency

- R8.1: A versioned provider-neutral local event schema shall render without networking; golden
  output validates against the documented schema; an external adapter maps it to OpenTelemetry.
  Core performs no network call and adds no module dependency.
- R8.2: An adapter round-trip fixture shall preserve correlation and privacy fields.
- R8.3: A context-efficiency report shall deterministically show estimated/actual tokens, omitted
  items, retry count, first-pass result, duration, and cost, with explicit `unknown` values rather
  than zeros.

### R9 — Attested ingestion, routing metadata, and roll-ups

- R9.1: An adapter ingestion contract shall accept signed/hashed provider-attested envelopes from an
  allowlisted key/config and reject tampering; all adapter tests run with networking disabled.
- R9.2: Model-routing recommendations shall be policy metadata, not model execution: the same
  task/config always yields the same routing class; no gate contacts a provider; an unavailable
  recommended model stays an adapter concern and cannot bypass evidence.
- R9.3: Cross-spec economic roll-ups shall be exact, stable, bounded-cardinality, and clearly
  separate missing telemetry from zero. Threshold/drift alerts reference source records and never
  mutate lifecycle state.

## Non-goals

- No provider SDK, pricing table, or hosted collector in core; optional adapters translate external
  data while `specd` stays stdlib-only and offline-capable.
- No model call in any gate or report; rubric/eval adapters may produce evidence, the core only
  validates declared contracts.
- No claim that worker telemetry is billing truth; trust source and attestation stay visible.
- No automatic cheapest-model selection in the first implementation — record and recommend before
  automating.
- No high-cardinality Prometheus trace model; correlation lives in JSONL/trace export.
- No raw-content telemetry by default; no gate derived from wall-clock ordering or from estimates
  presented as tokenizer truth.
