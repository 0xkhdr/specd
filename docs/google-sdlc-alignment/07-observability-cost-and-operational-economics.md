# Domain 07 — Observability, Cost, and Operational Economics

## Purpose

Define the production observability and economic contract that lets operators answer four
questions without trusting an agent's narrative:

1. What did the coding agent and harness do, in what order, and against which source revision?
2. What context was offered, retained, or omitted, and was the packet sufficient but bounded?
3. What latency, tokens, and cost did the model-provider boundary report?
4. Can the same on-disk facts be replayed, exported, and audited without network access or an LLM?

This domain extends `specd`'s existing evidence and reporting strengths. It must preserve the
local deterministic core, zero runtime dependencies, no-LLM gate/report paths, and optional
external adapters. It is not permission for `specd` to become a hosted telemetry service.

## Paper position

Google's paper treats observability as a first-class harness component alongside instructions,
tools, sandboxes, orchestration, and guardrails. It specifically names logs, traces,
evaluations, cost, and latency metering, because without them a team cannot see quiet drift or
audit why an agent made a deployment decision (`sdlc-paper.md:258-317`). The paper also makes
the economics operational:

- uncontrolled context and repeated correction loops create a high token burn rate;
- dense, dynamic, task-specific context is a financial lever, not merely prompt hygiene;
- tests/evals, root-cause clustering, regression checks, and production observations form a
  continuous quality flywheel (`sdlc-paper.md:220-226`);
- model routing should reserve expensive models for high-complexity work and use cheaper models
  for deterministic or lower-complexity work (`sdlc-paper.md:420-458`).

The paper's requirement is therefore broader than “show a token total.” Production alignment
requires correlated trajectories, explicit trust boundaries, latency/cost attribution, context
efficiency, and feedback into policy. The harness must expose facts; humans and optional policy
adapters decide how to optimize them.

## Current specd handling with evidence paths

`specd` already has a meaningful deterministic observability base. The comparison document
understates this current implementation in places.

| Current capability | Evidence | Assessment |
|---|---|---|
| Append-only verify-attempt ledger | `internal/core/evidence.go`, `.specd/specs/<slug>/evidence.jsonl` | Records `task_id`, command, exit code, git HEAD, timestamp, actor, optional evidence reference, and optional telemetry. Passing completion still requires exit `0` pinned to a real HEAD. |
| Worker-reported telemetry | `internal/core/telemetry.go`, `internal/cmd/registry.go` | `specd verify` and task completion accept optional `--tokens`, `--cost`, and `--duration-ms`. Values are validated, stored verbatim, never estimated, and absence remains explicit. Cost sums use exact rational arithmetic. |
| Orchestrated trajectory fragments | `internal/orchestration/acp.go`, `internal/cmd/brain_worker.go` | ACP events carry sequence, time, kind, task, deterministic mission ID, attempt, git HEAD, changed files, verify reference, and optional telemetry. This is useful trajectory evidence, but not yet a general run/span model. |
| Deterministic audit replay | `internal/core/history.go`, `internal/cmd/report.go` | `report --history [--json]` projects existing ledgers into a stable total order. It is read-only and does not invent missing timestamps or actors. |
| Metrics and export | `internal/core/report_metrics.go`, `internal/core/prometheus.go`, `docs/observability.md` | `report --metrics` gives task and telemetry totals; `report --format prometheus` emits node-exporter textfile format. Reports are local, network-free, and state-derived. |
| Bounded context manifest | `internal/context/manifest.go`, `internal/context/budget.go`, `internal/core/gates/contextbudget.go` | Context is referenced per task and memory is shed before steering. `context.max_tokens` is gateable and `--hud` exposes the estimate. |
| Context scale checks | `internal/context/perf_test.go`, `docs/scale-envelope.md`, `scripts/perf-gate.sh` | Manifest construction is pinned against task-count amplification and disabled-budget overhead. |
| Crash/recovery observability | `internal/orchestration/checkpoint.go`, `internal/orchestration/recover.go`, `scripts/stress-acp.sh`, `scripts/stress-checkpoint-fault.sh` | Deterministic mission IDs, checkpoint ordering, ledger replay, and stress tests protect against duplicate dispatch and orphaned work. |
| Privacy baseline | `docs/observability.md`, `SECURITY.md`, `internal/core/verify/exec.go` | Core reports do not phone home. Verify uses a scrubbed environment; optional sandboxing fails closed. |
| Cost brake skeleton | `internal/orchestration/brakes.go`, `internal/orchestration/sense.go`, `internal/cmd/brain_run.go` | A `MaxCost` decision field and brake exist, but production sensing/CLI do not populate or configure them. The tested primitive is not an active operational budget. |

Two distinctions matter. First, `HistoryEvent` is an audit projection, not a full trace: it has
no parent/child spans or general run correlation. Second, telemetry is explicitly
worker-reported. It is useful accounting input, but it is not provider-attested truth.

## Common contract and fields

The following is the smallest common vocabulary between the paper's observable harness and
`specd`'s current ledgers. “Target” fields should be additive and versioned; older JSONL records
must continue to decode.

| Contract field | Paper meaning | Current specd mapping | Target rule |
|---|---|---|---|
| `spec_id` / `slug` | Work/product boundary | State and every report are slug-scoped | Required on export envelopes; avoid repeating it in each per-spec on-disk line unless needed for portability. |
| `task_id` | Unit of delegated work | Evidence and ACP records | Required for task spans; optional only for spec/release spans. |
| `run_id` | One agent loop/run | ACP has session and mission identity; verify evidence does not | Deterministic harness ID; stable through retry/recovery. Never use provider trace ID as the primary key. |
| `span_id`, `parent_span_id` | Ordered trajectory | Only source sequence/order exists | Deterministic within a run; span type is a closed enum. |
| `attempt` | Retry economics and failure clustering | ACP claim count; evidence append order | Explicit, monotonic per task/run, derived under the spec lock. |
| `kind` | Model/tool/context/edit/verify/eval/approval/deploy activity | History event and ACP kind strings | Versioned closed enum with an extension field, so unknown critical kinds fail parsing rather than silently disappearing. |
| `started_at`, `ended_at`, `duration_ms` | Latency | Timestamp plus optional worker duration | Timestamps are informational; duration is reported with a source/trust label. Never derive a gate from wall-clock ordering alone. |
| `git_head` | Artifact identity | Evidence, ACP, state records | Required for any span claiming code effects or completion. |
| `actor`, `worker_id` | Human/agent attribution | Host-reported actor; ACP lease worker | Labels, not authentication. Export must preserve that trust caveat. |
| `tool`, `command`, `exit_code` | Action/result | Verify evidence and history references | Keep command output bounded/redacted; store hashes or references for large payloads. |
| `changed_files` | Trajectory/scope compliance | ACP worker report | Normalize, sort, and compare against task `files:`; do not use file paths as metric labels. |
| `context_manifest_digest` | Exact dynamic-context policy | Manifest has version/items/estimate but no digest | Digest canonical manifest, effective config, role, and steering versions; record supplied vs. omitted items and reasons. |
| `estimated_input_tokens` | Pre-run context budget | Manifest estimate | Estimate the content the host is expected to load, not only reference-string length; distinguish estimate from actual. |
| `input_tokens`, `output_tokens`, `cached_tokens` | Provider token accounting | One undifferentiated `tokens` integer | Optional, non-negative, provider-reported; retain legacy total for compatibility. |
| `cost`, `currency`, `pricing_ref` | Economic attribution | Currency-agnostic decimal cost | Cost stays exact decimal. Currency/unit and pricing source are mandatory when a cost is present. `specd` never prices tokens itself. |
| `provider`, `model` | Model routing attribution | Absent | Optional bounded identifiers from the adapter; never required for valid deterministic evidence. |
| `telemetry_source`, `attestation_ref` | Trust boundary | Implicitly “worker reported” | Required provenance: `worker`, `provider_adapter`, or `operator`; attestation remains external and optional. |
| `status`, `error_class` | Outcome and failure clustering | Exit code, task status, event name | Deterministic status; optional bounded error class. Raw error/output stays referenced, capped, and redacted. |
| `evidence_ref` | Link to detailed artifact | Evidence/ACP reference fields | Workspace-relative or content-addressed reference only; reject traversal and URLs in the core schema unless an adapter resolves them. |

## Gaps and failure modes

### 1. Audit history is not a complete run trace

Manual conductor runs, context construction, model calls, tool calls, edits, retries, and
approvals cannot currently be correlated into one hierarchy. ACP gives better orchestration
facts, but verify evidence is task-keyed and the history view merges sources after the fact.
This cannot answer “which context and model call caused this failed verify?”

### 2. Telemetry is coarse and untrusted by design

`tokens` combines input/output/cache categories; `cost` has no currency, provider, model, or
pricing reference; `duration_ms` has no definition of elapsed segment. A worker can omit or
misreport all three. Current behavior correctly avoids fabricating values, but reports must not
present reported values as independently measured facts.

### 3. The orchestration cost brake is not wired

`EvaluateBrakes` can halt on `Snapshot.Cost`, but `Sense` leaves cost unset and `brain_run`
constructs limits with retries only. An operator may infer budget enforcement from the types and
tests even though no production command activates it. This is a misleading partial feature and
must either be wired end to end or explicitly removed/deferred.

### 4. Context budget can underestimate the host's actual payload

Steering and memory estimates use file size, but the four “core” manifest items estimate token
cost from their kind/path/task-ID strings. The manifest references all of `requirements.md` and
`tasks.md`, omits `design.md` and the task's declared source files, and does not prove what the
host actually loaded. Consequently, a passing `context-budget` gate does not yet prove that the
agent received sufficient context or stayed under its real model input budget.

### 5. Context quality is not connected to outcome economics

There is no run-level relation between manifest digest, first-pass success, retries, latency,
and cost. Teams cannot tell whether a large steering file improves success, whether stale memory
causes retries, or whether a context item is never used.

### 6. Export scope and cardinality are underspecified

Prometheus labels are currently low-cardinality (`spec`, task/status where applicable), which is
a good base. A future trace implementation could accidentally add mission IDs, git SHAs, paths,
models, actors, or error text as labels and make production metrics expensive or unsafe.

### 7. Trace privacy could regress the current no-phone-home posture

Prompts, model responses, command output, file contents, actor names, evidence references, and
paths may contain source code, secrets, or personal data. A naive OpenTelemetry exporter would
turn a local audit harness into a data-exfiltration path. The existing secret scanner and output
truncation do not by themselves make arbitrary trace payloads safe.

### 8. Model/provider policy is outside the observable contract

Keeping model choice outside deterministic gates is correct, but absent provider/model/source
fields make routing quality and cost impossible to audit. Conversely, embedding provider SDKs or
pricing tables in core would violate the local, vendor-neutral, zero-dependency design.

## Target best-practice workflow

1. `specd context <slug> <task> --json` emits a versioned, content-digested manifest. Every item
   states why it is needed, whether it is required, how the host should load it, its actual byte
   footprint estimate, and why any optional item was omitted.
2. The agent/host starts a run using that manifest digest plus handshake/config/palette/template
   digests. Core allocates deterministic run/attempt/span identities under the spec lock.
3. The host records metadata-only spans for context load, model boundary, tool call, edit,
   verify, eval, approval, and dispatch. Large or sensitive bodies are off by default and are
   represented by hashes/local references.
4. Provider-specific adapters may attach token categories, model/provider, latency, exact cost,
   currency, and an external attestation reference. `specd` validates and stores; it never calls
   a model, estimates a bill, or trusts the annotation as task-completion evidence.
5. Verify/eval outcomes remain the completion authority. Observability enriches diagnosis and
   may trigger deterministic resource brakes, but missing telemetry never turns failure into
   success.
6. `report --trace --json`, `report --metrics`, and Prometheus export are pure projections of
   the ledgers. Trace JSON carries high-cardinality correlation; metrics expose bounded aggregate
   labels only.
7. A configured budget can stop *future* dispatch once trusted or policy-accepted accumulated
   cost/tokens/duration crosses a threshold. It cannot undo a completed provider call, and it
   fails closed when production policy requires telemetry that is missing or malformed.
8. Teams compare manifest digest/size, first-pass pass rate, retries, latency, and cost across
   runs, then update steering, skills, tests, or routing outside the deterministic core.

## Recommended action plan

### P0 — Make current claims accurate and safe

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Version a common run/telemetry envelope and state trust semantics explicitly. Preserve legacy `Annotations`. | `internal/core/telemetry.go`, `internal/core/evidence.go`, `internal/orchestration/acp.go`, `docs/observability.md`, `docs/open-spec-format.md` | Old evidence/ACP fixtures decode unchanged; canonical new records round-trip byte-stably; malformed decimals, negative units, cost-without-currency, and unknown required schema versions fail closed. |
| Add correlation fields without changing completion semantics. | New `internal/core/runledger.go`; `internal/cmd/verify.go` or current verify handler in `internal/cmd/registry.go`; `internal/cmd/brain_worker.go`; `.specd/specs/<slug>/runs.jsonl` | Manual and Brain task runs produce one stable run identity with monotonic attempts; the evidence gate passes/fails identically with the ledger absent; racing writers do not duplicate an attempt. |
| Correct context accounting and sufficiency. | `internal/context/manifest.go`, `internal/context/budget.go`, `internal/core/gates/contextbudget.go`, `internal/context/*_test.go` | Estimates cover the bytes the contract tells the host to load; required requirement/design/task/role/declared-file items cannot be silently dropped; an over-budget required set fails with a concise remediation; repeated manifests are byte-identical. |
| Expose actual cost-brake status honestly. Either wire it from accepted telemetry or remove the dormant public implication. | `internal/orchestration/sense.go`, `internal/orchestration/brakes.go`, `internal/cmd/brain_run.go`, `internal/core/commands.go` | A configured threshold halts only subsequent dispatch and records the exact reason; missing required telemetry fails closed; without configured limits behavior matches today. No test may populate `Snapshot.Cost` in a way production cannot. |
| Define privacy and metric-cardinality policy before adding payloads. | `docs/observability.md`, `SECURITY.md`, new `docs/telemetry-schema.md` | Static contract test rejects Prometheus label additions outside an allowlist; default trace fixtures contain no prompt, response, file contents, raw command output, secrets, or absolute home paths. |

### P1 — Add useful traces and exports

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Implement metadata-only run spans and stable JSONL trace rendering. | New `internal/core/runspan.go`, `internal/cmd/report_trace.go`, `internal/core/history.go`, `internal/core/commands.go` | Parent references resolve; span IDs are unique in a run; ordering is stable on equal/missing timestamps; two exports over the same tree are byte-identical. |
| Expand provider-neutral annotations: input/output/cache tokens, provider/model, currency, pricing reference, source, attestation reference. | `internal/core/telemetry.go`, command flag schema in `internal/core/commands.go`, MCP schema tests, docs pair `command-reference.md`/`CHEATSHEET.md` | Optional fields remain optional; exact-decimal aggregation has golden tests; legacy total and new category totals cannot silently contradict each other. |
| Export a versioned provider-neutral local event schema without networking; let an external adapter map it to OpenTelemetry. | New neutral event renderer under `internal/core/`, report format handling in `internal/cmd/registry.go`, adapter contract under `docs/adapters/` | Golden local output validates against the documented schema; core performs no network calls and introduces no module dependency; adapter round-trip fixtures preserve correlation and privacy fields. |
| Add context-efficiency report. | `internal/context`, `internal/core/telemetry.go`, `internal/cmd/report.go` | Given fixed fixtures, report deterministically shows estimated/actual tokens, omitted items, retry count, first-pass result, duration, and cost with explicit `unknown` values rather than zeros. |

### P2 — Operational optimization through optional boundaries

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Define an adapter ingestion contract for provider-attested usage and external trace backends. | `docs/adapters/telemetry.md`, new `internal/core/attestation.go`, optional scripts or separate repositories | Core accepts signed/hashed envelopes from an allowlisted key/config and rejects tampering; all adapter tests run against fixtures with networking disabled. |
| Add model-routing recommendations as policy metadata, not model execution. | `project.yml` config schema in `internal/core/config_*`, task/role packet fields in `internal/context` | Same task/config always yields the same routing class; no gate contacts a provider; an unavailable recommended model remains an adapter concern and cannot bypass evidence. |
| Add cross-spec economic roll-ups and drift alerts. | `internal/core/program.go`, report models/renderers, optional dashboard examples | Aggregates are exact, stable, bounded-cardinality, and clearly separate missing telemetry from zero. Threshold alerts reference source records and never mutate lifecycle state. |

Any CLI flag addition requires synchronized updates to `docs/command-reference.md` and
`docs/CHEATSHEET.md`. Any ledger change requires backward-compatible decoding and explicit
schema-version tests.

## Production validation scenarios

1. **Unannotated manual run:** verify succeeds with no telemetry. Completion works; reports show
   telemetry as absent, not zero, and no budget claim is made.
2. **Provider-annotated run:** input/output/cache tokens, exact cost/currency, model, duration,
   and source survive evidence, trace, metrics, and JSON export without rounding.
3. **Failed retry:** two failures and one pass remain three attempts with one stable task/run
   chain. Cost and latency include all attempts; completion points only to passing HEAD evidence.
4. **Crash during append:** recovery yields either the prior complete record or one complete new
   record, never a silently accepted partial line or duplicate mission/run.
5. **Budget brake:** cost crosses the configured limit after a report. The completed call remains
   recorded; the next dispatch halts with a deterministic reason and no worker lease.
6. **Missing required accounting:** production policy requires provider-attested usage but an
   adapter sends worker-only data. Dispatch/report gate fails closed with one actionable message.
7. **Context under pressure:** optional memory is removed before standing steering; required
   design/task/file context that cannot fit causes dispatch refusal rather than a starved agent.
8. **Context drift:** changing a role, steering file, config, or selected source changes the
   manifest digest; replay against the old digest is rejected or explicitly marked historical.
9. **Privacy:** fixtures containing secrets, user names, absolute paths, prompt text, and command
   output export only allowed metadata/redacted references by default.
10. **Cardinality:** thousands of runs increase JSONL lines but do not create Prometheus series
    by run, mission, SHA, path, actor, or error message.
11. **Offline operation:** every gate, aggregation, report, and export succeeds with networking
    disabled. Only an explicitly invoked adapter may communicate externally.
12. **Backward compatibility:** pre-telemetry and current telemetry ledgers load and report after
    upgrade; downgrade behavior is documented and never rewrites data implicitly.

## Context-safety considerations

- Treat a manifest as an auditable load plan, not proof that the model actually saw every item.
  The host should acknowledge loaded item digests; the harness records the acknowledgement.
- Required context should be task-specific: approved acceptance criteria, relevant design
  decisions, exact task row, role constitution, declared files, and directly needed steering.
  Avoid unconditional repository dumps and avoid loading all historical memory.
- Progressive disclosure must be explicit. Optional memory/skills use `reference-if-needed`;
  their omission is recorded. Standing safety rules cannot be optional or silently budgeted out.
- Separate estimated input footprint, host-reported actual input tokens, and provider-billed
  tokens. They answer different questions and must never be merged into one authoritative value.
- Do not place raw prompts, chain-of-thought, model responses, source contents, or full command
  output in the default run ledger. Use bounded local artifacts and hashes when diagnosis needs
  detail.
- Agent-facing status should be concise: current phase, one frontier task, exact next valid
  commands, blockers, manifest digest, and required evidence. Historical traces belong in
  on-demand reports, not every model turn.

## Non-goals and risks

- **No provider SDK or hosted collector in core.** Optional adapters translate external data;
  `specd` remains stdlib-only and offline-capable.
- **No model calls in gates or reports.** Rubric/eval adapters may produce evidence, but the
  deterministic core only validates declared contracts.
- **No claim that worker telemetry is billing truth.** Trust source and attestation must remain
  visible in every relevant report.
- **No automatic cheapest-model selection in the first implementation.** Bad routing can reduce
  correctness and raise total cost through retries; record and recommend before automating.
- **No high-cardinality Prometheus trace model.** Use JSONL/trace export for correlation.
- **No raw-content telemetry by default.** Rich traces increase privacy, compliance, storage,
  prompt-injection, and secret-leak risk.
- **Risk: observability overload.** More fields can bloat state and model context. Keep run
  ledgers separate, reports on demand, records append-only, payloads bounded, and summaries
  compact.
- **Risk: telemetry-controlled safety.** A malicious or broken worker can misreport cost. Limits
  based on untrusted data must say so; production policy may require an attested adapter.
- **Risk: false context precision.** Byte/token heuristics are planning estimates, not tokenizer
  truth. The schema must label estimates and preserve the provider-reported actual separately.
