# V7 Tasks — Orchestrator Scale & Auto-Escalation

Plan coverage: P3.1–P3.5. Dependencies: V1, V4, V6. Dependents: V9, V10, V12.

## Wave 1 — Worker pool hardening (P3.1)

- [ ] Extend `backend_conformance_test.go` to full
  lease/heartbeat/checkpoint/crash-recovery matrix over
  memory/git/redis/postgres.
- [ ] Fault injection: kill worker mid-lease → lease expiry → reclaim; extend
  skew tolerance tests to remote backends.
- [ ] CI: redis/postgres via services when available, skipped-not-failed
  otherwise; document topology + failure modes in
  `docs/agent-integration.md`.
- **Validation:** `go test ./internal/core/... ./internal/worker/... -run 'Backend|Lease' -race`

## Wave 2 — Escalation engine (P3.2, depends on Wave 1)

- [ ] `internal/core/escalation.go`: rule set (verifyFail≥2, retry≥max,
  blocker≥1, costOverTierBudget, complexity≥threshold), thresholds from
  `config.json.escalation`; table-driven tests.
- [ ] Wire into brain step + verify record: pause task, write
  `state.json.escalation`, emit SSE + webhook event; `mode_recommend` flips
  to `conductor` with facts.
- [ ] Resolution paths: `specd mode --set conductor`,
  `specd orchestrate resume --override`; e2e: two verify fails → paused →
  conductor session on escalated task with context brief.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Escalat -race -count=2`

## Wave 3 — ACP handoff interop (P3.3, depends on Wave 2)

- [ ] Brief schema: `role`, `tier`, `handoff {from, reason, artifacts}` in
  `pinky_brief.go` / `acp_*.go`; versioned + validated.
- [ ] Scout→craftsman handoff e2e via ACP store; A2A mapping documented.
- **Validation:** `go test ./internal/core/... -run 'Brief|ACP|Handoff' -race`

## Wave 4 — Submit + schedules (P3.4 + P3.5, depends on Wave 3)

- [ ] PR summary sections: eval scores, security results, cost, escalation
  history (render "not configured" until V5/V8 recorded data exists).
- [ ] `specd submit <spec> [--waves]`: bundle gate validation → summary →
  sandbox-recorded exec of `submit.command`; failure = exit 1, no partial
  state; adversarial config tests same PR.
- [ ] `specd program schedule --interval` manifest + `specd program tick`
  (host-triggered, CAS-guarded idempotent, no daemonization); double-invoke
  test; `specd-maintenance` skill.
- **Validation:** `go test ./internal/cmd/... -run 'Submit|Program' -race -count=2`

## Rollout & cleanup

- [ ] Docs: agent-integration (backends, handoff, A2A mapping),
  command-reference (submit, schedule/tick), CHANGELOG; parity tests green.
- **Rollback:** escalation thresholds → disabled; submit/schedule unused
  without config.
- **Completion evidence:** `make ci` green incl. conformance matrix; e2e
  escalation→conductor handoff test committed.
