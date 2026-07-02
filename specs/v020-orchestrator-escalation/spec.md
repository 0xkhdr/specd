# V7 — Orchestrator Scale & Auto-Escalation

## 1. Purpose and requirement coverage

Harden the multi-agent orchestrator to production grade and close the SDLC
"80% problem": remote worker-pool conformance, deterministic auto-escalation
(pause + conductor handoff, never auto-switch), ACP handoff interop
(A2A-ready fields), batch PR submission, and scheduled maintenance programs.
Covers plan tasks **P3.1–P3.5** (P0/P1/P2).

## 2. Verified current state

- Remote backends exist behind build tags: `backend_{git,redis,postgres}.go`
  with `backend_conformance_test.go`; ACP lease/claim/archive protocol
  (`acp_*.go`); worker package `internal/worker/`.
- Clock-skew tolerance tests exist (`progress_skew_test.go`).
- PR summary: `internal/core/prsummary.go`; programs:
  `program.go`/`program_session.go`; brain policy/retries:
  `internal/cmd/brain*.go`, `brain_policy.go`.
- V1 `state.json.escalation` block; V4 budget-breach escalation record;
  V6 conductor sessions to hand off into.

## 3. Proposed design and end-to-end flow

- **Pool hardening (P3.1):** promote redis/postgres from build-tag experiments
  to documented, conformance-tested backends — full
  lease/heartbeat/checkpoint/crash-recovery matrix, fault injection (kill
  worker mid-lease → lease expiry → reclaim), skew tolerance extended to
  backends; deployment topology + failure modes documented.
- **Escalation engine (P3.2):** `internal/core/escalation.go` — deterministic
  rules on every brain step and verify record: `verifyFailCount >= 2`,
  `retryCount >= maxRetries`, `blockerCount >= 1`, `costOverTierBudget` (V4),
  `complexityScore >= threshold`; thresholds in `config.json.escalation`.
  Trigger → brain pauses the task, writes `state.json.escalation`
  `{task, rule, facts, time}`, emits SSE + webhook event, and
  `mode_recommend` flips to `conductor` with the facts as rationale. Humans
  resolve via `specd mode --set conductor` or
  `specd orchestrate resume --override`. Never auto-switches.
- **ACP handoff (P3.3):** mission-brief schema gains `role`, `tier`,
  `handoff: {from, reason, artifacts}` so a scout's output brief feeds a
  craftsman; A2A concept mapping documented, wire compat deferred (§5.6).
- **Batch PR (P3.4):** PR summary gains eval/security/cost/escalation
  sections. New `specd submit <spec> [--waves w1,w2]`: validates all gates
  green for the bundle, generates the summary, execs user-configured
  `submit.command` (e.g. `gh pr create --body-file -`) sandbox-recorded. No
  git/GitHub logic embedded.
- **Scheduled programs (P3.5):** `specd program schedule --interval` writes a
  schedule manifest; `specd program tick` is host-triggered (cron/systemd/CI)
  and CAS-guarded idempotent; the binary never daemonizes. `specd-maintenance`
  skill ships alongside.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Config:** `escalation` thresholds, `submit.command` in config.json.
- **New commands:** `submit`, `program schedule|tick`,
  `orchestrate resume --override` (registry discipline).
- **Stable:** brief schema versioned + validated; same conformance suite must
  pass against memory/git/redis/postgres backends.
- **Dependencies:** V1, V4 (budget rule), V6 (conductor handoff target),
  V5+V8 feed submit's summary sections (render empty until they land).
  **Dependents:** V9 (deploy after submit-style gating), V10 (schedules run
  migration packs), V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Escalation rules pure over countable facts (invariant 1); table-tested.
- Submit command is hostile-config exec: shared sandboxed exec path, env
  scrub, recorded exit code; failure → exit 1, no partial state (adversarial
  tests in the same PR, P4.4 cadence).
- Redis/postgres in CI via services when available, **skipped-not-failed**
  otherwise.
- Tick idempotent under concurrent invocation (CAS-guarded, like brain
  resume); no background threads.
- **Rollback:** disable escalation via config thresholds; backends remain
  opt-in via build tags/config.

## 6. Acceptance criteria and validation commands

- Backend conformance matrix green across all four backends; fault-injection
  (mid-lease kill → reclaim) test.
- Table-driven escalation rule tests; e2e: two verify failures → paused task +
  escalation record → conductor session starts on the escalated task with full
  context brief.
- Scout→craftsman handoff e2e via ACP store; brief schema validation tests.
- Submit: summary deterministic from fixtures; command failure recorded,
  exit 1, no partial state.
- Tick: double-invoke idempotency test.
- `go test ./internal/core/... ./internal/worker/... -run 'Backend|Escalat|Handoff|Submit|Program' -race -count=2 && make stress`

## 7. Open decisions and deviations

- Path deviation DV1. A2A wire compatibility explicitly deferred to v0.3.0.
- Open: whether `submit` requires the review gate (V8) when
  `config.review.required` is on. Decision: yes — submit validates *all*
  configured gates; sections render as "not configured" otherwise.
