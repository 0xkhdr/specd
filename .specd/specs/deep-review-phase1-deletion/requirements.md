# Requirements — deep-review-phase1-deletion

> Source: DEEP-REVIEW.md §2 findings #1, #2, #5; §3.3; §4 Phase 1.

## R1 — Remove dead A2A orchestration layer

- owner: 0xkhdr
- priority: must
- risk: low

- R1.1: When the A2A envelope code (`internal/orchestration/a2a.go` and its tests) is removed, the system shall build and pass the full test suite with no production behavior change.
- R1.2: When the A2A contract is retained as a planned future surface, the system shall record a dated deferral decision instead of keeping the unreferenced code.
- edge: If a hidden caller of the A2A symbols exists, the system shall fail the build or tests before the deletion commit lands.

## R2 — Remove unreachable adapter machinery

- owner: 0xkhdr
- priority: must
- risk: medium

- R2.1: When the adapter symbols with zero production callers (`adapter.Run`, `NegotiateCapabilities`, `MatchIdentity`, `Historical`, `ValidateFeedbackCommit`, `FeedbackRequest`, `FeedbackFromRequest`, `MissionRequest`) are removed, the system shall retain the symbols production uses today (`Adapter`, `Request`, `SchemaVersion`, `MissionFromRequest`, `ExportOTel`) and pass the full test suite.
- R2.2: When any removed adapter surface is kept for a planned verb, the system shall carry an issue naming the driving verb and a deadline.
- edge: If a surviving adapter symbol depends on a deleted one, the system shall fail compilation rather than keep a partial surface.

## R3 — Cut the triage deferred verb

- owner: 0xkhdr
- priority: must
- risk: low

- R3.1: When `triage` is removed from the palette, dispatch, docs, and tests, the system shall fail closed with exit code 2 on `specd triage` like any unknown verb.

## R4 — Repo hygiene

- owner: 0xkhdr
- priority: should
- risk: low

- R4.1: When the stray `coverage.out` artifact is deleted from the repo root, the system shall keep local coverage runs targeting a scratch directory so the artifact shall not reappear at root.

## Non-goals

- No behavior change to any reachable verb.
- No ledger-verb consolidation (deferred to deep-review-phase5-decisions).
- No docs restructure beyond removing rows for deleted surfaces.
