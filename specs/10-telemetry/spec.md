# 10-telemetry — Cost/duration annotations on task and ACP records

Wave 2. FINDINGS refs: B.15, B.20 (evidence-rigor half), D-tier2 item 12.

## Problem

v1 stored worker cost telemetry (`task --tokens --cost`, pinky
`--host-tokens --host-cost --duration-ms`) — stored, never computed. The
current version has none, so `report --metrics` can never say anything
about cost. Separately (B.20), current ACP worker reports lack v1's rigor:
no attempt numbers, no git-HEAD-stamped claim/report records. FINDINGS
verdicts: **port** the telemetry annotations (tiny — fields on existing
records, aligns with "countable facts only" doctrine); **adapt** ACP
records to carry attempt/HEAD rigor.

## Requirements (EARS)

- R1: WHEN a task is completed (or its verify recorded), THE SYSTEM SHALL
  accept optional annotations `--tokens <int> --cost <decimal>
  --duration-ms <int>` and store them verbatim on the record; THE SYSTEM
  SHALL never compute, estimate, or derive these values.
- R2: WHEN an annotation value is malformed (non-integer tokens/duration,
  non-decimal cost, negative anything), THE SYSTEM SHALL fail closed
  (exit 2) without writing.
- R3: ACP ledger records for claim/report SHALL carry: attempt number
  (monotonic per task), git HEAD at record time, changed-files list (as
  reported), verification record reference, and the R1 annotations when
  supplied.
- R4: WHEN a user runs `report --metrics`, THE SYSTEM SHALL aggregate the
  stored annotations per spec and per task (totals + per-attempt
  breakdown), clearly marking tasks with no telemetry as such — absence of
  data is shown, never imputed.
- R5: Records without annotations SHALL remain fully valid — telemetry is
  always optional (workers that cannot report cost still function).
- R6: Cost SHALL be stored as a decimal string with currency-agnostic
  semantics (no float arithmetic on money in aggregation — integer/decimal
  string math only).

## Design notes / best practice

- "Stored, never computed" is the doctrine line — it keeps the harness
  deterministic and honest. Enforce in review: any PR computing cost inside
  specd violates this spec.
- Decimal handling (R6): store as string, aggregate via `math/big.Rat` or
  scaled integers (stdlib only). Test with values that break float64
  (e.g. 0.1+0.2 accumulation over many records).
- Attempt numbers (R3): derive next attempt = count of existing
  claim-records for the task + 1 under the spec lock — countable fact, not
  a stored counter that can skew. Feeds spec 06's escalation counting.
- Schema: extending record shapes may touch state.json → follow spec 02
  migration discipline (bump, migrate, fixture).
- Pinky agent docs (`.claude/agents/pinky-*.md`) gain the reporting flags in
  their mission templates so host workers actually pass them.
- Forward hook: cost-over-budget escalation (v1 rule) becomes possible once
  this lands — note in spec 06's code comment, do not implement here.

## Out of scope

- Cost budgets/enforcement, trend analytics, `eval` machinery.
- Any token counting inside specd.

## Acceptance

- Complete a task with `--tokens 1200 --cost 0.034 --duration-ms 45000`;
  record holds verbatim strings; `report --metrics` aggregates exactly;
  no-telemetry tasks shown as missing; malformed values exit 2; ACP
  claim/report records carry attempt + HEAD; suite green with fixtures
  migrated.
