# FINDINGS — Current `specd` vs the frozen v1 (`reference/`)

Date: 2026-07-05. Scope: full read of the current codebase (`internal/`, `docs/`,
`scripts/`, command palette) against the frozen v1 museum under `reference/`
(its command registry, changelog, and package layout). This document records
(A) where the current version stands, (B) what the old version had that the new
one does not, (C) defects and drift inside the current version itself, and
(D) a prioritized recommendation roadmap. Judgments respect the project's own
**subtractive bias**: not every removed feature is a gap — some cuts are the
point of the rewrite. Each gap below carries an explicit *port / adapt / skip*
verdict with reasoning.

---

## A. Current position

### A.1 What the new specd is

The rewrite is a deliberately lean re-derivation of the same thesis — *the
agent reasons, the harness enforces* — as a stdlib-only Go binary:

- **Pipeline**: requirements → design → tasks → evidence-gated execution,
  driven by `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json}`.
- **Command palette (17 live verbs + 1 deferred)**: `help`, `init`, `new`,
  `approve`, `midreq`, `decision`, `next` (with `--waves`/`--dispatch`),
  `status`, `task`, `check` (with `--security`), `verify` (with
  `--revert-on-fail`, `--sandbox`), `context` (with `--hud`), `memory`,
  `mcp`, `handshake` (`bootstrap|policy`), `brain` (`start|step|run|status`,
  `--authority`), `report` (`--pr|--metrics|--json`). `triage` is registered
  but **deferred** (prints a deferral notice, exits 0).
- **Gate registry (12 core + 1 opt-in)**: `task-ids`, `dependencies`, `dag`,
  `roles`, `files`, `verify`, `evidence`, `context-budget`, `ears`,
  `approval`, `sync`, `design`, plus the opt-in `security` gate.
- **Structural invariants intact**: atomic writes, CAS on `state.json`
  revision, reentrant per-spec lock, byte-stable tasks parser, `go:embed`
  templates, zero runtime dependencies, evidence pinned to a resolvable git
  HEAD with **no bypass flag**.
- **Orchestration**: minimal deterministic brain (`lease`, `decide`, `acp`
  ledger, `authority` fail-closed, `brakes`, `sense`, `session`, `driver`).
  Worker roles (scout / craftsman / validator / auditor) are delegated to
  **host-side agent definitions** (`.claude/agents/pinky-*.md`) instead of a
  `specd pinky` CLI protocol.
- **Config**: a small loader (agent selection, verify gate severity, context
  token budget, orchestration enable/model, memory promotion threshold).
- **Docs**: `README.md` + six docs (`concepts`, `user-guide`,
  `command-reference`, `validation-gates`, `agent-integration`,
  `contributor-guide`).

### A.2 Honest characterization

The core loop — plan, gate, execute against the DAG frontier, verify with
recorded evidence, complete — is **complete and sound**, and structurally
cleaner than v1 (one handler per verb, one gate registry, no 27 KB `init.go`).
What was shed is almost everything *around* the loop: post-completion
lifecycle (review, submit, deploy, observe), quality instrumentation (eval,
cost telemetry, analytics), operability (dashboard, migrate, version, rich
reports), and ecosystem surface (packs, harness sharing, host-config
snippets, phase-compatibility metadata). Some of that shedding is healthy;
some of it removed real enforcement and operability value. Section D sorts
which is which.

---

## B. Gap analysis — old features absent from the current version

The v1 registry had **32 verbs**; the current palette has 17. Feature-by-
feature, with a verdict.

### B.1 Enforcement & evidence gaps (highest value — these guarded the core promise)

| # | Old capability | What it did | Verdict |
|---|---|---|---|
| 1 | `verify --criterion <r>.<n> --status pass\|fail --evidence` | Per-acceptance-criterion proof records, distinct from task-level verify. Closed the gap between "task verify passed" and "requirement satisfied". | **Port.** Cheap (one flag path + one record shape) and it materially strengthens the evidence story. Today an approved requirement's criteria are never individually attested. |
| 2 | Phase/mode compatibility metadata (`PhaseCompatibilityMeta`, `ModeCompatibilityMeta`) — commands declared which statuses/phases they were valid in; dispatch could fail closed on out-of-phase calls | Deterministic guard against an agent running execution verbs during planning, etc. | **Port.** The current `Command` struct is name/usage/description/flags only. This metadata was *harness enforcement*, exactly the project's thesis, and it also drives better MCP tool descriptions for free. |
| 3 | Exit-code metadata + enum-annotated flags in the registry | Machine-readable contract (`help --json`) — exit meanings, flag enums, defaults, examples. | **Port.** Low cost, high leverage: the MCP server and agent role prompts can consume it instead of prose docs. |
| 4 | Security suite: entropy-based `secrets` scanner with reasoned allowlist (`.specd/security/allow.json`), `injection` heuristics, `slopsquat` (manifest parsing + edit distance vs embedded popular-package list), per-scanner `off/warn/error` config, findings recorded in `state.security` | Real deterministic supply-chain and secret hygiene. | **Port (adapt).** The current security gate is a handful of hardcoded literal substring patterns — it will neither catch a real key nor a typosquatted dependency. Either upgrade it to the v1 suite's design or clearly document it as a placeholder; today it gives false confidence. |
| 5 | Review workflow (`specd review`, `review checklist`, opt-in `config.review.required` gate blocking verifying→complete without a fresh `approve`-verdict report) | Structured adversarial review as a gate, extraction of checklist from design/tasks. | **Port (staged).** The reviewer *role prompt* already exists host-side; what's missing is the deterministic half — the scaffolded report format and the gate that refuses completion without it. Port the gate + scaffold; skip the fancy checklist extraction initially. |
| 6 | Auto-escalation engine (`specd orchestrate status/resume --override`) — deterministic rules over countable facts (`verify-fail`, `retry-exhausted`, `blocker`, `cost-over-budget`), human-only clearing | Turned repeated failure into a mandatory human checkpoint. | **Adapt.** Full engine is heavy, but the core idea — after N failed verifies on a task, block further attempts until a human records an override — is a small, high-value ratchet. Implement as a gate + a `task ... --override --reason` path. |

### B.2 Lifecycle-completeness gaps (the loop currently ends at "complete")

| # | Old capability | Verdict |
|---|---|---|
| 7 | `submit` — validate all gates green, generate deterministic PR summary, stream to operator-configured `config.submit.command` via sandboxed exec | **Port.** This is the natural terminal verb of the pipeline and it embeds no git/GitHub logic (operator supplies the command). Current `report --pr` produces the summary but nothing consumes it under gate enforcement. |
| 8 | `deploy` / `deploy rollback` — evidence-gated deploy driver over `.specd/deploy/<env>.json`, append-only `deploy.jsonl`, human approval for production | **Defer.** Well-designed but a large hostile-input surface (step schemas, timeouts, rollback chains). Only port when a real user needs post-merge enforcement. Record the deferral as a decision. |
| 9 | `observe correlate` (+ optional loopback listener) — production error → spec attribution → evidenced mid-requirement | **Defer.** The flywheel is elegant but presumes deploy + ledger infrastructure (item 8). Same trigger condition. |
| 10 | `eval` / `promote` / `--prototype` lifecycle — deterministic rubric engine (`artifact_present`, `regex`, `trajectory`, sandboxed `command` checks), `eval init` from approved requirements, `eval trend`, prototype specs that cannot complete until promoted with evidence | **Adapt (partial port).** The rubric engine minus `trajectory` is small and reuses the existing sandboxed exec path. Prototype/promote is genuinely useful for spike work — currently a spike must either fake full planning or live outside specd. Port prototype + a minimal rubric; skip trend analytics. |
| 11 | `ingest` — legacy codebase inventory (`inventory.json`, countable facts only) + opt-in coverage gate | **Skip for now.** Brownfield adoption story matters eventually, but it is a separable product surface. Record as a decision. |
| 12 | `conductor` — interactive micro-task sessions with append-only ledger and mandatory rejection reasons | **Skip.** Overlaps with what interactive agent harnesses (Claude Code itself) already provide natively. The one idea worth keeping: *mandatory rejection reasons as a training signal* — capturable later via `midreq`. |

### B.3 Operability & observability gaps

| # | Old capability | Verdict |
|---|---|---|
| 13 | `version` verb (with `--json`, ldflags-injected) | **Port immediately.** Trivial, and a binary you cannot version-identify in the field is an operational liability. v1 even had a goreleaser pipeline injecting it. |
| 14 | `migrate` + state schema versioning (v5→v6 path, additive-blocks report, idempotent) | **Port the discipline, not the tool.** Current `state.json` needs an explicit `schemaVersion` field and a documented forward-migration policy *now*, while there are no users — retrofitting versioning after adoption is far more painful. The `migrate` verb itself can wait until there are two schema versions. |
| 15 | Task/worker cost telemetry — `task --tokens --cost`, pinky `--host-tokens --host-cost --duration-ms` (stored, never computed) | **Port.** Tiny (annotation fields on existing records) and it is the only way `report --metrics` can ever say anything about cost. Aligns with the "countable facts only" doctrine. |
| 16 | Rich reports — `--format md\|html\|prometheus`, `--serve` (SSE live dashboard), `--watch`, `--history` (audit replay), `--diff` (artifacts between git refs), `--conductor` analytics | **Adapt.** Current `report` has `--pr`/`--metrics`/`--json`. Worth adding: `--history` (replay from the ledgers that already exist) and a Prometheus textfile view (cheap, pure function of state). Skip HTML/serve/SSE. |
| 17 | `dashboard` — unified loopback web dashboard | **Skip.** Big surface, zero enforcement value. `status --json` + external tooling covers it. |
| 18 | `status --program` — cross-spec dependency links (`link/unlink`, cycle-refused), program frontier, `schedule`/`tick` host-driven maintenance | **Adapt (link/unlink only).** Cross-spec dependencies are a real modeling gap: today two specs cannot express ordering, so a multi-spec effort loses the frontier guarantee at the program level. Port `link/unlink` + program frontier read. Skip schedules/tick (host cron + `check` covers it). |
| 19 | Brain lifecycle depth — `pause/resume/cancel/checkpoint`, `--ledger`, `--compact`, `directive` (host answers a worker query), session recovery (v1 had 17 KB of recovery tests) | **Port (staged).** Current brain is `start/step/run/status` only. A controller that cannot be paused, cancelled, or resumed after a crash is not safe to leave driving waves. Minimum: `cancel`, crash-safe `resume`, and a checkpoint on every step. `directive` can follow. |
| 20 | `pinky` CLI protocol — `claim/update/report/block/release` with attempt numbers, changed-files, git-head, verification refs | **Adapt.** The current design (host agents + ACP ledger) is defensible and lighter. But worker reports currently lack the v1 rigor (attempt-numbered, git-HEAD-stamped claim/report records). Either extend the ACP ledger records to carry that, or accept weaker orchestration evidence and document it. |

### B.4 Ecosystem & integration gaps

| # | Old capability | Verdict |
|---|---|---|
| 21 | `init` depth — `--repair/--refresh/--force` (marker-based managed-asset maintenance), `--dry-run`, multi-host agent detection (codex, claude-code, cursor, antigravity, vscode), `--scope project\|global` with consent rules | **Port `--repair`, `--refresh`, `--dry-run`.** Idempotent re-init with drift repair is the difference between a scaffold and a managed asset; users *will* hand-edit `AGENTS.md`. Multi-host detection: adapt to whatever hosts are actually targeted. |
| 22 | `mcp --config <host>` (ready-to-paste snippets), `--root`, `--spec` pinning | **Port.** Small and removes the highest-friction step of adoption. |
| 23 | `check --schema` / `--schema-only` — embedded JSON Schema for `state.json` | **Port.** Cheap, pairs with item 14 (schema versioning), and gives external tools a validation contract. |
| 24 | Spec packs (`init --pack`, `--registry`, SHA256-pinned `pack.lock`, declarative-only manifests) | **Skip for now.** A distribution ecosystem before there are users is inverted priorities. The security design (pinning, no executable hooks) is worth reusing verbatim when the time comes. |
| 25 | `harness push/pull/list/enable` — team policy sharing with quarantine of executable artifacts | **Skip.** Same reasoning; the quarantine pattern is the part to remember. |
| 26 | Handshake depth — command-schema digest, config digest drift detection (`--expect-config-digest`) | **Adapt.** Current handshake exists but without digests. Digest-pinning lets an agent detect that its cached palette is stale — cheap and on-thesis. |

---

## C. Findings in the current version itself (independent of v1)

These are defects/drift found while auditing, ordered by severity.

1. **CLAUDE.md documents infrastructure that does not exist.** It instructs
   running `./scripts/test-lint.sh`, `./scripts/docs-lint.sh`,
   `./scripts/stress*.sh`, and asserts `docs/CHEATSHEET.md` mirrors
   `docs/command-reference.md`. None of these files exist — `scripts/`
   contains only `audit-progress.sh`, `regress-all.sh`, `regress-domains.sh`,
   `regress-lint.sh`, `verify-progress.sh`, and `docs/` has no `CHEATSHEET.md`.
   The project's own "Docs sync" invariant is violated by its onboarding file.
   **Fix now:** either create the missing lint scripts + cheatsheet, or edit
   CLAUDE.md/README to match reality. (This also silently weakens CI claims —
   a contributor "running the gates before pushing" runs nothing.)
2. **`triage` is a permanent deferral with no decision record.** A registered
   verb that prints a deferral notice is fine as a transition state, but there
   is no recorded decision on whether the extended-loop triage tier is coming
   or cut. Per the subtractive-bias rule ("cut or defer *and record the
   decision*"), record it — or remove the verb.
3. **No `version` verb and no release pipeline.** `reference/` had
   `.goreleaser.yml` + ldflags version injection; the current repo has
   neither. Any bug report against a deployed binary is currently
   unattributable. (Same as B.13/B.14 but it is also a *current* hygiene gap.)
4. **Security gate is cosmetic.** Hardcoded literal substrings; no entropy
   check, no allowlist, no manifest awareness, no severity config. An opt-in
   gate that cannot catch real findings is worse than absent, because
   `check --security` passing reads as assurance. (See B.4.)
5. **`state.json` carries no schema version.** The CAS revision counter
   guards concurrency, not evolution. First schema change after adoption
   becomes a breaking event with no migration hook. (See B.14.)
6. **Brain has no abort/recovery path.** `start/step/run/status` with no
   `cancel`, no documented crash-recovery semantics for a half-run session,
   and no checkpointing. The lease mechanism bounds the damage but the
   operator has no verb to act with. (See B.19.)
7. **Config surface is ahead of enforcement.** The loader accepts
   orchestration and severity blocks, but several knobs have no consumer yet
   (e.g., orchestration model selection). Unconsumed config is silent
   misconfiguration waiting to happen — validate-and-reject unknown keys, or
   wire them.
8. **Command metadata is too thin to drive the integration surfaces.** Help,
   MCP tool descriptions, and role prompts each independently restate command
   semantics; v1 generated all three from one `CommandMeta`. Divergence risk
   grows with every verb added. (See B.2/B.3.)

---

## D. Recommendation roadmap

Ordered; each tier is shippable independently. Verdicts above give the detail.

### Tier 0 — hygiene, this week (no design work)

1. Fix CLAUDE.md/reality drift: create `docs/CHEATSHEET.md` + `docs-lint.sh`
   + `test-lint.sh`, or rewrite CLAUDE.md/README to match what exists (C.1).
2. Add `specd version` (ldflags-injected) and restore a goreleaser pipeline
   from the v1 config (C.3, B.13).
3. Add `schemaVersion` to `state.json` + a load-time forward-migration hook,
   even if the only version is 1 (C.5, B.14).
4. Record a decision (ADR via `specd decision` on the meta-spec, or docs) for
   every deliberate skip in section B — `triage`, conductor, dashboard,
   packs, harness, ingest, deploy/observe (C.2).

### Tier 1 — enforcement completeness (the core-promise gaps)

5. Rich command metadata: exit codes, flag enums, examples, phase/mode
   compatibility — enforced fail-closed at dispatch and consumed by help
   `--json` and MCP (B.2, B.3, C.8).
6. Per-criterion verify evidence (`verify --criterion`) (B.1).
7. Real security suite: entropy secrets + reasoned allowlist, slopsquat via
   embedded package list, per-scanner severity config (B.4, C.4).
8. Escalation ratchet: N failed verifies on a task ⇒ block until human
   `--override --reason` (B.6).
9. Brain safety: `cancel`, crash-safe `resume`, per-step checkpoint (B.19, C.6).

### Tier 2 — lifecycle completion & operability

10. `submit` with operator-configured command over sandboxed exec (B.7).
11. Review gate: scaffolded `review_report.md` + opt-in completion block (B.5).
12. Cost/duration telemetry annotations on task and ACP records (B.15).
13. `mcp --config <host>` snippets; `init --repair/--refresh/--dry-run`
    (B.21, B.22); `check --schema` (B.23); handshake config digests (B.26).
14. Cross-spec `link/unlink` + program frontier view (B.18).
15. `report --history` and Prometheus textfile format (B.16).

### Tier 3 — only on demonstrated demand (record the decision now)

16. Eval rubric engine + prototype/promote lifecycle (B.10).
17. Deploy driver + observe correlation (the post-merge flywheel) (B.8, B.9).
18. Packs/registry, harness sharing (reuse v1's pinning + quarantine design),
    ingestion, dashboard, conductor (B.11, B.12, B.17, B.24, B.25).

### What NOT to bring back

- `dashboard`, `report --serve` SSE machinery — UI surface, no enforcement.
- `conductor` — duplicated by interactive agent hosts.
- `status --program schedule/tick` — host cron does this.
- The v1 mega-`init` (27 KB) as-is — port its capabilities, not its shape.
- Anything that reintroduces the pre-release `boot`/`enrich` mistake (repo
  perception inside the binary) — v1's own changelog flags this as a
  Foundational Split violation.

---

## E. Bottom line

The rewrite kept the right spine: deterministic gates, evidence-pinned
completion, atomic state, wave frontier, zero dependencies. The material
losses are (1) enforcement metadata and per-criterion evidence, (2) a real
security suite, (3) brain lifecycle safety, (4) versioning/migration
discipline, and (5) the terminal `submit` step that makes the pipeline end in
an enforced artifact instead of a report nobody consumes. The self-inflicted
issue is documentation that describes tooling that does not exist. Tier 0 and
Tier 1 close everything with genuine enforcement value; most of the remaining
v1 surface deserves the recorded-skip treatment the project's own subtractive
bias demands.
