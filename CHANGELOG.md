# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Scheduled maintenance programs (`specd program schedule` / `specd program
  tick`, P3.5).** Register recurring maintenance in `program.json`
  (`schedule <name> --interval <seconds> --command <cmd> [--sandbox <backend>]`);
  a host scheduler invokes `program tick`, which runs each due schedule exactly
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

### Removed (breaking)

- **`boot` and `enrich` commands removed**, along with the two repo-global
  freshness gates (`specd check --boot` / `--enrich`), `boot.json`, and
  `enrich.json`. These performed repo *perception* and steering *authoring* inside
  the binary, violating the Foundational Split (the agent reasons; the harness
  enforces). `specd boot` and `specd enrich` are now unknown commands (exit 2).

- **13 deprecated legacy command aliases removed.** Each is now an unknown
  command (exit 2); the surviving flag-based home is listed where one exists:
  - `doctor` — no replacement. `specd init --repair` covers scaffold/pack
    repair, but `doctor`'s diagnostics (sandbox/container availability, MCP and
    host-registration health checks) are **not** preserved. This is a real
    capability loss, not a rename — see `SECURITY.md` for the updated
    threat-model note.
  - `dispatch` → `specd next --dispatch`
  - `program` → `specd status --program`
  - `validate` → `specd check --schema-only`
  - `schema` → `specd check --schema`
  - `replay` → `specd report --history`
  - `diff` → `specd report --diff`
  - `serve` → `specd report --serve`
  - `watch` → `specd report --watch`
  - `mode` → `specd status <slug> --set-mode` / `--recommend`, `specd new --orchestrated`
  - `migrate` — removed along with `specd init --migrate` (see below)
  - `update` — removed (see below)
  - `uninstall` — removed (see below)

- **`specd migrate config` / `specd init --migrate` removed.** Legacy JSON
  config is still *read* automatically; it is just no longer convertible to the
  current format via a built-in command.

- **`scripts/uninstall.sh` removed.** See `README.md`'s Uninstall section for
  the manual removal steps (the installer only ever placed a plain binary in
  `~/.local/bin`, with no directory or symlink to clean up).

- **`specd update` self-update command removed.** Reinstall via
  `scripts/install.sh --force` or your package manager instead.

### Added

- **`init` scaffolds a skill pack** under `.specd/skills/`: `specd-foundations`,
  `specd-steering`, `specd-requirements`, `specd-design`, `specd-tasks`, and
  `specd-execute`. The agent reads `specd-steering` to inspect the repo and author
  `product/structure/tech.md` + set `config.defaultVerify` itself — replacing
  `boot`/`enrich` with progressive-disclosure agent knowledge.

## [0.1.0] - 2026-06-14

First public release of `specd`, a spec-driven coding harness (stdlib-only Go, no
external dependencies).

### Added

- Core CLI with a unified command registry and consistent exit-code handling.
- `init` command: idempotent, marker-based `AGENTS.md` scaffolding and merge.
- `boot` command with boot-freshness validation gate.
- `enrich` command with `plan`, `apply`, and `status` sub-verbs.
- `dispatch` command plus verification records and an acceptance gate.
- `uninstall` command.
- Spec lifecycle: spec files, state with schema versioning, and CAS-guarded writes.
- Goroutine-safe spec locking with lock assertions and hardened slug validation.
- DAG engine for spec dependencies with cached regexes, preallocated slices, and benchmarks.
- Modular check gates with blocker utilities.
- Verified self-update flow with checksum (`SHA256SUMS`) verification.
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

[0.1.0]: https://github.com/0xkhdr/specd/releases/tag/v0.1.0
