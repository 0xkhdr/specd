# V4 Tasks — Model Routing + Token Economics

Plan coverage: P1.5, P1.6. Dependencies: V1. Dependents: V7, V12.

## Wave 1 — Policy engine

- [ ] `internal/core/routing.go`: config block parse/validate (tiers, ordered
  rules, mandatory default, unknown fields rejected with context).
- [ ] First-match evaluation over countable facts (role, complexity via
  `mode_recommend` signals, file count, retry count); table-driven tests.
- **Validation:** `go test ./internal/core/... -run Rout -race -count=1`

## Wave 2 — Stamping + budget (depends on Wave 1)

- [ ] Stamp tier + budget into mission briefs (`pinky_brief.go`, versioned
  additive fields) and `state.json.routing` per task.
- [ ] Extend `cost_brake.go`: per-tier budgets; `brain start --budget` →
  spec-level cap; breach → brake + escalation record (shape shared with V7).
- **Validation:** `go test ./internal/core/... -run 'Brief|Brake' -race`

## Wave 3 — Economics report (depends on Wave 2)

- [ ] `specd report <spec> --cost`: per-task/per-wave/per-tier spend; CapEx vs
  OpEx split from phase-at-record-time (`report_metrics.go`).
- [ ] Reconciliation test: report totals == telemetry rollup (extend
  `telemetry_rollup_test.go`); rendering deterministic from fixtures.
- [ ] Cost section added to PR summary data model (rendering wired in V7's
  submit task).
- **Validation:** `go test ./internal/core/... -run 'Report|Cost' -count=2`

## Rollout & cleanup

- [ ] Docs: command-reference (`--cost`, `--budget`), user-guide routing
  section, CHANGELOG; parity tests green.
- **Rollback:** remove routing block from config; no behavior change remains.
- **Completion evidence:** `make ci` green; reconciliation + table tests
  committed; no network code introduced (review checklist item).
