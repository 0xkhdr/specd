# V4 — Model Routing Policy + Token Economics

## 1. Purpose and requirement coverage

Router as **policy, not client**: deterministic tier assignment stamped into
mission briefs and state, budget enforcement via the cost brake, and a
CapEx/OpEx cost report. specd never opens a socket to a model — model *names*
are host concerns, tiers are harness concerns. Covers plan tasks **P1.5** (P0)
and **P1.6** (P1); architecture §5.3.

## 2. Verified current state

- Cost machinery: `internal/core/cost_brake.go`, `telemetry.go`,
  `report_metrics.go`, `telemetry_rollup_test.go`.
- Mission briefs: `internal/core/pinky_brief.go`; complexity signals:
  `internal/core/mode_recommend.go`.
- Reports/PR summary: `internal/core/report.go`, `prsummary.go`.
- Pinky telemetry already records tokens/cost/duration as metadata.

## 3. Proposed design and end-to-end flow

`.specd/config.json` gains a `routing` block (plan §5.3): named `tiers` with
`maxCostUSD`, ordered `rules` with `match` predicates over countable task
facts (role, complexity score reusing `mode_recommend` signals, file count,
retry count) and a mandatory `default` rule. Evaluation is first-match,
deterministic. Output stamped into the mission brief (tier + budget so any
host can honor it) and `state.json.routing` per task (V1 block).

Budget: `cost_brake.go` extended with per-tier budgets; `--budget` on
`brain start` maps to a spec-level cap; breach → brake + escalation record
(consumed by V7). Economics: `specd report <spec> --cost` renders per-task,
per-wave, per-tier spend from recorded telemetry; CapEx (spec-authoring
sessions) vs OpEx (execution sessions) split derived from the phase at
telemetry-record time. PR summary gains a cost section (finished in V7's
submit work).

## 4. Interfaces, contracts, data, configuration, dependencies

- **Config:** `routing` block in `.specd/config.json`; unknown fields rejected
  with context; missing block = no routing (compat).
- **Stable:** brief schema additive (`tier`, `budgetUSD` fields versioned);
  no network code added (invariant 2).
- **Dependencies:** V1 (`state.json.routing`). **Dependents:** V7 (escalation
  on budget breach, handoff tiers), V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Determinism: rule evaluation pure over countable facts; table-tested.
- Reported model/cost is *claimed* by the host — specd records and enforces
  budget, never verifies model identity (recorded as claimed, invariant 6).
- Cost report renders only from telemetry on disk; totals must reconcile with
  the telemetry rollup.
- Compat: repos without a routing block behave exactly as v0.1.x.
- **Rollback:** delete routing block; brake reverts to existing behavior.

## 6. Acceptance criteria and validation commands

- Rule-evaluation table tests (first-match, default, unknown-field rejection).
- Brief includes tier + budget; brief schema round-trip test.
- Budget breach e2e: reported spend over tier budget → brake + escalation
  record written.
- `--cost` report deterministic from fixtures; totals reconcile
  (`telemetry_rollup_test.go` extended).
- `go test ./internal/core/... -run 'Rout|Cost|Telemetry' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1 (`internal/core/routing.go`).
- Open: complexity score scale shared with `mode_recommend` — reuse its raw
  signal integers rather than defining a second scale (single source of
  countable truth).
