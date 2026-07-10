# Design — Deterministic observability and operational economics

## Decision

Turn the audit projection into a correlated run/span model without touching completion authority.
Core allocates a deterministic run/attempt/span identity under the spec lock and appends to
`runs.jsonl`; verify/eval evidence stays the only thing that completes a task. Every measured value
gets a versioned envelope and a `telemetry_source` trust label, so a worker-reported number is never
rendered as billing truth. Context accounting is corrected to estimate the host's real payload and
to refuse dispatch when a required item cannot fit. The dormant cost brake is either wired from
accepted telemetry or removed. Privacy and cardinality become a static contract before any payload
is added. Traces, metrics, a neutral event schema, and economic roll-ups are pure offline
projections; OTel and provider-attested usage arrive only through validated adapter artifacts.

```text
context build → estimate host payload + required-set check → manifest_digest ──┐
        ↓ (refuse if required over budget)                                     │
run/attempt/span identity (spec lock) → runs.jsonl ───────────────────────────┼─> report --trace (JSONL, hi-card)
        ↓                                                                      │
verify/eval evidence (completion authority, pinned HEAD) ──────────────────────┼─> report --metrics (allowlist labels)
        ↓                                                                      │
telemetry envelope {source, tokens{in/out/cache}, cost/currency/pricing} ──────┼─> neutral event schema → [adapter → OTel]
        ↓                                                                      │
accepted trusted telemetry → Sense.Cost → brake (halt future dispatch) ────────┘
        ↓
cross-spec roll-up + drift alert (references source records, mutates nothing)
```

Missing telemetry never turns failure into success and never blocks a valid completion. A brake on
untrusted data says so; production policy may require an attested adapter.

## Run/span identity and ledger

New `internal/core/runledger.go` allocates identity under `WithSpecLock`:

```text
RunV1
  run_id (deterministic, stable through retry/recovery), spec_id, task_id,
  attempt (monotonic per task/run), started_at, git_head, actor, worker_id,
  telemetry_source, envelope_version
```

`run_id` is derived from spec/task/baseline, never from a provider trace ID. `runs.jsonl` is
append-only and additive: the evidence gate passes or fails identically with the ledger absent.
Manual runs (verify handler in `internal/cmd/registry.go`) and Brain runs
(`internal/cmd/brain_worker.go`) share one allocator. Crash safety reuses the existing atomic
append + checkpoint replay so a torn line yields one complete record or the prior one.

## Telemetry envelope and trust

`internal/core/telemetry.go` gains a versioned wrapper preserving legacy `Annotations`:

```text
TelemetryV1
  source (worker|provider_adapter|operator), envelope_version,
  input_tokens?, output_tokens?, cached_tokens?, tokens_total? (legacy),
  cost? (exact decimal), currency?, pricing_ref?,
  provider?, model?, duration_ms? (with source label), attestation_ref?
```

Decode rules: unknown *required* schema version fails closed; unknown optional field is retained;
cost present without currency fails; negative unit or malformed decimal fails; absent stays absent,
never zero. Legacy total and category totals are validated to never silently contradict; exact
rational aggregation keeps golden tests.

## Context accounting and sufficiency

`internal/context/manifest.go` and `budget.go` stop estimating the four core items from
kind/path/task-ID strings. Estimate covers the bytes the contract tells the host to load, adds
`design.md` and the task's declared source files, and separates three quantities:

```text
ContextAccountingV1
  estimated_input_tokens (planning estimate, labelled),
  host_reported_input_tokens? (from load ack),
  provider_billed_tokens? (from adapter)
  context_manifest_digest (manifest + effective config + role + steering versions)
  supplied_items[], omitted_items[]{ref, reason}
```

`internal/core/gates/contextbudget.go` refuses dispatch when the required set exceeds budget with a
concise remediation; optional memory/skills use `reference-if-needed` and their omission is
recorded; standing safety rules cannot be budgeted out. The host acknowledges loaded item digests;
the harness records the ack — a passing gate is a load plan, not proof of what the model saw.

## Honest cost brake

`internal/orchestration/sense.go` populates `Snapshot.Cost` only from accepted trusted telemetry;
`brakes.go` `EvaluateBrakes` halts subsequent dispatch and records the exact reason; `brain_run.go`
and `internal/core/commands.go` expose the threshold config. If wiring is deferred, the dormant
public implication is removed so no operator infers enforcement from types/tests. A brake based on
untrusted worker data is labelled as such; missing required telemetry under production fails closed.
No completed provider call is undone; no worker lease is taken on halt.

## Privacy and cardinality contract

Before any payload lands, a static contract test pins the Prometheus label allowlist
(`spec`, task/status only) and rejects additions. Default trace/run fixtures are metadata-only: no
prompt, response, chain-of-thought, file content, raw command output, secret, or absolute home path.
`evidence_ref` is workspace-relative or content-addressed; traversal and URLs are rejected in the
core schema. Central redaction (Domain 06) applies before display/export. Policy is documented in
new `docs/telemetry-schema.md` plus `docs/observability.md` and `SECURITY.md`.

## Spans and trace export

New `internal/core/runspan.go` records metadata-only spans:

```text
SpanV1
  span_id (unique in run), parent_span_id?, run_id, kind (closed enum + extension),
  started_at?, ended_at?, duration_ms?, git_head? (required for code-effect/completion),
  status, error_class?, evidence_ref?
kind ∈ {context_load, model_boundary, tool_call, edit, verify, eval, approval, dispatch}
```

`internal/cmd/report_trace.go` renders stable JSONL: parent references resolve, IDs unique,
ordering stable on equal/missing timestamps, two exports byte-identical. `history.go` remains the
audit projection; the trace adds hierarchy on top of the same ledgers. No gate reads wall-clock
ordering for an outcome.

## Neutral event schema, adapter, and efficiency report

A neutral event renderer under `internal/core/` emits a versioned provider-neutral schema validated
by golden output; an external adapter under `docs/adapters/telemetry.md` maps it to OpenTelemetry,
with round-trip fixtures preserving correlation and privacy fields and networking disabled. The
context-efficiency report (`internal/cmd/report.go` + `internal/context`) deterministically shows
estimated/actual tokens, omitted items, retry count, first-pass result, duration, and cost, using
explicit `unknown` rather than zero.

## Attested ingestion, routing, roll-ups

New `internal/core/attestation.go` accepts signed/hashed provider-attested envelopes from an
allowlisted key/config and rejects tampering (adapter tests offline). Routing recommendations are
policy metadata in `project.yml` and role/task packet fields (`internal/context`): same task/config
→ same routing class, no provider contact, unavailable recommended model cannot bypass evidence.
`internal/core/program.go` adds cross-spec exact roll-ups and drift alerts that reference source
records and mutate no lifecycle state, separating missing telemetry from zero.

## Verification ladder

1. Unit/golden: envelope round-trip + legacy decode; run/attempt allocation; span parse/order;
   exact-decimal aggregation; context estimate vs. host payload; manifest digest stability;
   neutral event schema validation; redaction.
2. Fail-closed: unknown required schema version; cost without currency; negative units; over-budget
   required context; missing required attested telemetry under production; label outside allowlist.
3. Black-box fixtures: unannotated run; provider-annotated run; failed retry chain; crash mid-append;
   budget brake after report; missing attested accounting; context under pressure; context drift;
   secret/PII/path privacy; thousand-run cardinality; offline operation; backward compatibility.
4. Conformance: adapter round-trip preserves correlation/privacy; trace/metrics/event exports agree
   on the same ledgers; downgrade documented and never rewrites data implicitly.
5. Full race/vet/lint/regression after integrated domain; docs mirror
   (`command-reference.md`/`CHEATSHEET.md`); `go mod tidy` clean.
