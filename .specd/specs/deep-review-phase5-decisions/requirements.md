# Requirements — deep-review-phase5-decisions

> Source: DEEP-REVIEW.md §2 findings #4, #9; §3.2; §4 Phase 5. Owner calls, recorded — not code-first.

## R1 — Ledger-verb consolidation decision

- owner: 0xkhdr
- priority: must
- risk: medium

- R1.1: When the ledger-verb usage audit completes, the system shall hold a recorded decision (via `specd decision record`) that either consolidates the thin append-a-record verbs (`incident`, `recurring`, `release`, `deploy`, `spike`, `midreq`, `drift`, `eval`, `exception`, `memory`, `link`, `unlink`, `decision`, `handshake`) under one `specd record <kind>` verb, or defers the ones without a real user with a dated deferral note.
- edge: If a verb is found to have a real external consumer, the system shall keep it and name the consumer in the decision record.

## R2 — One metrics format

- owner: 0xkhdr
- priority: should
- risk: low

- R2.1: When the observability decision is recorded, the system shall name exactly one export format (Prometheus text or OTel JSON) as the kept surface, with the other scheduled for deletion in a follow-up spec.
- R2.2: When the kept format is chosen, the system shall document it in `docs/observability.md`.

## R3 — Contract docs cite their live driver

- owner: 0xkhdr
- priority: should
- risk: low

- R3.1: When a contract doc (`docs/adapter-contract.md`, `delivery-contract.md`, `operating-model-contract.md`, `telemetry-schema.md`, `scale-envelope.md`, `data-classification.md`) is audited, the system shall show in each doc either the verb/test that proves the contract live, or an explicit historical/aspirational marker.

## Non-goals

- No code changes in this spec; consolidation and exporter deletion land as follow-up specs after the decisions are recorded.
