# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-03

### Added

- **Team harness sharing (`specd harness`, P6.1).** The configured policy —
  guardrails, deploy templates, roles, routing — becomes a versioned, shareable
  team asset under `.specd/harness/` with a SHA256-pinned `harness.json`
  manifest. `specd harness push <git-url>` bundles and pushes (monotonic
  version); `pull` clones, verifies every pinned checksum, refuses a version
  downgrade or a locally-modified overwrite without `--force`, and — the
  load-bearing supply-chain guard — **quarantines** every imported executable
  `command` artifact: copied to `.specd/harness/quarantine/`, listed, never
  installed until `specd harness enable <path>`, which is recorded in the harness
  decision log. Sharing rides the same hardened git-exec discipline as the git
  state backend (scrubbed env, transport allowlist, remote-URL validation). No
  hosted services. Hostile manifest/URL fixtures ship in the same PR.
- **Unified dashboard (`specd dashboard`, P6.2).** A read-only, dependency-free,
  loopback-bound project dashboard rendering every spec's status, orchestrator
  waves, conductor sessions, eval trends, cost attribution, escalations, and the
  shared harness bundle — assembled purely from local state and ledgers with zero
  outbound network. `--mode <all|conductor|orchestrator|cost|eval>` filters
  panels; the existing SSE stream drives live updates. A project-wide alias over
  the `serve` machinery; the render is a pure function of on-disk state
  (snapshot-tested).
- **Pack registry (`specd init --pack <name> --registry <git-url>`, P6.3).**
  Named packs resolve without a hosted service: the registry index is itself a
  git repo holding a `registry.json` that maps a pack name to a pinned
  `{url, sha256}`. Resolution clones the index over the hardened git-exec path,
  verifies the referenced pack's SHA256 fail-closed, and pins the result in
  `.specd/pack.lock`; a later resolution whose content disagrees with the lock is
  a hard failure (mutated-registry guard). Pack manifests remain declarative-only
  (no executable hooks) — the quarantine equivalent for packs.
- **Migration tooling (`specd migrate`, P6.4).** Idempotent one-shot that moves a
  v0.1.x project onto v0.2.0: it rewrites every spec's `state.json` at the current
  schema version (the v5→v6 migration is otherwise silent on first load) and
  reports which additive policy blocks (guardrails, routing, eval/review gates)
  are available to adopt. It never writes policy content, so a migrated repo keeps
  the new gates default-off (backward-compat invariant 9). Running it twice is a
  no-op.
- **Deploy driver runner (`specd deploy`, P5.1).** Evidence-gated deploy past
  `complete`: `.specd/deploy/<env>.json` declares `steps` (each with a
  `command`, optional `rollbackCommand`, and a mandatory `timeoutSeconds`) plus
  `requiresGates` and `approvalRequired`. `specd deploy <slug> --env <env>`
  refuses unless the spec is complete, every required gate is recorded green,
  and — for a production env or an approval-required plan — a human deploy
  approval exists (`specd approve <slug> --deploy --env <env>`). Steps run
  sequenced through the shared sandboxed exec path with a scrubbed env; every
  result is appended to the append-only `deploy.jsonl`. `specd deploy rollback`
  replays the recorded inverse chain (successful steps, reverse order); a failing
  rollback step halts and exits 3. No CD logic is embedded. Deploy configs are
  hostile input (strict schema, mandatory/bounded timeouts, env-name traversal
  rejection) with adversarial tests in the same PR.
- **Production observability inbound (`specd observe`, P5.2).** `observe
  correlate <payload.json>` reads a schema-validated, size-capped error payload,
  deterministically attributes it to a spec by matching stack-frame files against
  task `files:` contracts (falling back to the recent deploy ledger), and appends
  an evidenced entry to that spec's `mid-requirements.md` — gating high/critical
  impact for human approval, exactly like `specd midreq`. `observe --listen`
  starts an optional loopback-only, bearer-token-authed HTTP receiver applying the
  same transform. Frame paths that are absolute or traverse the repo are rejected.
- **Feedback flywheel (P5.5).** observe → midreq → approve → … → deploy → observe
  is now a composed, fake-driver end-to-end test in the suite, plus the
  `docs/flywheel.md` operator guide.
- **Legacy ingestion (`specd ingest`, P5.3).** `ingest new <slug> --path <dir>`
  validates the path (no traversal outside the repo), writes a deterministic
  `inventory.json` (sorted file list, sizes, and manifest-derived module names via
  stdlib — countable facts only; the binary never reads legacy semantics), and
  scaffolds an ingestion-flavored spec. Scoping respects `.gitignore` via `git
  ls-files` (`--include-ignored` forces a bounded walk with default excludes). The
  opt-in `ingest` gate (`gates.ingest`) flags any inventoried file that no
  requirement references and no reasoned waiver excuses. Ships the `specd-ingest`
  skill (reverse-engineering workflow) and fuzzed manifest parsers.
- **Migration spec packs (P5.4).** `migrate-deps`, `modernize-tests`, and
  `upgrade-go` built-in packs (`specd init --pack <name>`) each ship a steering
  file, a task-DAG template, and a V5 eval rubric; runnable on V7 schedules.
- **Scheduled maintenance programs (`specd status --program schedule` /
  `specd status --program tick`, P3.5).** Register recurring maintenance in `program.json`
  (`--program schedule <name> --interval <seconds> --command <cmd> [--sandbox <backend>]`);
  a host scheduler invokes `status --program tick`, which runs each due schedule exactly
  once through the shared sandboxed exec path with a scrubbed env. specd never
  daemonizes — the claim is CAS-guarded under the program lock, so a
  double-invoked tick is idempotent (nothing due runs twice). Ships the
  `specd-maintenance` skill.
- **ACP inter-role handoff (P3.3).** Mission payloads and briefs now carry an
  optional `tier` and `handoff {from, reason, artifacts}` (e.g. scout → craftsman),
  schema-versioned and validated (known origin role, non-empty reason). The
  fields are omitempty and excluded from the dispatch digest, so a fresh dispatch
  is byte-identical to the pre-handoff format. Maps to the A2A handoff concept.
- **Reviewer role + `specd-review` skill.** The read-only adversarial reviewer
  role is now a scaffolded `.specd/roles/reviewer.md` template (seeds a reviewer
  sub-agent), and the `specd-review` skill documents the report structure, the
  `review checklist`, and the `review` gate.
- **Threat-model refresh (P4.4).** SECURITY.md now documents every v0.2.0 exec
  surface — eval rubric commands, executable guardrails, `submit`, maintenance
  `tick`, and the security scanners — and how each is scrubbed/sandboxed/opt-in
  (deploy drivers + observe listener tracked as pending until V9).
- Add state schema v6 with v5 migration support and optional `evals`, `routing`, `conductor`, and `escalation` blocks.
- **Auto-escalation engine (`specd orchestrate`).** Deterministic, opt-in
  (`config.escalation.enabled`) rule set over countable facts — `verify-fail`,
  `retry-exhausted`, `blocker`, `cost-over-budget`, `complexity` — evaluated on a
  failed verify. When a rule fires the task's `state.escalation` is recorded and a
  conductor handoff is *recommended*, never auto-switched. `specd orchestrate
  <slug> status` surfaces the escalation; `resume --override` is the human
  override that clears it. Off by default (migrated repos unaffected).
- **Security gate suite (`specd check --security`).** Deterministic, stdlib-only
  scanners over working-tree changed files: `secrets` (entropy + known formats,
  allowlist with mandatory reasons at `.specd/security/allow.json`), `injection`
  (SQL-concat / exec-interpolation heuristics, advisory by default), and
  `slopsquat` (manifest parsing + edit distance vs an embedded popular-package
  list — no network, no CVE DB). Findings are recorded in `state.security` and
  rendered in the PR summary. Each scanner is `off`/`warn`/`error` per
  `config.security.*`; advisory findings never fail the command.
- **Review workflow (`specd review`).** Scaffolds a structured `review_report.md`
  (Summary, Bugs, Security, Hallucinated Dependencies, Style, Verdict) and prints
  the read-only adversarial reviewer brief; `review checklist` deterministically
  extracts a review checklist from `design.md` + `tasks.md`. New opt-in review
  gate (`config.review.required`) blocks `approve` verifying→complete until a
  fresh, structurally-valid report with an `approve` verdict exists — human
  approval stays final. Off for migrated repos.
- **Batch PR submission (`specd submit`).** Validates all configured gates are
  green, generates the deterministic PR summary (now including eval / security /
  escalation sections), and streams it on stdin to the operator-configured
  `config.submit.command` run through the shared sandboxed exec path with a
  scrubbed env. No git/GitHub logic embedded; `--dry-run` prints without
  executing. A gate violation or non-zero command exit fails with no partial state.
- **Eval framework (`specd eval`, `specd promote`).** Deterministic rubric
  engine with `artifact_present`, `regex`, `trajectory`, and sandboxed `command`
  check kinds; `specd eval <slug> init` compiles a rubric skeleton from approved
  requirements; `specd eval <slug> trend` reports score deltas and failure
  clustering. New opt-in `config.gates.eval=required` blocks completion until a
  passing rubric run is recorded (off by default, including migrated repos).
  Ships the `specd-eval-author` skill.
- **Prototype lifecycle.** `specd new --prototype` creates a spec that skips the
  design/tasks planning gates but can never reach `complete`; `specd promote`
  converts it to a full spec after a passing eval (evidence mandatory).
- **Conductor mode (`specd conductor`).** Interactive micro-task sessions
  (`start|step|accept|reject|stop|replay|switch|status`) over an append-only
  `conductor.jsonl` ledger, with `micro:` task syntax in `tasks.md`;
  `reject --reason` is mandatory (the training signal). Micro-approval never
  bypasses the `verify:` evidence gate. Exposed as the `specd_conductor` MCP tool.
- **Context HUD.** `specd context <slug> --hud` renders the deterministic load
  list with measured byte/approx-token cost and the active mode/tier.
- **Rejection analytics.** `specd report <slug> --conductor` clusters conductor
  rejection reasons (exact string + count) from the ledger.

## [0.1.0] - 2026-07-03

First public release of `specd`, a spec-driven coding harness (stdlib-only Go, no
external dependencies). The binary enforces; the agent reasons — repo perception and
steering authoring live in the agent-facing skill pack, not in the CLI.

### Added

- Core CLI with a unified command registry and consistent exit-code handling.
- `init` command: idempotent, marker-based `AGENTS.md` scaffolding and merge, and
  scaffolds a skill pack under `.specd/skills/` (`specd-foundations`,
  `specd-steering`, `specd-requirements`, `specd-design`, `specd-tasks`,
  `specd-execute`). The agent reads `specd-steering` to inspect the repo and author
  `product/structure/tech.md` and set `config.defaultVerify` itself.
- Spec lifecycle: spec files, state with schema versioning, and CAS-guarded writes.
- Goroutine-safe spec locking with lock assertions and hardened slug validation.
- DAG engine for spec dependencies with cached regexes, preallocated slices, and benchmarks.
- Modular check gates with blocker utilities.
- Security model documentation and hardening review.
- Install guide with fallback to build from source when no release binary is available.
- `goreleaser` release pipeline (linux/darwin/windows, amd64/arm64) with version
  injected via ldflags.
- Comprehensive test harness: deterministic `FakeClock`, spec builder, assertions,
  and end-to-end lifecycle tests.
- Hardened CI/testing pipeline.

### Notes

- Migrated from the original TypeScript implementation to Go.
- Renamed the `SPECd_JSON` environment variable to `SPECD_JSON`.
- **Supersedes the 2026-06-14 pre-release build carrying the same `v0.1.0` tag.**
  That early build shipped `boot`, `enrich`, `dispatch`, `uninstall`, and a
  self-`update` command; all were removed before this final cut. `boot`/`enrich`
  performed repo perception and steering authoring inside the binary, violating the
  Foundational Split — that work now lives in the skill pack above. `dispatch` moved
  to `specd next --dispatch`; the self-`update` and `uninstall` commands and
  `scripts/uninstall.sh` were dropped (reinstall via `scripts/install.sh --force`).

<<<<<<< HEAD
[Unreleased]: https://github.com/0xkhdr/specd/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/0xkhdr/specd/compare/v0.1.0...v0.2.0
=======
[Unreleased]: https://github.com/0xkhdr/specd/compare/v0.1.0...HEAD
>>>>>>> f918e74739408607887e1161609e15734d90004d
[0.1.0]: https://github.com/0xkhdr/specd/releases/tag/v0.1.0
