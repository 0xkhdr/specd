# Design — deep-review-phase5-decisions

- references: R1, R1.1, R2, R2.1, R2.2, R3, R3.1
disposition: accepted
owner: 0xkhdr

## Boundaries

- Owned: the three decision records (verb consolidation, metrics format, contract-doc dispositions) and the historical/driver markers inside the six contract docs.
- Excluded: implementing the consolidation or exporter deletion — those become follow-up specs traced to these decisions.

## Interfaces

- Decisions land in this spec's ledger via `specd decision record deep-review-phase5-decisions --text …`.
- Contract docs gain a one-line `Driver:` (verb/test citation) or `Status: historical` header.

## Invariants

- Subtractive bias: anything without a named user or driver is deferred/marked, never silently kept.
- No LLM in any decision path — the audit is a human/owner call recorded deterministically.

## Failure

- A decision made but unrecorded is the failure mode; each task's acceptance requires the ledger entry or doc marker to exist on disk.

## Integration

- Follow-up specs (verb consolidation implementation, exporter deletion) must cite these decision records as their source.

## Alternatives

- Deciding inside code PRs without records — rejected: violates the repo's own record-the-decision rule.
- Auditing contract docs by deleting them outright — rejected: some may name planned surfaces; marking keeps the history honest.

## Verification

- Ledger entries present for R1.1 and R2.1; grep over `docs/*-contract.md` and schema/envelope/classification docs shows a Driver citation or historical marker in each.

## Deployment

- Records only; no binary or CI change.

## Rollback

- Decisions are append-only records; a reversal is a new decision record, not a deletion.
