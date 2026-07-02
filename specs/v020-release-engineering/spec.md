# V12 — v0.2.0 Release Engineering

## 1. Purpose and requirement coverage

Ship v0.2.0: migration tooling, docs sweep, benchmark comparison, upgrade
verification, and the release gates (threat model from V8, success metrics
from the plan). Covers plan task **P6.4** (P0) and Part III cross-cutting
standards as release acceptance.

## 2. Verified current state

- Release tooling: `.goreleaser.yml`, `scripts/` install flow with SHA256
  verification, `Makefile` ci/bench/cover-check targets.
- Docs-parity tests enforce command-reference completeness.
- Baselines: `docs/agent-harness-baselines.md`; CHANGELOG follows
  Keep-a-Changelog.
- State migration v5→v6 landed in V1; config additions across V2–V11.

## 3. Proposed design and end-to-end flow

- `specd migrate`: documented one-shot for config/state — idempotent wrapper
  over the V1 silent migration plus config-block scaffolding hints (never
  auto-writes policy content like guardrails rules; it reports what's
  available).
- CHANGELOG: Keep-a-Changelog discipline, breaking changes called out (none
  expected — invariant 9).
- Docs sweep: every new command in command-reference + user-guide;
  validation-gates covers guardrails/eval/review/security/ingest/deploy
  gates; mcp-guide covers `specd_conductor` + new resources; AGENTS.md
  template gains eval/review/conductor discipline; docs-parity tests enforce.
- Benchmarks: refresh `docs/agent-harness-baselines.md` vs v0.1.x; perf
  within ±10% on existing paths (regression program floor).
- Upgrade test: a v0.1.x-initialized fixture repo runs **every** v0.2.0
  command correctly after `specd migrate`.
- Success-metrics verification (plan Part III table): each metric's measuring
  test exists and is wired into CI (first-pass verify telemetry, security
  catch rate, mode-switch continuity, ingestion coverage, cost attribution
  reconciliation, eval-coverage gate, observe→midreq evidence).

## 4. Interfaces, contracts, data, configuration, dependencies

- **New command:** `specd migrate` (registry discipline).
- **Stable:** goreleaser matrix unchanged; install.sh SHA256 flow
  re-verified; exit-code contract 0/1/2/3 untouched; `go.mod` still 3 lines.
- **Dependencies:** V1–V11 all complete. **Dependents:** none (terminal).

## 5. Invariants, security, errors, observability, compatibility, rollback

- Backward compat is the release thesis: all v0.1.x commands unchanged;
  migrated repos default-off for new gates (eval/review), new inits
  default-on.
- V8 threat-model refresh is a hard release gate.
- Coverage floors held or raised (`make cover-check`); race-clean, `-count=2`
  order-independent across the suite.
- **Rollback:** v0.2.0 binaries read v5 state via migration; downgrade story
  documented (new blocks ignored by v0.1.x readers is NOT guaranteed —
  document explicitly).

## 6. Acceptance criteria and validation commands

- `make ci` green on the full tree (includes flywheel e2e from V9).
- Upgrade test: v0.1.x fixture + `specd migrate` → every v0.2.0 command
  correct.
- `bash scripts/install_test.sh` (checksum flow) green.
- Docs-parity + registry/help parity tests green; CHANGELOG complete.
- `make bench` comparison recorded in baselines doc; floors held.
- Success-metrics table: each row's measuring test identified and green.

## 7. Open decisions and deviations

- Deviation DV1 propagated: release docs must reference `internal/core`
  paths, and the plan's "state v1→v2" is documented as "schema v5→v6".
- Open: whether v0.2.0 is tagged from `v0.2.0` branch or merged to `main`
  first. Follow repo convention: PR into `main`, tag on `main`.
