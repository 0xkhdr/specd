# specd v0.2.0 — Comprehensive Action Plan
## Full-Cycle Agentic Engineering Platform, Built Without Betraying the Harness

**Date:** 2026-07-02
**Target version:** v0.2.0
**Inputs:** `specd_v0.2.0_evolution_plan.md` (draft evolution plan), *The New SDLC with Vibe Coding* (`Day_1_v3.pdf`), current codebase on branch `v0.2.0`
**Status:** Actionable plan — supersedes the draft evolution plan where they disagree

---

## Part I — Analysis

### 1. Vision statement

specd v0.1.x proved one idea: **the agent reasons, the harness enforces**. Every artifact is
Markdown on disk, every state change is evidence-gated, every report is deterministic, and the
binary makes zero LLM calls with zero external dependencies.

The SDLC paper describes the destination: a **factory model** where developers configure a
harness (requirements, architecture, guardrails), run it (implementation), close feedback loops
(testing, evals), and observe it (review, deployment, maintenance) — fluidly switching between
**Conductor** (hands-on, real-time, the "last 20%") and **Orchestrator** (async, multi-agent
delegation, the "first 80%").

v0.2.0's thesis: **specd becomes the full-cycle harness by extending its enforcement surface,
never by absorbing the agent's reasoning.** Every gap in the SDLC theory closes through one of
three mechanisms, and only these three:

1. **Deterministic machinery in the binary** — parsers, gates, routers-as-policy, ledgers,
   schedulers. (Where the draft plan says "AI does X inside specd", this plan says "specd
   *demands, records, and validates* X".)
2. **Skills + roles for the agent** — progressive-disclosure Markdown that teaches any host
   agent to produce the artifact the gate demands (`specd-review`, `specd-ingest`,
   `specd-eval-author`, `specd-conductor`).
3. **External command plugins** — the custom-gate pattern generalized: user-configured
   executables (LM judge, security scanners, deploy drivers) that specd invokes, sandboxes,
   and records, but never embeds.

This is not a compromise of the draft plan — it is the only version of it that survives contact
with specd's own constitution. The repo already learned this lesson once: `boot` and `enrich`
were **removed** (see CHANGELOG "Unreleased → Removed (breaking)") because they performed repo
perception and steering authoring *inside the binary*, violating the Foundational Split. The
draft plan's "Legacy Ingestion Mode" and "AI Review Agent" as in-binary features would repeat
that exact mistake. This plan corrects them.

### 2. Ground truth: the draft plan's baseline is stale

The draft plan gap analysis assumes a v0.1.0 that no longer exists. The current tree already
contains substantial v0.2.0-direction machinery. Building the plan against reality:

| Draft plan says "missing/partial" | Actually present today | Consequence for this plan |
|---|---|---|
| "Brain/Pinky single-project, single-model, local only" | `internal/spec/backend_{git,redis,postgres}.go` remote worker backends; ACP lease/claim/archive protocol (`internal/spec/acp_*.go`); `internal/worker/` | Worker-pool scaling is a **hardening + docs task**, not a build task |
| "No cost tracking" | `internal/spec/cost_brake.go`, `telemetry.go`, `report_metrics.go`, pinky telemetry (tokens/cost/duration as metadata) | Token-economics ledger is an **extension** (per-tier attribution, CapEx/OpEx report), not new |
| "Guardrails static text only" | Custom gates engine (`internal/spec/customgate.go`, `docs/custom-gates.md`) | Executable guardrails = **new built-in gate class** on the existing pipeline |
| "No dashboards / no watch" | `serve.go`, `watch.go`, `watch_sse.go`, `watch_webhook.go`, `docs/dashboard.md` | Unified dashboard = extend, not create |
| "No PR summary" | `internal/spec/prsummary.go` | Enhance with eval/security/cost sections |
| "No mode awareness" | `mode.go`, `mode_recommend.go` (deterministic simple/orchestrated recommendation from countable facts) | Conductor/orchestrator switching **extends the existing mode state machine** |
| "CLI-only, no host integration" | `internal/integration/` host adapters (Claude, Codex, Cursor, VS Code, Antigravity), MCP server with HTTP transport, handshake/negotiation | Conductor mode builds on the **existing host-adapter + MCP layer**; a bespoke IDE extension is out of scope for the Go repo (see §5.2) |
| "No session replay" | `session_replay.go`, `replay.go` | Conductor ledger replay reuses this |

Genuine gaps confirmed (the real v0.2.0 work): **eval framework, trajectory ledger, model
routing policy, conductor micro-task protocol, auto-escalation engine, AI-review gate,
security gate suite, deployment/observe integration, legacy ingestion workflow, guardrails
gate, feedback flywheel, harness sharing.**

### 3. Non-negotiable invariants (the constitution)

Every task below must hold these. A task that cannot is redesigned, not excepted.

1. **Foundational Split** — the binary never perceives, reasons, or generates prose. It
   scaffolds, validates, gates, records, routes, schedules.
2. **Zero LLM calls in the binary** — semantic scoring goes through external command plugins
   the user configures, with results recorded as evidence.
3. **Zero external Go dependencies** — stdlib only (`go.mod` stays 3 lines). YAML is therefore
   out: all new config/rubric artifacts are **JSON** (`encoding/json`) or the existing bespoke
   Markdown/line formats. The draft plan's `.yml` artifacts become `.json`. (Existing
   build-tagged redis/postgres backends already respect this pattern; keep it.)
4. **Specs as source of truth, state mutated only via CLI** — new artifacts live under
   `.specd/`, dual-write discipline extends to every new ledger.
5. **Evidence gates every state change** — new phases (review, deploy) get the same
   exit-code-recorded proof discipline as `verify:`.
6. **Deterministic reporting** — eval scores, security findings, costs, escalations render
   from `state.json`/ledgers; no generated prose in reports.
7. **Determinism + byte-stable round-trips** — every new parser gets round-trip tests; every
   new gate is pure over on-disk facts; `FakeClock`/testharness for anything time-dependent.
8. **Security model extends, never weakens** — everything executed from agent-authored files
   is hostile input: env scrubbing, path validation, sandbox (`bwrap`) for all new exec
   surfaces (guardrail commands, eval plugins, deploy drivers).
9. **Backward compatibility** — all v0.1.x commands unchanged; `state.json` migrates
   version 1 → 2 transparently on load (repo already has this pattern in `state.go`);
   additive JSON fields; exit-code contract (0/1/2/3) untouched.
10. **Registry discipline** — every new command: `internal/cmd/<cmd>.go` + `cmd.Registry`
    entry + `CommandMeta` + tests; `TestRegistryMatchesHelp` and docs-parity tests must pass.

### 4. SDLC gap → mechanism map

How each theory concept closes under the three allowed mechanisms:

| SDLC phase | Theory concept | v0.2.0 mechanism | Type |
|---|---|---|---|
| Requirements | AI-assisted refinement, edge-case discovery | `specd-requirements` skill upgrade + `ears` gate additions (edge-case coverage heuristics: countable "unhappy path" requirement presence) | Skill + gate |
| Requirements | Specs as eval criteria | `specd eval init` compiles approved `requirements.md` into a rubric skeleton (`.specd/evals/<spec>.json`) the agent completes; `eval` gate blocks completion without a rubric run | Binary + skill |
| Requirements | Interactive prototyping | `specd new --prototype` lifecycle: a spec flagged `prototype` skips design/tasks gates but **cannot** reach `complete` — must be promoted (`specd promote`) into a full spec | Binary |
| Design | Decision rationale w/ alternatives | `design` gate v2: mandatory "Alternatives Considered" section; `decisions.md` ADR link check (traceability gate extension) | Gate |
| Design | AI scaffolding from design | `specd-design` skill: emit `scaffold:` block in `design.md`; `specd scaffold <spec>` applies it deterministically (mkdir/touch/template only, no content generation) | Binary + skill |
| Design | Guardrails as code | `.specd/guardrails.json` + built-in `guardrails` gate (forbidden imports/regex/paths/commands), deterministic, runs in `specd check` before any agent work | Binary |
| Implementation | Conductor mode | Micro-task protocol + conductor session ledger + SSE stream via existing `serve`/`watch`; host adapters surface it (§5.2) | Binary + integration |
| Implementation | Orchestrator multi-agent | Harden existing Brain/Pinky + remote backends; A2A-style handoff via ACP mission briefs with `role` + `tier` fields | Binary |
| Implementation | 80% problem detection | Auto-escalation engine: deterministic rules over countable facts (verify-fail count, retry count, blocker count, contract complexity score from `mode_recommend` signals) → pause + conductor handoff record | Binary |
| Implementation | Intelligent model routing | Router as **policy, not client**: config maps task role/complexity → tier; tier stamped into mission brief + `state.json.routing`; the *host* picks the actual model, specd records claimed model + cost and enforces budget via extended cost brake | Binary |
| Testing | Output evals | Already covered (`verify:`) — keep | — |
| Testing | Trajectory evals | Trajectory ledger (`.specd/specs/<slug>/trajectory.jsonl`): pinky/MCP tool-event appends; `specd eval --trajectory` scores against rubric (deterministic checks) + optional judge plugin | Binary + plugin |
| Testing | Quality flywheel | `specd eval trend` over ledger history; failure clustering by deterministic keys (gate name, task id, error class); regression detection = score deltas | Binary |
| Testing | AI-generated edge-case tests | `specd-tasks` skill: tasks must declare `tests:` intent; `evidence` gate accepts property-test evidence class | Skill + gate |
| Review | AI-first review | New role `reviewer` + `specd-review` skill; `specd review <spec>` scaffolds `review_report.md`, validates its structure, records it; new `review` gate blocks `approve` into `complete` without a filed report (human `approve` stays final) | Binary + skill |
| Review | Security gates | Built-in gate suite: `secrets` (entropy+pattern), `deps` (lockfile CVE via plugin), `injection` (pattern), `slopsquatting` (package-name distance vs manifest) — `specd check --security` | Binary + plugin |
| Deployment | Pipeline integration, canary, rollback | `specd deploy` = evidence-gated **driver runner**: `.specd/deploy/<env>.json` declares external commands (gh CLI, kubectl, argocd); specd sandboxes, sequences, records exit codes; rollback = declared inverse command; no CD logic embedded | Binary + plugin |
| Maintenance | Legacy ingestion | `specd ingest` scaffolds an ingestion spec + `specd-ingest` skill teaches the agent to reverse-engineer into it; `ingest` gate validates coverage (every listed source file mapped to a requirement) — perception stays with the agent (the boot/enrich lesson) | Binary + skill |
| Maintenance | Auto-refactor | Spec packs (`internal/pack`) gain migration packs; scheduled programs run them | Packs |
| Maintenance | Production feedback loop | Extend `watch_webhook.go` inbound: `specd observe` receives error payloads → deterministic correlation (stack path ↔ task file lists) → auto-append `mid-requirements.md` entry (existing midreq machinery) | Binary |
| Ecosystem | Team harness sharing | `.specd/harness/` versioned bundle (guardrails, evals, routing, deploy configs) + `specd harness pull/push` via git (stdlib exec of `git`) | Binary |

### 5. Architecture decisions

#### 5.1 Dual mode = state, not product split
`ExecutionMode` today: `simple | orchestrated`. v0.2.0 adds `conductor`. All three share
`state.json` and artifacts. `mode_recommend.go` already computes deterministic advisory
recommendations — extend its signal set (contract complexity, verify-fail history) to also
recommend `conductor`, and let the escalation engine *request* (never force) the switch.
Switching = `specd mode <spec> --set conductor --reason "..."` recorded in the decision log.
This satisfies the draft plan's "mode switching is a state transition" with ~200 lines instead
of a parallel subsystem.

#### 5.2 Conductor without owning an IDE extension
The draft plan's P0 "VS Code extension" is a separate product with a separate release
lifecycle. The Go repo ships the **protocol**; editors consume it:

- **Micro-task protocol**: `tasks.md` schema gains optional `micro:` sub-items under a task
  (parser extension, round-trip tested). `specd conductor step/accept/reject` walks them.
- **Conductor ledger**: every propose/accept/reject appended (O_APPEND, like existing ledgers)
  to `conductor.jsonl`; `specd conductor replay` reuses `session_replay.go`.
- **Live surface**: existing `specd serve` + SSE watch stream conductor events; existing MCP
  server exposes `specd_conductor` tool so Claude Code / Cursor / any MCP host *is* the IDE
  integration on day one. The `internal/integration` host adapters scaffold host-native
  bindings (e.g. VS Code tasks.json, Claude Code skill) — that's the shipped "extension".
- A real marketplace extension becomes a **separate repo** consuming the SSE + CLI contract;
  out of v0.2.0 scope, unblocked by it.

#### 5.3 Router is a policy engine
`.specd/config.json` gains:
```json
"routing": {
  "tiers": {
    "frontier": {"maxCostUSD": 2.0},
    "fast": {"maxCostUSD": 0.1}
  },
  "rules": [
    {"match": {"role": "scout"}, "tier": "fast"},
    {"match": {"complexityAtLeast": 7}, "tier": "frontier"},
    {"default": true, "tier": "fast"}
  ]
}
```
Deterministic evaluation (first-match), stamped into mission briefs and `state.json.routing`.
specd never opens a socket to a model. Budget enforcement extends `cost_brake.go`: reported
spend over tier budget → brake → escalation record. Model *names* are host concerns; tiers are
harness concerns. This keeps agent-agnosticism (principle 5) and zero-LLM (invariant 2).

#### 5.4 Evals: deterministic core, plugin judge
- **Rubric** `.specd/evals/<suite>.json`: array of checks, each one of the deterministic kinds
  `file_pattern` (regex over changed files), `artifact_present`, `trajectory` (predicates over
  the tool-event ledger: max retries, forbidden tools, verify-before-complete ordering),
  `command` (sandboxed executable, exit code = pass/fail — the custom-gate pattern), each with
  points.
- **Trajectory ledger**: append-only JSONL of tool events. Sources: pinky reports (already
  structured), MCP server middleware (it sees every tool call), and a documented
  `specd trace append` for other hosts. Scoring is arithmetic over recorded facts.
- **LM judge** = a `command` check the user points at their own script. Disabled by default.
  specd records its exit code and stdout digest as evidence — determinism of the *record*,
  not of the judge.

#### 5.5 Review and ingestion follow the "demand the artifact" pattern
`specd review <spec>`: scaffolds `.specd/specs/<slug>/review_report.md` from template with
mandatory sections (Bugs, Security, Style, Hallucinated deps, Verdict), prints the reviewer
role brief; the agent (any host) fills it; `specd check` gains a `review` gate validating
structure + verdict present; `specd approve` into `complete` requires it. Identical shape for
`specd ingest` (scaffold ingestion spec + coverage gate). The binary never judges code or
reads legacy semantics — it makes skipping the judgment impossible.

#### 5.6 What this plan cuts from the draft (with reasons)
- **In-binary LM judge / AI reviewer / AI ingestion** → plugin/skill pattern (invariants 1–3).
- **Marketplace VS Code extension inside this repo** → separate repo post-protocol (§5.2).
- **MCP server marketplace, org web dashboard, certified-partner program** → deferred to
  v0.3.0; they are ecosystem programs, not harness code, and would dilute six months of focus.
- **YAML configs** → JSON (invariant 3).
- **A2A protocol full implementation** → v0.2.0 ships ACP mission-brief interop fields
  (role/tier/handoff) and documents the mapping; full Google-A2A wire compat deferred until
  the spec stabilizes — tracked as a v0.3.0 candidate.

---

## Part II — Implementation plan

Six phases, ~24 weeks, each phase shippable behind additive flags. Task IDs `P<phase>.<n>`.
Every task implicitly ends with: unit + integration tests (race-clean, `-count=2` safe),
`make ci` green, docs updated (`docs/command-reference.md`, `docs/validation-gates.md`,
CHANGELOG entry), and registry/help/docs parity tests passing.

### Phase 1 — Evaluation & policy foundation (weeks 1–4)

**P1.1 — `state.json` schema v2 + migration** *(P0)*
- `internal/spec/state.go`: add `mode` (extended enum), `evals`, `routing`, `conductor`,
  `escalation` blocks; bump `version: 2`; loader migrates v1 silently (existing migration
  pattern), writer always emits v2.
- Best practice: additive fields with `omitempty`; migration idempotent; corrupt-state tests
  extend `config_corruption_test.go` patterns; CAS/revision semantics untouched.
- Accept: v1 fixtures load, round-trip to valid v2; all existing state tests pass unmodified.

**P1.2 — Trajectory ledger** *(P0)*
- New `internal/spec/trajectory.go`: append-only `trajectory.jsonl` per spec (O_APPEND +
  fsync discipline from `io.go`); event schema `{time, actor, tool, args_digest, outcome,
  task}`; args are **digested (sha256), never stored raw** — ledger must not become a secrets
  sink.
- Wire producers: pinky report/progress paths; MCP server `tools.go` middleware; new
  `specd trace append` command for CLI-only hosts.
- Accept: concurrent-append stress test (reuse lock/stress harness); NUL/oversize-line
  rejection; replay-safe ordering by monotonic sequence, not wall clock.

**P1.3 — Eval rubric engine + `specd eval`** *(P0)*
- New `internal/spec/eval.go` + `internal/cmd/eval.go`. Rubric JSON schema (validated with
  actionable line-level errors, like `tasksparser`). Check kinds: `artifact_present`,
  `file_pattern`, `trajectory`, `command` (sandboxed via the existing custom-gate exec path:
  env scrub, timeout, bwrap when available).
- `specd eval <spec> [--suite <name>]` → scores into `state.json.evals` + full result file
  `.specd/specs/<slug>/evals/<suite>-<seq>.json`.
- `specd eval init <spec>` compiles approved requirements into rubric skeleton: one
  `artifact_present`/`file_pattern` stub per EARS requirement ID (deterministic transform —
  no interpretation), agent refines via new `specd-eval-author` skill.
- Accept: rubric round-trip; scoring pure (same inputs → same score); hostile rubric tests
  (command injection attempts in rubric fields are rejected/escaped); exit 1 on below
  `minScore`.

**P1.4 — Executable guardrails gate** *(P1)*
- `.specd/guardrails.json`: `forbiddenImports`, `forbiddenPatterns` (RE2), `forbiddenPaths`,
  `forbiddenCommands` (matched against `verify:` lines and rubric `command` checks).
- New built-in gate `guardrails` in the `specd check` pipeline (`internal/spec/gates.go`),
  running **first**; scans only tracked/changed files by default (`git diff --name-only`
  against base, via stdlib exec) with `--all` override for full scans.
- Scaffold via `specd init --guardrails`; ship secure-default template (no `crypto/md5`,
  `crypto/des`, `math/rand` for tokens, etc. — language-aware sets in template comments).
- Accept: gate deterministic; RE2-only (no backtracking DoS); guardrails file itself is
  agent-visible but changes to it are surfaced in `specd status` (tamper visibility).

**P1.5 — Model router policy engine** *(P0)*
- New `internal/spec/routing.go`: config block (§5.3), first-match rule evaluation over
  countable task facts (role, complexity score reusing `mode_recommend` signals, file count,
  retry count). Output stamped into mission briefs (`pinky_brief.go`) and
  `state.json.routing` per task.
- Extend `cost_brake.go`: per-tier budgets; `--budget` flag on `brain start` maps to spec-level
  cap; breach → brake + escalation record (consumed by P3.2).
- Accept: rule evaluation table-tested; unknown fields rejected with line context; brief
  includes tier + budget so any host can honor it; no network code added.

**P1.6 — Token economics report** *(P1)*
- Extend `report_metrics.go`/`prsummary.go`: `specd report <spec> --cost` renders per-task,
  per-wave, per-tier spend from recorded telemetry; CapEx (spec authoring sessions) vs OpEx
  (execution sessions) split derived from phase at time of telemetry record.
- Accept: rendering deterministic from fixtures; totals reconcile with telemetry rollup
  (`telemetry_rollup_test.go` extended).

### Phase 2 — Conductor mode (weeks 5–8)

**P2.1 — Micro-task schema** *(P0)*
- `tasksparser.go`: optional `micro:` list items under a task (`- [ ] m1: rename x`), IDs
  `m<N>` scoped to parent task. Byte-stable round-trip mandatory; specs without `micro:`
  parse identically to today (golden compatibility tests on existing fixtures).
- DAG untouched: micro-tasks are a linear sequence inside one task (draft plan's "sub-DAG"
  deferred — linear covers the conductor use case and keeps the parser honest).

**P2.2 — Conductor session engine** *(P0)*
- New `internal/spec/conductor.go` + `internal/cmd/conductor.go`:
  `specd conductor start <spec>` (requires mode `conductor`, takes spec lock, opens session in
  `state.json.conductor` + `conductor.jsonl` ledger) · `step` (emit next micro-task brief) ·
  `accept [--evidence]` · `reject --reason` (mandatory reason — it is the training signal) ·
  `stop`. Task completes only when all micro-tasks accepted **and** the normal evidence gate
  passes — micro-approval never bypasses `verify:` (integrity core untouched).
- `specd conductor switch orchestrated` / mode command equivalent: closes session, records
  transition + reason in decision log.
- Accept: lifecycle e2e test (start→step→reject→step→accept→verify→complete); lock prevents
  concurrent brain session on same spec; ledger replay reconstructs session exactly.

**P2.3 — Live conductor surface (SSE + MCP)** *(P0)*
- Extend `watch_sse.go`: conductor events stream on existing endpoint (new event types,
  versioned). Extend MCP `tools.go`: `specd_conductor` tool (step/accept/reject) with the
  same validation as CLI (shared core functions, no logic in transport — existing parity-test
  pattern `parity_test.go` extended).
- Accept: MCP/CLI parity test for every conductor verb; SSE event schema documented in
  `docs/agent-integration.md`; auth on HTTP transport unchanged.

**P2.4 — Context HUD (deterministic)** *(P1)*
- Extend `internal/context/estimate.go` + `specd context <spec> --hud`: table of loaded
  steering files, active skills, byte/approx-token counts, current mode/tier. Also exposed as
  SSE event + MCP resource.
- Accept: counts derived from files on disk only; stable across runs.

**P2.5 — Conductor replay + rejection analytics** *(P1)*
- `specd conductor replay <spec> [--session <id>]` reusing `session_replay.go`.
- `specd report <spec> --conductor`: deterministic aggregation of rejection reasons (exact-
  string clustering + count) — the "developer tends to reject AI error handling" signal
  without any interpretation.
- Accept: replay byte-identical for same ledger; report from fixtures.

**P2.6 — Host adapter bindings for conductor** *(P1)*
- `internal/integration`: extend VS Code / Claude / Cursor adapters to scaffold conductor
  bindings (VS Code `tasks.json` entries, Claude Code skill stub invoking
  `specd conductor ...`, MCP config). This is the shipped "IDE integration" for v0.2.0.
- Accept: adapter conformance tests extended; `specd init --ide <host>` idempotent
  (marker-merge pattern from `agents.go`).

### Phase 3 — Orchestrator scale & escalation (weeks 9–12)

**P3.1 — Remote worker pool hardening** *(P1)*
- Promote redis/postgres backends from build-tag experiments to documented, conformance-
  tested backends: extend `backend_conformance_test.go` to full lease/heartbeat/checkpoint/
  crash-recovery matrix; document deployment topology + failure modes in
  `docs/agent-integration.md`.
- Best practice: fault-injection tests (kill worker mid-lease → lease expiry → reclaim);
  clock-skew tolerance tests exist (`progress_skew_test.go`) — extend to backends.
- Accept: same test suite passes against memory/git/redis/postgres backends (redis/postgres
  in CI via services when available, skipped-not-failed otherwise).

**P3.2 — Auto-escalation engine** *(P0)*
- New `internal/spec/escalation.go`: deterministic rules evaluated on every brain step and
  verify record: `verifyFailCount >= 2`, `retryCount >= maxRetries`, `blockerCount >= 1`,
  `costOverTierBudget`, `complexityScore >= threshold`. Configurable thresholds in
  `config.json.escalation`. Trigger → brain pauses task, writes
  `state.json.escalation = {task, rule, facts, time}`, emits SSE + webhook event, and
  `mode_recommend` output flips to `conductor` with the facts as rationale.
- Human resolves via `specd mode --set conductor` or `specd orchestrate resume --override`.
  The harness never auto-switches mode (principle 6: human at boundaries).
- Accept: table-driven rule tests; e2e: two verify failures → paused + escalation record →
  conductor session can start on the escalated task with full context brief.

**P3.3 — ACP handoff interop (A2A-ready)** *(P1)*
- Extend mission brief schema (`pinky_brief.go`, `acp_*.go`): `role`, `tier`,
  `handoff: {from, reason, artifacts}` fields so a scout worker's output brief becomes a
  craftsman worker's input. Document the mapping to A2A concepts in
  `docs/agent-integration.md`; wire-level A2A deferred (§5.6).
- Accept: brief schema versioned + validated; scout→craftsman handoff e2e via ACP store.

**P3.4 — Batch PR workflow** *(P1)*
- `specd report --pr-summary` (exists) gains eval scores, security gate results, cost, and
  escalation history sections. New `specd submit <spec> [--waves w1,w2]`: validates all gates
  green for the bundle, generates the summary, then execs user-configured
  `submit.command` (e.g. `gh pr create --body-file -`) sandbox-recorded. No git/GitHub logic
  embedded.
- Accept: summary deterministic from fixtures; command failure → recorded, exit 1, no
  partial state.

**P3.5 — Scheduled maintenance programs** *(P2)*
- Extend `program.go`/`program_session.go`: `specd program schedule --interval` writes a
  schedule manifest; execution is host-triggered (`specd program tick` — cron/systemd/CI
  invokes it; the binary does not daemonize). Ship `specd-maintenance` skill: scan for
  deprecated deps/failing CI, author new specs through the normal gates.
- Accept: tick idempotent (CAS-guarded like brain resume); no background threads.

### Phase 4 — Review & security (weeks 13–16)

**P4.1 — Review workflow + gate** *(P0)*
- New role template `roles/reviewer.md` (read-only, adversarial checklist) + `specd-review`
  skill. `specd review <spec>`: scaffolds `review_report.md` (mandatory sections: Summary,
  Bugs, Security, Hallucinated Dependencies, Style, Verdict `approve|revise`), prints role
  brief. New `review` gate: report exists, structurally valid, verdict present, newer than
  latest task completion. `specd approve` verifying→complete blocked without it
  (`config.review.required` — default **on** for new inits, off for migrated repos:
  compat).
- Accept: gate tests (missing/stale/malformed report); e2e verifying→complete path; human
  approval remains the final authority (report is evidence, not decision).

**P4.2 — Built-in security gate suite** *(P0)*
- New `internal/spec/security/` gates, all deterministic, all stdlib:
  - `secrets`: high-entropy string + known-format patterns (AWS keys, PEM blocks, JWTs) over
    changed files; allowlist file for false positives (`.specd/security/allow.json`,
    entries require a reason string).
  - `injection`: pattern heuristics (string-concatenated SQL, `exec` of interpolated input) —
    advisory severity by default, blocking via config.
  - `slopsquatting`: dependency names from manifests (go.mod, package.json, requirements.txt
    — parsed with stdlib) checked by edit-distance against a shipped popular-package list;
    flags near-misses.
  - `deps` CVE scan: **plugin gate** (`command` kind) — user points at osv-scanner/grype;
    specd records findings. No CVE database embedded.
- `specd check <spec> --security` runs the suite; findings recorded in `state.json` +
  rendered in reports/PR summary.
- Best practice: write these as pure functions over file contents; benchmark on large repos
  (existing bench-test pattern); document false-positive workflow prominently — a noisy
  security gate gets disabled by users, which is worse than a modest one.
- Accept: synthetic vulnerable fixtures (target: >90% catch on the fixture corpus, tracked in
  a test); zero findings on this repo itself; allowlist requires reasons.

**P4.3 — Review checklist generator** *(P1)*
- `specd review checklist <spec>`: deterministic transform of `design.md` sections +
  `tasks.md` contracts into a human checklist (one item per contract/file claim). No
  interpretation — extraction only.
- Accept: golden-free content assertions from fixtures (repo test style).

**P4.4 — Threat-model refresh** *(P0, docs+tests)*
- Update `SECURITY.md` + `docs/validation-gates.md` for every new exec surface (eval
  `command`, guardrail scans, deploy drivers, submit command, observe listener): env
  allowlist, sandbox, input validation, listener bound to localhost by default with token
  auth (reuse HTTP transport auth). Add adversarial tests: hostile rubric, hostile deploy
  config, hostile webhook payload (oversize, path traversal in correlation hints).
- Accept: `specd check` on a repo with hostile fixtures never executes them without explicit
  config; fuzz tests for new parsers (Go native fuzzing, like `host_caps_fuzz_test.go`).

### Phase 5 — Deployment & maintenance loop (weeks 17–20)

**P5.1 — Deploy driver runner** *(P0)*
- `.specd/deploy/<env>.json`: `{env, requiresGates: [...], steps: [{name, command,
  rollbackCommand, timeoutSeconds}], approvalRequired}`. New `specd deploy <spec> --env
  <env>`: refuses unless spec `complete` + required gates (eval/security/review) recorded
  green; runs steps sequenced, sandbox-recorded, appends to `deploy.jsonl` ledger;
  `specd deploy rollback` runs recorded inverse chain in reverse order.
  `--env production` additionally requires `specd approve --deploy` (human gate).
- Canary/blue-green live **in the user's commands** (their argo/k8s tooling); specd
  contributes gating, sequencing, evidence, rollback bookkeeping.
- Accept: e2e with fake driver scripts (testharness sandbox); mid-chain failure → recorded,
  rollback chain correct; production path impossible without approval record.

**P5.2 — Production observability inbound** *(P0)*
- Extend webhook machinery: `specd observe --listen` (localhost+token by default) accepts
  error payloads (schema-validated, size-capped); correlation is deterministic: payload file
  paths/stack frames matched against task `files:` lists and recent deploy ledger → append
  structured entry to `mid-requirements.md` (existing midreq machinery, severity mapped from
  payload) with correlation confidence facts.
- `specd observe correlate <file>` for batch/offline (CI pulls from Sentry, pipes in) — the
  listener is optional, the transform is the feature.
- Accept: correlation table-tested; malformed/hostile payloads rejected (P4.4 tests); midreq
  entries carry the evidence trail.

**P5.3 — Legacy ingestion workflow** *(P0)*
- `specd ingest new <slug> --path <dir>`: validates path, creates ingestion-flavored spec
  scaffold with a deterministic **inventory** (file list, sizes, package/module names from
  manifests — countable facts only) written to `inventory.json`. `specd-ingest` skill teaches
  the agent to reverse-engineer requirements/design/tasks from the code. New `ingest` gate:
  every inventory file is referenced by ≥1 requirement or explicitly waived with reason
  (coverage as a countable fact). Normal approve ratchet applies.
- This is the boot/enrich lesson applied: the binary inventories (countable), the agent
  understands (semantic), the gate enforces coverage (countable).
- Accept: inventory deterministic; coverage gate math tested; e2e: ingest a fixture module →
  agent-authored spec passes all gates.

**P5.4 — Migration spec packs** *(P1)*
- Ship packs under `internal/pack/embed_packs/`: `migrate-deps`, `modernize-tests`,
  `upgrade-go` — each a spec template + skill with pre-filled task DAG shape and rubrics.
  Runnable via P3.5 schedules.
- Accept: `specd init --pack <name>` produces gate-passing scaffolds.

**P5.5 — Feedback flywheel wiring + docs** *(P1)*
- The loop is composition, not new code: observe (P5.2) → midreq → human approve → spec →
  orchestrate → eval (P1.3) + review (P4.1) → submit (P3.4) → deploy (P5.1) → observe.
  Deliverables: e2e integration test walking the full loop with fake drivers, and
  `docs/flywheel.md` operator guide.
- Accept: the loop e2e test is in `make ci`.

### Phase 6 — Platform & ecosystem (weeks 21–24)

**P6.1 — Team harness sharing** *(P0)*
- `.specd/harness/` bundle (guardrails, eval suites, routing, deploy templates, roles) with
  `harness.json` manifest (name, version, provenance). `specd harness push/pull <git-url>`
  via stdlib-exec `git`; pull verifies manifest + refuses to overwrite local modifications
  without `--force`; imported `command` checks are **quarantined** (listed, disabled until
  explicitly enabled — supply-chain guard).
- Accept: round-trip push/pull e2e (local bare repo fixture); quarantine tests.

**P6.2 — Unified dashboard** *(P1)*
- Extend `specd serve`: unified view rendering conductor sessions, orchestrator waves, eval
  trends, cost, escalations from state/ledgers (server-rendered HTML + existing SSE; no JS
  framework, no new deps). `specd dashboard` alias with `--mode` filter.
- Accept: renders from fixtures deterministically; no network calls out.

**P6.3 — Spec pack registry (git-based)** *(P2)*
- `specd init --pack <git-url|name>`: named packs resolve via a registry index file that is
  itself a git repo (no hosted service in v0.2.0). Same quarantine rules as P6.1.
- Accept: remote pack e2e against local fixture repo; checksum pinning in lockfile.

**P6.4 — Release engineering** *(P0)*
- v0.2.0 release: CHANGELOG (Keep-a-Changelog discipline, breaking changes called out),
  `specd migrate` documented one-shot for config/state, goreleaser matrix unchanged,
  install.sh SHA256 flow re-verified, docs sweep (every new command in command-reference +
  user-guide; docs-parity tests enforce), benchmark comparison vs v0.1.x
  (`docs/agent-harness-baselines.md` refresh).
- Accept: `make ci` green; upgrade test: v0.1.x-initialized fixture repo runs every v0.2.0
  command correctly after `specd migrate`.

---

## Part III — Cross-cutting standards

### Testing strategy (per repo policy, TESTING.md)
- Every parser: byte-stable round-trip + fuzz. Every gate: table-driven pass/fail + hostile
  input. Every ledger: concurrent-append stress + crash-recovery. Every command: registry/
  help/docs parity + JSON contract test. Every exec surface: sandbox + env-scrub assertion.
  All race-clean, `-count=2` order-independent, coverage floor maintained. FakeClock for
  anything temporal; no golden files (content assertions).

### Security review cadence
- P4.4 threat-model refresh is a gate for the v0.2.0 release, and each phase that adds an
  exec or network surface (P1.3, P1.4, P3.4, P5.1, P5.2, P6.1, P6.3) lands with its
  adversarial tests in the same PR — not deferred to Phase 4.

### Documentation
- Each phase updates: command-reference, validation-gates, user-guide, agent-integration,
  mcp-guide (for new tools), and the embedded `AGENTS.md` template (new rules for eval/review/
  conductor discipline). Skills ship in the same PR as the machinery they teach.

### Success metrics (measurable with v0.2.0's own machinery)
| Metric | Target | Measured by |
|---|---|---|
| First-pass verify success | >85% | telemetry rollup per spec |
| Security fixture catch rate | >90% | P4.2 fixture corpus test in CI |
| Mode-switch friction | <30s, zero context loss | conductor e2e timing + ledger continuity test |
| Ingestion coverage | 100% inventory mapped/waived | `ingest` gate |
| Cost visibility | 100% tasks tier+cost attributed | routing/telemetry reconciliation test |
| Eval coverage | every completed spec has ≥1 recorded eval run | `eval` gate on completion (config-on) |
| Production correlation | every observed error → midreq entry with evidence | P5.2 tests |

### Top risks
1. **Scope** — the draft plan is a v1.0 disguised as v0.2.0. Mitigation: the §5.6 cut list,
   phase-shippable increments, P0-only fallback per phase.
2. **Gate fatigue** — eval/review/security gates that annoy get disabled. Mitigation:
   advisory-by-default severities where noted, allowlists-with-reasons, migrated repos
   default-off / new inits default-on.
3. **Exec-surface creep** — every new plugin point is an attack surface. Mitigation: single
   shared sandboxed-exec path (extend the custom-gate runner rather than new exec code),
   quarantine for imported harnesses, P4.4 adversarial suite.
4. **Parser complexity** — `micro:` and new artifacts strain the bespoke parsers. Mitigation:
   JSON for everything new except `tasks.md` micro-items; round-trip + fuzz mandatory.
5. **Host divergence** — conductor UX depends on hosts honoring briefs. Mitigation: MCP tool
   parity as the reference integration; SSE contract versioned; adapter conformance tests.

---

## Appendix A — Command surface added in v0.2.0

```text
specd eval <spec> [--suite <name>] [--trajectory]      specd conductor start|step|accept|reject|stop|replay <spec>
specd eval init <spec>                                 specd mode <spec> --set simple|orchestrated|conductor
specd trace append <spec> --tool ... --outcome ...     specd review <spec> · specd review checklist <spec>
specd check <spec> --security                          specd deploy <spec> --env <e> · specd deploy rollback
specd scaffold <spec>                                  specd observe --listen · specd observe correlate <file>
specd submit <spec> [--waves ...]                      specd ingest new <slug> --path <dir>
specd program schedule|tick                            specd harness push|pull <git-url>
specd report --cost|--conductor|--pr-summary(+)        specd dashboard [--mode ...]
specd migrate
```

## Appendix B — New artifacts

| Path | Format | Owner |
|---|---|---|
| `.specd/evals/<suite>.json` | JSON rubric | human/agent-authored, gate-validated |
| `.specd/guardrails.json` | JSON | human-authored constitution |
| `.specd/security/allow.json` | JSON (reasons mandatory) | human |
| `.specd/deploy/<env>.json` | JSON driver config | human |
| `.specd/harness/harness.json` | JSON manifest | shared bundle |
| `.specd/specs/<slug>/trajectory.jsonl` | JSONL ledger | CLI-owned, append-only |
| `.specd/specs/<slug>/conductor.jsonl` | JSONL ledger | CLI-owned, append-only |
| `.specd/specs/<slug>/deploy.jsonl` | JSONL ledger | CLI-owned, append-only |
| `.specd/specs/<slug>/review_report.md` | Markdown, structure-gated | agent-authored, human-approved |
| `.specd/specs/<slug>/evals/<suite>-<seq>.json` | JSON results | CLI-owned |
| `.specd/specs/<slug>/inventory.json` | JSON facts | CLI-owned (ingestion) |

---

*The harness expands; the split holds. The agent still reasons; specd still enforces — now
across the whole life cycle.*
