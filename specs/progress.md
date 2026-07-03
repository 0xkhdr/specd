# specd v0.2.0 — Implementation Progress

**Program:** Full-cycle harness (SPECD_V0.2.0_ACTION_PLAN.md).
**Overall status:** Waves 1–5 landed green; Wave 6 next.
**Current wave:** Wave 6 (V11 harness sharing/dashboard, V12 release).
**Note:** `specs/progress.md` tracks the separate regression program; this
file tracks v0.2.0 only.

## Plan task → spec coverage (P1.1–P6.4)

| Plan task | Description | Spec |
|-----------|-------------|------|
| P1.1 | state.json schema v2 + migration | V1 |
| P1.2 | Trajectory ledger | V3 |
| P1.3 | Eval rubric engine + `specd eval` (+ prototype/promote from §4) | V5 |
| P1.4 | Executable guardrails gate | V2 |
| P1.5 | Model router policy engine | V4 |
| P1.6 | Token economics report | V4 |
| P2.1–P2.6 | Micro-tasks, conductor engine, SSE/MCP surface, HUD, replay/analytics, host bindings | V6 |
| P3.1 | Remote worker pool hardening | V7 |
| P3.2 | Auto-escalation engine | V7 |
| P3.3 | ACP handoff interop (A2A-ready) | V7 |
| P3.4 | Batch PR workflow (`submit`) | V7 |
| P3.5 | Scheduled maintenance programs | V7 |
| P4.1 | Review workflow + gate | V8 |
| P4.2 | Security gate suite | V8 |
| P4.3 | Review checklist generator | V8 |
| P4.4 | Threat-model refresh (release gate) | V8 |
| P5.1 | Deploy driver runner | V9 |
| P5.2 | Production observability inbound | V9 |
| P5.3 | Legacy ingestion workflow | V10 |
| P5.4 | Migration spec packs | V10 |
| P5.5 | Feedback flywheel wiring + docs | V9 |
| P6.1 | Team harness sharing | V11 |
| P6.2 | Unified dashboard | V11 |
| P6.3 | Spec pack registry | V11 |
| P6.4 | Release engineering | V12 |

All 26 plan tasks covered by exactly one spec. ✅

## Spec status

| Spec | Directory | Deps | Wave | Status | Primary validation |
|------|-----------|------|------|--------|--------------------|
| V1  | `specs/v020-state-schema-v2` | — | 1 | Done ✅ | `go test ./internal/core/... -run 'State|Migrat' -count=2 && make stress` |
| V2  | `specs/v020-guardrails-gate` | — | 1 | Done ✅ | `go test ./internal/core/... -run Guardrail -count=2` |
| V3  | `specs/v020-trajectory-ledger` | V1 | 2 | Done ✅ | `go test ./internal/core/... -run Trajectory && make stress` |
| V4  | `specs/v020-model-routing-economics` | V1 | 2 | Done ✅ | `go test ./internal/core/... -run 'Rout|Cost'` |
| V5  | `specs/v020-eval-framework` | V1,V2,V3 | 3 | Done ✅ | `go test ./... -run 'Eval|Promote' -count=2` |
| V6  | `specs/v020-conductor-mode` | V1,V3(,V4) | 3 | Done ✅ | `go test ./... -run 'Conductor|Micro' -count=2` |
| V7  | `specs/v020-orchestrator-escalation` | V1,V4,V6 | 4 | Done ✅ | `go test ./... -run 'Backend|Escalat|Submit'` |
| V8  | `specs/v020-review-security-gates` | V1,V2,V5,V7 | 4 | Done ✅ (P4.4 deploy/observe deferred to post-V9 per note) | `go test ./... -run 'Review|Secur' -count=2` |
| V9  | `specs/v020-deploy-observe` | V5,V7,V8 | 5 | Done ✅ | `make ci` (flywheel e2e) |
| V10 | `specs/v020-legacy-ingestion-packs` | V1,V5,V7 | 5 | Done ✅ | `go test ./... -run 'Ingest|Pack' -count=2` |
| V11 | `specs/v020-harness-sharing-platform` | V2,V4,V5,V8,V9 | 6 | Done ✅ | `go test ./internal/core/... ./internal/pack/... ./internal/cmd/... -run 'Harness|Dashboard|Registry|Pack' -race -count=2` |
| V12 | `specs/v020-release-engineering` | V1–V11 | 6 | In progress (W1 landed; docs/bench/tag remain) | `make ci` + upgrade e2e |

## Waves (implementation order; mark done after each lands green)

- [x] **Wave 1 (foundation, no deps):** V1 state schema v6, V2 guardrails gate
- [x] **Wave 2 (ledgers + policy):** V3 trajectory (←V1), V4 routing/economics (←V1)
- [x] **Wave 3 (eval + conductor):** V5 evals (←V1,V2,V3), V6 conductor (←V1,V3)
- [x] **Wave 4 (scale + trust):** V7 orchestrator/escalation (←V4,V6), V8 review/security (←V2,V5,V7) — escalation engine, ACP scout→craftsman handoff, submit, `program schedule`/`tick` maintenance, review workflow + gate, security scanners, checklist; threat-model P4.4 covers exec surfaces now shipped (deploy/observe extended when V9 lands)
- [x] **Wave 5 (lifecycle close):** V9 deploy/observe/flywheel (←V5,V7,V8), V10 ingestion/packs (←V5,V7) — evidence-gated `deploy` driver + rollback + `approve --deploy`, `observe` correlate/listen → gated midreq, flywheel e2e + `docs/flywheel.md`, `ingest new` + inventory + `ingest` gate + `specd-ingest` skill, `migrate-deps`/`modernize-tests`/`upgrade-go` migration packs
- [~] **Wave 6 (platform + ship):** V11 harness sharing/dashboard/registry (←V9)
  **done**; V12 release (←all) — `migrate` + upgrade e2e landed; docs sweep,
  benchmark refresh, and the `main` tag remain

Ordering note: V8 depends on V7 only for PR-summary section wiring (P3.4
stubs); V8 Waves 1–2 can start in parallel with V7 if needed — the P4.4
threat-model wave must land after all exec surfaces (V5 command checks, V7
submit, V9 drivers) exist. Per-phase P0-only fallback applies (plan risk 1).

## Constitution checklist (every spec, every PR)

1. Foundational Split — binary never perceives/reasons/generates prose
2. Zero LLM calls in the binary (judges = external command plugins)
3. Zero external Go deps — stdlib JSON only, no YAML, `go.mod` stays 3 lines
4. State mutated only via CLI; dual-write discipline on every new ledger
5. Evidence gates every state change (micro-approval never bypasses `verify:`)
6. Deterministic reporting from state/ledgers, no generated prose
7. Byte-stable round-trips; FakeClock; fuzz every new parser
8. New exec surfaces = hostile input: env scrub, path validation, bwrap;
   single shared sandboxed exec path; adversarial tests in the same PR
9. Backward compat: v0.1.x commands unchanged; additive JSON; exit codes
   0/1/2/3 untouched; migrated repos default-off for new gates
10. Registry discipline: cmd file + Registry + CommandMeta + parity tests

## Success metrics (plan Part III — verify at V12)

| Metric | Target | Measured by |
|--------|--------|-------------|
| First-pass verify success | >85% | telemetry rollup per spec |
| Security fixture catch rate | >90% | V8 corpus test in CI |
| Mode-switch friction | <30s, zero context loss | V6 e2e timing + ledger continuity |
| Ingestion coverage | 100% mapped/waived | V10 `ingest` gate |
| Cost visibility | 100% tasks tier+cost attributed | V4 reconciliation test |
| Eval coverage | every completed spec ≥1 eval run | V5 gate (config-on) |
| Production correlation | every error → midreq w/ evidence | V9 tests |

## Decisions & deviations (plan vs verified codebase)

- **DV1 — Package path.** Plan references `internal/spec/*.go` for core
  machinery. Actual: `internal/core/` holds state/gates/parsers/briefs/
  telemetry/mode/programs; `internal/spec/` is a small phase/role/status
  package. All specs use `internal/core` paths.
- **DV2 — Schema version.** Plan says state "migrates version 1 → 2".
  Actual SchemaVersion is **5** (`internal/core/state.go`); v0.2.0 bumps
  **5 → 6** using the existing migration pattern. Plan's "v2 blocks" = v6
  blocks.
- **DV3 — Progress file placement.** `specs/progress.md` already tracks the
  regression program; v0.2.0 progress lives here (repo root) to avoid
  clobbering evidence.
- **DV4 — Prototype lifecycle homed in V5.** Plan §4 lists
  `--prototype`/`promote` under Requirements without a Phase-1 task ID;
  covered explicitly in V5 Wave 4 so no gap remains.

## Remaining work

- [x] Execute Wave 1 (V1, V2)
- [x] Execute Wave 2 (V3, V4)
- [x] Execute Wave 3 (V5, V6) — eval framework + prototype lifecycle, conductor
  mode + micro-tasks, context HUD, rejection analytics; eval gate + skill + docs
- [x] Execute Wave 4 (V7, V8) — escalation engine + conductor handoff, ACP
  inter-role handoff schema, batch `submit`, scheduled maintenance
  (`program schedule`/`tick`, `specd-maintenance` skill), review workflow + gate
  + reviewer role + `specd-review` skill, security scanner suite, review
  checklist, threat-model refresh (deploy/observe surfaces land with V9)
- [x] Execute Wave 5 (V9, V10) — deploy driver + rollback, observe correlation +
  loopback listener, feedback flywheel e2e + docs, legacy ingestion inventory +
  coverage gate + `specd-ingest` skill, migration spec packs
- [~] Execute Wave 6 (V11, V12) — V11 harness sharing (`harness push/pull/list/
  enable` + quarantine + decision log), unified `dashboard` (project-wide
  read-only panels, `--mode` filter, zero outbound), pack registry (`init
  --pack <name> --registry <git-url>` + `.specd/pack.lock` checksum pin) all
  landed with tests + docs; V12 `specd migrate` + upgrade e2e landed, CHANGELOG +
  SECURITY quarantine model + command-reference updated
- [x] `make ci` restored to GREEN (2026-07-03 review pass): raised coverage on
  new V11/V12 code above all floors (overall 79.1%, core 80.8%, cmd 71.2%,
  pack 87.6%) and renamed six banned `wave4_*`/`wave5_*` test files that were
  hard-failing `test-lint`. See `SPECD_V0.2.0_IMPLEMENTATION_REVIEW.md`.
- [ ] V12 remaining: full docs sweep (user-guide/validation-gates/mcp-guide/
  AGENTS.md — `specd migrate` absent outside command-reference), `make bench`
  vs v0.1.x baseline refresh, success-metrics table verification wiring
- [ ] Final release gate: metrics table verified in CI, v0.2.0 tagged from `main`
