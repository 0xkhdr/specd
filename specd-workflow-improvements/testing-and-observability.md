# Testing and observability

## Domain definition

Owns regression strategy, cross-component contract tests, production journeys, event/history
visibility, gate input provenance, and local workflow metrics.

## Current behavior

Repository has broad unit, race, lint, regression, and smoke coverage. Yet production smoke has
historically downgraded profile, fields existed only in tests, and parser/guidance/dispatch pairs were
not always exercised together. Reports derive from state and ledgers.

## Evidence from feedback

- [Three production deadlocks survived green CI](../WORKFLOW-FEEDBACK.md#2026-07-21--improvement--three-structural-deadlocks-in-one-afternoon-all-under-a-green-ci-the-production-lane-is-proved-by-not-being-run).
- [Gate result changed with no observable input change](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--evidence-policy-blocker-cleared-with-no-observable-state-change).
- [Check JSON empty while guide had blocker](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--improvement--specd-check---json-returns--while-status---guide---json-lists-a-blocker).

## Main problems

Tests validate components but not shared production routes. Fixture-only fields create dead
contracts. Gate results lack input digests. Silent success and exit 0 halts mislead automation.

## Root-cause analysis

Coverage is organized by package/feature, while failures occur at boundaries: template-parser,
parser-gate, guide-dispatch, authority-issuer, controller-worker, and profile-smoke.

## Desired behavior

Each declared workflow has one end-to-end executable proof. Every cross-surface contract has parity
tests. Gate plans expose inputs/result digest. History and local metrics reveal retries, delegated
approvals, reopen cycles, waits, and deprecated usage without network telemetry.

## Recommended design

Test layers:

1. Pure state/transition matrices.
2. Contract tests: scaffold-consumer, one-parser-per-field, help-handler, guidance-dispatch,
   authority-issuer-consumer, event-projection.
3. Black-box journeys: general request, consult, default managed, production managed, task reopen,
   spec reopen, delegated unattended, migration, crash recovery.
4. Lints: persisted field assigned only in tests; direct splitting of typed task fields; declared flag
   unused; production smoke profile mutation; recovery command absent from palette.
5. Fault injection: CAS, atomic writes, checkpoint/ledger, migration, grant consumption.

Gate result includes plan id, armed gates, each input path/digest, config digest, state revision,
result digest, and duration. Do not record raw source or secrets. `check` success prints summary.

Local metrics in reports: transition attempts/refusals by code, time/waits by stage, reopen counts,
stale descendants, delegated approvals, retries, zero-progress halts, legacy config/state use. No
daemon or outbound reporting.

## Workflow implications

CI proves actual supported postures. Operators can explain pass/fail flips. Regressions become
feedback-linked tests rather than prose backlog.

## Data-model implications

Plan/result digests and event timing; optional local aggregate projection derived from history, not a
second mutable metrics store.

## CLI implications

`check --json` plan envelope, `report --workflow-metrics`, `doctor --compat`, stable refusal codes.

## Coding-agent implications

Agent can cite exact plan/input digest and smallest reproducer. It does not infer success from empty
output.

## Compatibility implications

Additive report fields first. Exit changes documented. Existing CI commands stay, but misleading
production profile downgrade is removed only when replacement route works.

## Failure scenarios

Nondeterministic map order fails count=2/golden; missing input digest fails plan lint; recovery command
drifts and parity test fails; production journey blocked intentionally must assert exact boundary.

## Edge cases

No Git HEAD, warnings-only plan, old state/config, host advisory, multiple blockers, concurrent grant
use, torn last ledger line.

## Testing strategy

This document defines it; prioritize boundary contracts and journeys over more per-function tests.
Run race, repeated-count, lint, docs parity, and regression scripts as final gate.

## Implementation recommendations

Create feedback index first. Delete tests that duplicate lower-value implementation details when
journey/contract coverage supersedes them.

## Trade-offs

End-to-end tests cost runtime, but one per supported posture is cheaper than diagnosing structural
deadlocks. Local metrics add event fields but no operational service.

## Risks

Golden outputs can become churn-heavy. Pin schemas/semantics, not incidental whitespace, except
byte-stability contracts.

## Acceptance criteria

- Production journey never changes profile.
- Every typed field has one parser and cross-consumer test.
- Every advertised action passes dispatch parity.
- Gate output identifies input/config/state digests.
- Feedback entries map to regression/disposition.

## Open questions

- CI runtime budget for production and crash journeys.
- Which workflow metrics merit stable public schema.

