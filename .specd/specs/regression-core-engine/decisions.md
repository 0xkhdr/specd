# Decisions — Regression: Core Engine (DAG, gates, state, runner, telemetry)

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-17
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — R2.1 full gate-block (approve/task refusing while awaiting-approval) and R3.2 --unverified bypass are enforced in internal/cmd, not internal/core. Core T4 tests lock the engine primitives those depend on (PhaseReadiness blocking, PlanningAdvance forward-only ratchet, GateEvidence rejection, monotone SaveState revision). End-to-end CLI gate-block regression is owned by the regression-cli-cmd spec (wave 3). · 2026-06-17
**Context:** TODO
**Decision:** R2.1 full gate-block (approve/task refusing while awaiting-approval) and R3.2 --unverified bypass are enforced in internal/cmd, not internal/core. Core T4 tests lock the engine primitives those depend on (PhaseReadiness blocking, PlanningAdvance forward-only ratchet, GateEvidence rejection, monotone SaveState revision). End-to-end CLI gate-block regression is owned by the regression-cli-cmd spec (wave 3).
**Consequences:** TODO
**Supersedes:** —
