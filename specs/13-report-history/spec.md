# 13-report-history — Audit replay and Prometheus textfile output

Wave 2. FINDINGS refs: B.16, D-tier2 item 15.

## Problem

Current `report` has `--pr`/`--metrics`/`--json` only. v1's report surface
included `--history` (audit replay), `--diff`, `--format
md|html|prometheus`, `--serve` SSE, `--watch`. FINDINGS verdict: **adapt**
— worth adding exactly two pieces: `--history` (replay from ledgers that
already exist) and a Prometheus textfile view (cheap, pure function of
state). HTML/serve/SSE explicitly stay dead ("what NOT to bring back").

## Requirements (EARS)

- R1: WHEN a user runs `specd report --history`, THE SYSTEM SHALL replay
  the spec's recorded events in timestamp order — phase approvals,
  decisions, verify attempts (pass/fail, git HEAD), completions,
  escalations/overrides (spec 06), submissions (spec 08), ACP
  claims/reports — each line: timestamp, actor where recorded, event,
  reference.
- R2: THE history SHALL be derived purely from existing on-disk records
  (state.json, evidence store, ledgers); THE SYSTEM SHALL NOT introduce a
  new event store or write anything during `--history`.
- R3: WHEN records carry no ordering-safe timestamp collision resolution,
  ties SHALL break deterministically (documented rule, e.g. record-type
  then id) so repeated runs emit byte-identical output.
- R4: WHEN a user runs `specd report --format prometheus`, THE SYSTEM SHALL
  emit Prometheus textfile-exposition metrics (gauges/counters with HELP
  and TYPE lines): tasks by status, verify attempts/failures, escalated
  task count, criteria coverage (spec 04), telemetry totals where stored
  (spec 10), per spec, labeled `{spec="<slug>"}`.
- R5: `--format prometheus` output SHALL pass promtool-style lint rules
  (valid metric names, no duplicate series, escaped label values) —
  validated by internal tests, not by shipping promtool.
- R6: `--history --json` SHALL emit the same events as machine-readable
  JSON lines.

## Design notes / best practice

- Pure-read invariant (R2) is the whole design: report paths are already
  "generated from state.json + task artifacts" per project rules — history
  is a merge-sort over record sources, nothing more.
- Metric naming: `specd_` prefix, snake_case, unit suffixes
  (`_total`, `_seconds`) per Prometheus naming conventions; write the
  contract in a table in docs so names never churn (renaming a metric
  breaks dashboards — treat names as API).
- Textfile target: output suitable for node_exporter textfile collector
  (write-to-file left to operator/cron; specd only prints — no serve
  machinery, per non-goals).
- Determinism tests: run `--history` twice, byte-compare; fixture with
  equal timestamps exercises the tie-break rule.
- Graceful degradation: events from features not yet adopted (no
  escalations, no submissions) simply absent — no placeholders, no zero
  spam beyond well-formed zero-valued gauges where a total is natural.

## Out of scope

- `--serve`, `--watch`, SSE, HTML (recorded non-goals).
- `--diff` between git refs (revisit on demand; ADR).
- Cross-spec/program aggregation (single spec per invocation; program view
  belongs to spec 12's status).

## Acceptance

- Demo spec with fails, override, completion, submission: `--history`
  shows ordered replay, byte-identical across runs; `--format prometheus`
  parses clean under the internal lint and shows correct task/verify
  counts; `--history --json` line-parses. Full suite green.
