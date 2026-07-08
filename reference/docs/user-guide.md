# User Guide

Using `specd` inside a target repository — from install through a closed spec.
For the *why*, read [Concepts](./concepts.md); for command-level detail, the
[Command Reference](./command-reference.md).

## Contents

1. [Installation & setup](#installation--setup)
2. [The spec lifecycle](#the-spec-lifecycle)
3. [Writing spec artifacts](#writing-spec-artifacts)
4. [Task execution & evidence](#task-execution--evidence)
5. [Sharing, dashboards & migration](#sharing-dashboards--migration)
6. [Troubleshooting](#troubleshooting)

---

## Installation & setup

### Quick install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash
```

After install, restart your shell (or `source ~/.bashrc` / `source ~/.zshrc`).

### Install options

```bash
# Force reinstall / upgrade
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --force

# Pin a specific version
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --version 0.1.0
```

### Uninstall / update

```bash
# Update: reinstall in place with --force
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --force

# Uninstall: removal is manual — delete the installed binary (there is no uninstall script)
rm "$(command -v specd)"
```

**Upgrading a v0.1.x project to v0.2.0.** After replacing the binary, run
`specd migrate` once inside the project root. It rewrites each spec's on-disk
state to the current schema (the v5→v6 migration is otherwise silent on first
load) and lists the additive v0.2.0 config blocks (guardrails, deploy, routing,
eval, resilience) you can adopt. It never writes policy content — every new gate
stays default-off — and is idempotent, so re-running it is a no-op. See
[Sharing, dashboards & migration](#sharing-dashboards--migration).

- Linux, macOS, or Windows (amd64 / arm64)
  - *Windows note*: Windows users should reinstall with the installer command above instead of relying on in-place binary replacement. Verification commands also require `sh` or `bash` (e.g. from Git for Windows) in the `PATH` since verify execution uses `-c`.
- Git (optional — tarball fallback available)
- **Zero runtime dependencies** — single binary

### Initialize a project

The golden path is one command in your project root:

```bash
cd <your-project>
specd init --agent auto
```

This scaffolds `.specd/` (default templates, steering files, roles, skills,
`config.yml`) and merges `AGENTS.md`, **then** detects your coding agent, installs
project-scoped MCP registration for it, and verifies the integration with an
in-process MCP handshake. A rerun on a healthy project makes **zero byte changes**.

#### Choosing how hosts are configured

| Invocation | Behavior |
|---|---|
| `specd init --agent auto` | Detect hosts; in an interactive terminal, configure the **one** unambiguous host. In CI / non-TTY or when several are detected, scaffold only and report suggested actions (no mutation). |
| `specd init --agent claude-code --yes` | Configure one named host, non-interactively. |
| `specd init --agent all --yes` | Configure **every** detected supported host. |
| `specd init --agent none` | Scaffold only; touch no host config. |
| `specd init --agent codex --dry-run --json` | Print exact proposed mutations/commands as JSON; write nothing. |

**Consent & scope rules:**

- **Project scope is the default.** Generated config lives under your repo
  (`.mcp.json`, `.cursor/mcp.json`, `.agents/mcp_config.json`, …). `.agents/` is
  intentionally tracked in VCS so Antigravity config stays with the repo.
- **Global host config is never modified** without `--scope global` **and** explicit
  consent (`--yes` or an interactive confirmation).
- `--yes` authorizes only documented, non-destructive, project-scoped changes.
- Existing host config must parse before any mutation; specd fails closed and creates
  a timestamped backup before changing an existing file.
- Unrelated MCP servers and settings in a host config are always preserved.

Supported managed adapters (auto-detect + install): **claude-code, codex, cursor,
antigravity, vscode**. **claude-desktop** ships deterministic manual snippets only
(`specd mcp --config <host>`). See
[mcp-guide.md](mcp-guide.md) and [agent-harness-compat.md](agent-harness-compat.md)
for the full host matrix and trust boundaries.

#### Verify and repair

```bash
specd init --repair          # scaffold + MCP server + host-registration health, with remedies
specd init --repair    # apply safe, project-scoped, specd-owned repairs only
specd init --repair   # restore missing managed files without overwriting your edits
specd init --refresh  # update only specd-managed assets and AGENTS.md marker sections
```

#### Air-gapped / manual setup

No network and no host CLI is fine — scaffold offline, then paste a snippet:

```bash
specd init --agent none
specd mcp --config claude-code   # ready-to-merge config for the chosen host
```

### Bootstrap project context (recommended)

After `init`, seed the steering constitution from the real repository. This is
agent work — there is no detection command. Read
`.specd/skills/specd-steering/SKILL.md`, then:

1. Inspect the repo yourself — manifests (`go.mod`, `package.json`, …), the
   directory tree, `README`/`CONTRIBUTING`/`docs/`, and CI files.
2. Author `.specd/steering/product.md`, `structure.md`, and `tech.md`, grounding
   every claim in a file you actually read — never invented.
3. Set `defaults.verify_command` in `.specd/config.yml` to the real test command you
   found (e.g. `go test ./...`, `npm test`, `pytest`).

The harness performs **zero inference**: it scaffolds the skill pack at `init` and
enforces specs at `check`. Perceiving the stack and authoring steering is the
agent's job (the Foundational Split).

### Global vs Project Config

`specd` reads config as embedded defaults → global YAML → project YAML → `SPECD_*` environment overrides. Config is YAML-only as of v0.2.0 (`config.yml`). Use global config for personal defaults and project config for repository policy:

```yaml
# ~/.config/specd/config.yml
version: 2
defaults:
  verify_command: "go test ./..."
  report_format: "md"
roles:
  subagent_mode: "inline"
```

```yaml
# .specd/config.yml
version: 2
defaults:
  verify_command: "make test"
verify:
  sandbox: "bwrap"
orchestration:
  enabled: true
  approval_policy: "planning"
  max_workers: 4
```

Project values override global values field-by-field; lists replace lower-layer lists. `SPECD_DEFAULT_VERIFY`, `SPECD_VERIFY_SANDBOX`, and orchestration env overrides win last for one process. The runtime loader no longer reads legacy JSON config — a present `.specd/config.json` is ignored, not merged. Convert an existing one to `.specd/config.yml` with `specd migrate`, which renders it to YAML v2. Runtime files such as `state.json`, `.specd/program.json`, `session.json`, and integration state remain JSON.

---

## The spec lifecycle

A **spec** is a modular directory representing a single feature, task, or bugfix.
Its lifecycle has 5 phases driven by a status machine.

### Execution mode (Base vs Orchestrated)

Each spec runs in one of two modes, recorded per spec in its `state.json`:

- **Base** (default) — you drive every step yourself (`specd next` → implement →
  `specd verify`). This is the default for every new spec.
- **Orchestrated** — the Brain/Pinky multi-agent layer may drive the spec. Opt in
  explicitly: `specd new <slug> --orchestrated`, or `specd status <slug> --set
  orchestrated` on an existing spec (requires the project to have orchestration
  enabled via `specd init --orchestration …`).

Inspect or change a spec's mode with `specd status <slug>`. After your tasks are
planned, `specd status <slug>` prints a deterministic, advisory verdict
on whether orchestration would pay off — it's a suggestion, you decide. Base is
always the default and orchestration is never enabled automatically.

### Status → phase mapping

| Spec status (`state.json`) | Derived phase | Primary activities |
|---|---|---|
| `requirements` | `analyze` | Author EARS requirements in `requirements.md` |
| `design` | `plan` | Specify architecture in `design.md` |
| `tasks` | `plan` | Define execution waves in `tasks.md` |
| `executing` | `execute` | Implement tasks in dependency order |
| `blocked` | `execute` | Execution halted; tasks blocked |
| `verifying` | `verify` | Run overall verification; prepare sign-off |
| `complete` | `reflect` | Spec closed; learnings promoted; reports generated |

### Lifecycle flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ 1. REQUIRE  │────►│ 2. DESIGN   │────►│ 3. TASKS    │
│   (analyze) │     │   (plan)    │     │   (plan)    │
└─────────────┘     └─────────────┘     └─────────────┘
      │                   │                   │
   specd check        specd check        specd check
   specd approve      specd approve      specd approve
      └───────────────────┴───────────────────┘
                          ▼
                  ┌─────────────┐
                  │ 4. EXECUTE  │   specd next  / specd next --dispatch
                  │  (execute)  │   specd verify / specd task
                  └─────────────┘
                          ▼
                  ┌─────────────┐
                  │ 5. VERIFY   │   specd approve
                  │  (verify)   │
                  └─────────────┘
                          ▼
                  ┌─────────────┐
                  │ 6. COMPLETE │
                  │  (reflect)  │
                  └─────────────┘
```

### The planning ratchet

To prevent skipping process steps, the CLI enforces a planning ratchet with
`check` and `approve`:

```
requirements.md ──► design.md ──► tasks.md ──► Code/Tests
     │                  │             │            │
  Gate 1: EARS    Gate 2: Design  Gates 3&4:   Execution
  specd approve   specd approve   Task schema  specd next
                                  + DAG        specd verify
                                  specd approve specd task
```

### Walkthrough: single developer, single feature

```bash
specd init
specd new auth --title "Implement JWT Authentication"

# Requirements (analyze)
$EDITOR .specd/specs/auth/requirements.md
specd check auth && specd approve auth

# Design (plan)
$EDITOR .specd/specs/auth/design.md
specd check auth && specd approve auth

# Tasks (plan)
$EDITOR .specd/specs/auth/tasks.md
specd check auth && specd approve auth

# Execute
specd next auth
#   ... implement T1 ...
specd verify auth T1
specd task auth T1 --status complete

# Close
specd approve auth
```

---

## Writing spec artifacts

Core philosophy: **intent lives in Markdown, status lives in `state.json`.**

> ⚠️ Do not hand-edit `state.json` or manually check markdown boxes. Use `specd`
> commands only — they dual-write the artifact and the state atomically.

### requirements.md (analyze phase)

Requirements must conform to the **EARS** (Easy Approach to Requirements Syntax)
grammar.

| Pattern | Syntax | Use case |
|---|---|---|
| **Ubiquitous** | `THE SYSTEM SHALL <action>` | Always-active behavior |
| **Event-driven** | `WHEN <event> THE SYSTEM SHALL <action>` | Triggered by an event |
| **State-driven** | `WHILE <state> THE SYSTEM SHALL <action>` | Active while in a state |
| **Optional-feature** | `WHERE <feature> THE SYSTEM SHALL <action>` | Active if a feature is present |
| **Unwanted** | `IF <condition> THEN THE SYSTEM SHALL <action>` | Error / exceptional case |

```markdown
# Requirements — JWT Authentication

## REQ-001 — Token Issuance
**User story:** As an API client, I want to authenticate with credentials so I can obtain a secure JWT.

**Acceptance criteria:**
1. WHEN a user submits valid credentials to /login THE SYSTEM SHALL return an HTTP 200 with a JWT.
2. IF a user submits invalid credentials THEN THE SYSTEM SHALL return an HTTP 401 Unauthorized.
3. THE SYSTEM SHALL expire tokens after 1 hour.
```

### design.md (plan phase)

Must contain all **7 mandatory H2 headers**, non-empty, free of `TODO` placeholders:

```markdown
# Design — Feature Name

## Overview
## Architecture
## Components and interfaces
## Data models
## Error handling
## Verification strategy
## Risks and open questions
```

### tasks.md (plan phase)

Tasks are Markdown checklist items grouped under `## Wave N` headers.

#### Task metadata keys

| Key | Required | Description |
|---|---|---|
| `why` | ✅ | Architectural reason for this task |
| `role` | ✅ | Persona: `scout`, `craftsman`, `auditor`, `validator` |
| `files` | ✅ | Comma-separated files modified or researched |
| `contract` | ✅ | Technical signature or behavior contract |
| `acceptance` | ✅ | Test or user criteria for completion |
| `verify` | ✅ | Shell command to verify this task (or `N/A` for read-only roles) |
| `depends` | ✅ | Comma-separated task IDs, or `—` |
| `requirements` | ❌ | Comma-separated requirement numbers |

```markdown
# Tasks — Feature Name

## Wave 1
- [ ] T1 — Task title
  - why: Reason for this task
  - role: craftsman
  - files: path/to/file.go, path/to/file_test.go
  - contract: Function signature or behavior
  - acceptance: Criteria for completion
  - verify: go test -race ./path/...
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T2 — Dependent task
  - why: Reason for this task
  - role: craftsman
  - files: path/to/file2.go
  - contract: Function signature or behavior
  - acceptance: Criteria for completion
  - verify: go test -race ./path2/...
  - depends: T1
  - requirements: 1, 2
```

#### Checkbox ↔ state mapping

| tasks.md | state.json | Meaning |
|---|---|---|
| `- [ ] T1` | `"status": "pending"` | Not started / dependencies not cleared |
| `- [/] T1` | `"status": "running"` | Work initiated |
| `- [x] T1` | `"status": "complete"` | Complete with evidence |
| `- [!] T1` | `"status": "blocked"` | Blocked with reason |

---

## Task execution & evidence

### The verify → complete flow

```bash
# 1. Get the next runnable task
specd next my-feature
#    → T1 — Create token generation utility

# 2. Implement the task (the agent does the work)

# 3. Run verification — specd runs the task's own verify: command
specd verify my-feature T1
#    Records: exit code, output tail, duration, git HEAD, changed files, optional coverage

# 4. Mark complete — only allowed if the verify record passed
specd task my-feature T1 --status complete
```

### Evidence requirements

| Task type | Evidence required | Command |
|---|---|---|
| **Craftsman / Validator** | Passing `specd verify` record | `specd task <slug> <id> --status complete` |
| **Scout / Auditor** | Manual evidence (read-only roles) | `specd task <slug> <id> --status complete --unverified --evidence "..."` |

### Verification timeout

- Default: `600000ms` (10 minutes)
- Override: `SPECD_VERIFY_TIMEOUT_MS`
- On timeout, the **task's recorded** exit code is `124` and the record is marked
  `verified: false`. The `specd verify` process itself still exits `1` (failed
  verification).

### Sandboxed verify & rollback

`tasks.md` is agent-authored input, so a `verify:` line is untrusted code. Two
opt-in safeguards harden a run (defaults are unchanged):

```bash
# Isolate the run — bwrap or container; fails closed if the isolator is absent
specd verify my-feature T1 --sandbox bwrap

# Stash the working tree (recoverable) if verify exits non-zero, instead of
# leaving it dirty. Never uses `reset --hard`; a passing verify never touches the tree.
specd verify my-feature T1 --revert-on-fail
```

Set a project-wide default with `verify.sandbox` in `.specd/config.yml`. See
[SECURITY.md](../SECURITY.md) for the isolation and fail-closed contract.

### Telemetry annotations

Annotate a completion with token/cost figures from your agent runtime. They are
**stored, never computed** — specd does no pricing math — and roll up per
wave/spec in `specd report`:

```bash
specd task my-feature T1 --status complete --evidence "…" --tokens 12000 --cost 0.42
```

### Blocking a task

```bash
specd task my-feature T1 --status blocked \
  --reason "Underlying database client lacks connection pooling"
```

### Inspecting & streaming progress

Beyond `status` / `report`, these read-only commands surface live and historical
state (see the [Command Reference](./command-reference.md)):

| Command | Use |
|---|---|
| `specd report <slug> --serve` | Live read-only HTML dashboard + `GET /api/report` JSON |
| `specd report --watch [--once] [--sse <addr>] [--webhook <url>]` | Stream a `FrontierEvent` on every runnable-set change |
| `specd report <slug> --history` | Deterministic audit timeline from on-disk records |
| `specd check --schema` · `specd check <slug> --schema-only` | Emit / check against the open-spec-format JSON Schema |
| `specd mcp` | Drive the whole workflow from an MCP client |

### Autonomous Orchestration (Brain/Pinky)

For automated pipelines, `specd` provides an optional, deterministic orchestration layer:
- **The Brain** (`specd brain`) processes state and makes stepping decisions.
- **Pinky** (`specd pinky`) executes worker claims under lease without network or LLM calls in the core.

#### One-Command Autonomous Setup

You can enable and configure the entire Brain/Pinky orchestration stack at project bootstrap time using the `--orchestration` flag with `specd init`. 

```sh
specd init --agent auto --orchestration session --yes
```

This command:
1. Scaffolds `.specd/config.yml` with `orchestration.enabled: true`.
2. Registers the MCP server on your host agent (e.g., Claude Code).
3. Configures subagent mode to `"delegate"`, allowing Pinky workers to spawn in isolated context windows.
4. Installs the worker subagent definitions under `.claude/agents/pinky-*.md`.

##### Configuration Options
- `--orchestration <manual|planning|session>`: Enable orchestration and set the approval policy. Defaults to `planning`.
- `--orchestration-workers <1..64>`: Max concurrent Pinky workers (default: `4`).
- `--orchestration-retries <0..10>`: Max retries for failed tasks (default: `2`).
- `--orchestration-timeout <minutes>`: Session wall-clock timeout in minutes (default: `120`).
- `--orchestration-cost-limit <usd>`: Host-reported cost brake in USD (default: `0` / disabled).
- `--orchestration-mode <inline|delegate>`: Subagent coordination mode (default: `delegate`).
- `--orchestration-sandbox <none|bwrap|container>`: Default task verification sandbox (default: `none`).

For more details on orchestration configuration, see the [Agent Integration Guide](./agent-integration.md#brainpinky-orchestration).

---

## Sharing, dashboards & migration

Three v0.2.0 commands operate on the project as a whole rather than a single
spec: `migrate` (upgrade), `dashboard` (observe), and `harness` (share policy).

### `specd migrate` — upgrade a v0.1.x project

Run once after upgrading the binary. It is the documented, idempotent path onto
the v0.2.0 state schema:

```bash
specd migrate            # human-readable report
specd migrate --json     # {schemaVersion, specs[], hints[]}
```

It rewrites each spec's state at schema v6 and reports which additive config
blocks (`guardrails`, `deploy`, `routing`, `eval`, `resilience`) are present
(`●`) versus available to adopt (`○`). It never writes policy content, so a
migrated repo keeps every new gate default-off; adopt each block explicitly.
Exit codes: `0` success, `1` migration failed (concurrent write / corrupt
state), `3` no `.specd/`.

### `specd dashboard` — unified read-only view

A project-wide, loopback-only web view that aggregates every spec's status,
orchestrator waves, conductor sessions, eval trends, cost, escalations, and the
shared harness bundle. It is read-only (no mutating routes) and makes **zero
outbound network calls** — everything renders from local state and ledgers.

```bash
specd dashboard                                  # http://127.0.0.1:8765/
specd dashboard --mode cost                       # focus one panel: all|conductor|orchestrator|cost|eval
specd dashboard <slug> --addr 127.0.0.1:9000      # per-spec report as the default target
```

`GET /` is the aggregate view, `GET /s/<slug>` a per-spec report, `GET
/api/dashboard` the deterministic JSON projection, and `/events` the same SSE
live-update stream as `specd serve`. Append `?mode=` to any request to switch
panels without a restart. It binds loopback only; put it behind a
TLS-terminating reverse proxy to expose it.

### `specd harness` — share your policy as a team asset

Bundle the configured harness — guardrails, deploy templates, roles, routing —
and share it as a versioned asset over git, with SHA256 pinning and a
fail-closed quarantine for anything executable.

```bash
specd harness push <git-url> [--name <n>]   # bundle current policy and push
specd harness pull <git-url> [--force]      # import a bundle (refuses to clobber local edits without --force)
specd harness list                          # show the bundle + quarantine
specd harness enable <path> [--force]       # install one quarantined artifact, recording the decision
```

`pull` imports declarative policy directly but **quarantines every executable
`command` artifact** until you run `harness enable` on it — a deliberate
evidence gate (constitution #5) so a shared bundle can never silently introduce
code that runs on your machine. Locally modified files are refused unless you
pass `--force`. Exit `1` on any gate failure (refused overwrite, checksum
mismatch, downgrade). All git transport goes through a single hardened
`SecureGitClone` path (scrubbed env, transport allowlist — `ext::`/`file::`
style URLs are rejected).

#### Walkthrough: pull a bundle and recover a quarantined command

After `specd harness pull`, declarative policy (guardrails, roles, routing) is
installed immediately, but every artifact carrying an executable `command` is
held in quarantine — it is present on disk but never runs until you explicitly
enable it. `specd harness list` shows what landed and what is still gated. A `⚠`
marks executable files, and quarantined items are listed separately:

```
$ specd harness pull https://github.com/acme/specd-harness.git
harness pull: "acme-standard" v3
  installed:   1
  quarantined: 2 (executable — enable explicitly):
    ⚠ .specd/hooks/pre-submit.sh → `specd harness enable .specd/hooks/pre-submit.sh`
    ⚠ .specd/deploy/staging.sh → `specd harness enable .specd/deploy/staging.sh`

$ specd harness list
harness "acme-standard" v3 (from https://github.com/acme/specd-harness.git)
    .specd/guardrails.yml
  ⚠ .specd/hooks/pre-submit.sh
  ⚠ .specd/deploy/staging.sh
quarantined (awaiting enable):
  ⚠ .specd/hooks/pre-submit.sh
  ⚠ .specd/deploy/staging.sh
```

Inspect each quarantined file, then enable the ones you trust one at a time.
Each `enable` records the decision in the harness decision log (the evidence
gate, constitution #5), so the choice is auditable:

```
$ specd harness enable .specd/hooks/pre-submit.sh
✓ harness enable: installed .specd/hooks/pre-submit.sh (recorded in harness decision log)
```

If a bundle update would overwrite a file you edited locally, `enable` refuses
until you re-run it with `--force`. A quarantined path that does not exist exits
`3`; any gate failure (checksum mismatch, downgrade, refused overwrite) exits
`1`. Nothing executable ever runs on your machine without this explicit step.

---

## Troubleshooting

| Symptom | Cause & fix |
|---|---|
| `--status complete requires a passing specd verify` | No verify record. Run `specd verify <slug> <task>` first, or use `--unverified --evidence` for read-only tasks. |
| `verification is stale` | The `verify:` line changed since recording. Re-run `specd verify`. |
| `spec is gated (awaiting-approval)` | A `high`/`critical` `midreq` froze the spec. Review changes, then `specd approve`. |
| `exit 3` on any command | `.specd/` root or spec slug not found. Run `specd init` / `specd new`, or run from the target repo. |
| `dependency cycle` / `depends on missing task` | DAG error in `tasks.md`. Fix `depends:` keys; use `specd check` and `specd waves` to pinpoint. |
| CAS / revision write abort (`exit 1`) | Concurrent write clobber prevented. Re-read state and retry. |

### Agent tips

- Set `SPECD_JSON=1` (or pass `--json`) for structured output parsing.
- Use `specd help --json` to discover the command schema programmatically.
- All state mutations are atomic and versioned — safe for concurrent agent access.
- Never hand-edit `state.json` — always use CLI commands.

See [Agent Integration](./agent-integration.md) for wiring an agent end-to-end.
