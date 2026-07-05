# 0005 Report History & Prometheus Scope

Status: accepted

Context:
Spec 13 adapts v1's report surface. v1 shipped `--history`, `--diff`,
`--format md|html|prometheus`, `--serve` SSE, and `--watch`. FINDINGS verdict:
add exactly `--history` (audit replay) and a Prometheus textfile view; leave the
rest dead.

Decision:
- **`--history` is a pure merge-sort over existing records.** It reads
  `state.json` (approvals, decisions, mid-req notes), the evidence ledger
  (verify attempts + completions), the criterion and submission ledgers, and the
  ACP ledger. It introduces no new event store and writes nothing on replay
  (spec 13 R2).
- **Evidence records gained a `timestamp`/`actor` stamp.** They previously
  carried neither, so verify attempts could not be ordered against approvals.
  The fields are `omitempty` and stamped centrally in `AppendEvidence`; records
  written before this change decode as valid and sort by append order. This is
  the same stamping discipline every other ledger already used — not a new store.
- **Completions reuse the passing verify record.** A completion event is
  projected for each task currently marked complete, dated by its passing
  evidence record; there is no separate completion store.
- **Tie-break is (timestamp, source-rank, sequence).** Source-rank is a fixed
  per-source ordering and sequence is the record's position within its source,
  so the pair is unique and repeated `--history` runs are byte-identical
  (spec 13 R3).
- **Prometheus metric names are treated as API.** Names are `specd_`-prefixed,
  snake_case, `_total`/`_seconds` suffixed, and pinned in a table in
  `docs/command-reference.md`. Validity (names, unique series, escaped labels)
  is enforced by an internal promtool-style lint in tests, not by shipping
  promtool (spec 13 R5).
- **Escalation metrics degrade gracefully.** `specd_escalated_tasks` is a
  well-formed zero until spec 06 lands the escalation source; no placeholder
  spam beyond that natural zero-valued gauge.

Not brought back (recorded non-goals): `--serve`, `--watch`, SSE, HTML.
Deferred, revisit on demand via a new spec: `--diff` between git refs, and
cross-spec/program aggregation in `report` (program aggregation belongs to spec
12's `status --program`).

Consequences:
`report` stays a pure read of on-disk state. New event sources (e.g. spec 06
escalations/overrides) join `--history` by adding a reader, not by changing the
model.
