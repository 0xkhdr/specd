# Domain 07 — Observability, cost, and operational economics

## Goal

Make production observability a deterministic, offline, replayable contract, not an agent
narrative. Correlated run/span trajectories, honest trust labels on every measurement,
latency/token/cost attribution at the provider boundary, provable context sufficiency and budget,
and feedback into policy — all as pure projections of on-disk `.specd/` ledgers. No LLM in any
gate, aggregation, or report path. No phone-home in core. Stdlib-only; external trace backends and
provider-attested usage covered by an adapter/attestation contract, not by importing OTel or
provider SDKs. `specd` exposes facts; humans and optional policy adapters optimize them.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/07-observability-cost-and-operational-economics.md`.
Paper position: observability is a first-class harness component alongside instructions, tools,
sandboxes, orchestration, and guardrails — logs, traces, evals, cost, latency
(`sdlc-paper.md:258-317`). Economics is operational: uncontrolled context and correction loops burn
tokens; dense task-specific context is a financial lever; tests/evals/root-cause/regression form a
quality flywheel (`sdlc-paper.md:220-226`); routing reserves expensive models for high-complexity
work (`sdlc-paper.md:420-458`).

Current state: append-only verify-attempt ledger (`evidence.jsonl`), worker-reported optional
`--tokens`/`--cost`/`--duration-ms` (validated, exact rational cost, absence explicit), ACP
trajectory fragments, deterministic `report --history` audit projection, `report --metrics` and
Prometheus textfile export, bounded context manifest with gateable `context.max_tokens`, scale
checks, crash/recovery observability, no-phone-home baseline, and a dormant cost-brake skeleton.
Gaps: history is an audit projection not a full run/span trace; `tokens` conflates
input/output/cache and `cost` lacks currency/provider/model/pricing source; the cost brake is
untested in production (`Sense` leaves cost unset); context budget underestimates the host's real
payload and does not prove sufficiency; context quality is not related to outcome economics; export
cardinality/privacy could regress the no-phone-home posture; provider/model routing is outside the
observable contract.

## Ownership

| Area | Domain 07 owns | Other domain owns |
|---|---|---|
| Run/span identity | deterministic run/attempt/span IDs under spec lock, `runs.jsonl` ledger | Domain 05 mission/lease/ACP trajectory transport |
| Telemetry envelope | versioned envelope, trust source label, token categories, exact-decimal cost | Domain 04 verify/evidence completion authority |
| Context accounting | estimate-vs-actual footprint, required-item sufficiency, manifest digest | Domain 02 context selection/manifest content |
| Cost brake | wiring accepted telemetry → `Sense`/brake, threshold halt of future dispatch | Domain 05 dispatch loop, Domain 01 approve |
| Export projections | `report --trace`/`--metrics`/`--format`, cardinality-bounded labels, neutral event schema | Domain 10 external adapter/OTel transport |
| Privacy/redaction | default metadata-only ledgers, bounded refs | Domain 06 secret/redaction policy, policy digest |
| Economic roll-ups | cross-spec exact aggregates, drift alerts referencing source records | Domain 01 program identity |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `07a-observability-contract-baseline` | observed behavior, corrected docs wording, failing fixtures for every P0 gap | — |
| W1 | `07b-versioned-run-telemetry-envelope` | versioned envelope, explicit trust semantics, legacy JSONL decodes unchanged | 07a |
| W2 | `07c-run-correlation-and-attempt-identity` | deterministic run/attempt identity, `runs.jsonl`, completion semantics unchanged | 07a |
| W3 | `07d-context-accounting-and-sufficiency` | estimate covers host payload, required items cannot be dropped, over-budget fails with remediation | 07a, Domain 02 context |
| W4 | `07e-honest-cost-brake` | wire brake from accepted telemetry or remove dormant public implication; fail-closed on missing required telemetry | 07b, Domain 05 dispatch |
| W5 | `07f-privacy-and-cardinality-policy` | label allowlist contract test, metadata-only default fixtures, `telemetry-schema.md` | 07b, Domain 06 redaction |
| W6 | `07g-metadata-run-spans-and-trace-export` | metadata-only spans, stable JSONL `report --trace`, byte-identical replays | 07b,07c,07f |
| W7 | `07h-provider-neutral-annotation-expansion` | input/output/cache tokens, provider/model, currency, pricing ref, source, attestation ref | 07b,07f |
| W8 | `07i-neutral-event-schema-and-context-efficiency` | versioned offline event schema + adapter contract; context-efficiency report | 07d,07g,07h, Domain 10 adapter |
| W9 | `07j-attested-ingestion-routing-and-rollups` | attested-usage ingestion, routing recommendation metadata, cross-spec roll-ups + drift, release proof | 07e,07h,07i, Domain 10 adapter |

## DAG

```text
07a ─┬─> 07b ─┬─> 07e ─────────────────────────────┐
     │        ├─> 07f ─┬─> 07g ─┐                   │
     │        └────────┴─> 07h ─┼─> 07i ─> 07j
     ├─> 07c ──────────> 07g ───┘         ↑    ↑
     └─> 07d ──────────────────> 07i      │    │
Domain 02 context ─> 07d                  │    │
Domain 05 dispatch ─> 07e                 │    │
Domain 06 redaction ─> 07f                │    │
Domain 10 adapter ─> 07i ─────────────────┘    │
Domain 10 adapter ─> 07j ──────────────────────┘
```

## Program rules

1. No LLM, network call, or estimation-as-truth in any gate, aggregation, report, or export.
   Everything is a pure function of `.specd/` ledgers.
2. Verify/eval outcomes stay the sole completion authority. Missing, malformed, or absent telemetry
   never turns failure into success and never blocks a valid deterministic completion.
3. Trust source is always visible. Worker-reported measurement is an accounting hint, never billing
   truth; provider-attested usage arrives only through a validated adapter envelope.
4. Separate three quantities forever: estimated input footprint, host-reported actual input tokens,
   provider-billed tokens. Never merge them into one authoritative value.
5. Additive, versioned schema. Old evidence/ACP/telemetry records decode unchanged; new canonical
   records round-trip byte-stably; unknown required schema version fails closed.
6. Trace JSONL carries high-cardinality correlation; Prometheus/metrics expose only allowlisted
   bounded labels. No run/mission/SHA/path/actor/error-text label may enter metrics.
7. Metadata-only by default. No raw prompt, response, chain-of-thought, source content, or full
   command output in the run ledger; bounded local artifacts and hashes carry detail on demand.
8. Cost/currency stays exact decimal; `specd` never prices tokens or calls a model. A configured
   budget halts only *future* dispatch and fails closed when required telemetry is missing.
9. Stdlib-only, offline core. External trace/attestation adapters produce pinned artifacts the
   deterministic core validates. No `reference/` edits.

## Completion claim

Fresh fixtures prove: an unannotated manual run completes and reports telemetry as absent (not
zero); a provider-annotated run's input/output/cache tokens, exact cost/currency, model, and
duration survive evidence/trace/metrics/JSON without rounding; two failures plus one pass stay three
attempts on one stable run chain with completion pinned only to the passing HEAD; a crash mid-append
yields one complete record, never a partial or duplicate run; a configured budget halts the next
dispatch with a deterministic reason after a recorded call, undoing nothing; production policy
requiring attested usage fails closed when an adapter sends worker-only data; a required
design/task/file context that cannot fit refuses dispatch instead of starving the agent; changing a
role/steering/config/source changes the manifest digest and replay against the old digest is
rejected or marked historical; fixtures with secrets/usernames/absolute paths/prompt text export
only allowed metadata; thousands of runs add JSONL lines but create no Prometheus series by
run/mission/SHA/path/actor/error; every gate/aggregation/report/export succeeds with networking
disabled; pre-telemetry and current ledgers load and report after upgrade. Docs never present a
reported value as independently measured, or claim an export boundary the core does not enforce.
