# User Guide

Using `specd` inside a target repository — from install through a closed spec.
For the *why*, read [Concepts](./concepts.md); for command-level detail, the
[Command Reference](./command-reference.md).

## Contents

1. [Installation & setup](#installation--setup)
2. [The spec lifecycle](#the-spec-lifecycle)
3. [Writing spec artifacts](#writing-spec-artifacts)
4. [Task execution & evidence](#task-execution--evidence)
5. [Troubleshooting](#troubleshooting)

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
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | bash
specd update            # self-update to the latest release
specd update --force
```

- Linux, macOS, or Windows (amd64 / arm64)
  - *Windows note*: `specd` runs on Windows, except for `specd update` which is blocked by OS file locks (reinstall manually). Verification commands also require `sh` or `bash` (e.g. from Git for Windows) in the `PATH` since verify execution uses `-c`.
- Git (optional — tarball fallback available)
- **Zero runtime dependencies** — single binary

### Initialize a project

The golden path is one command in your project root:

```bash
cd <your-project>
specd init --agent auto
```

This scaffolds `.specd/` (default templates, steering files, roles, skills,
`config.json`) and merges `AGENTS.md`, **then** detects your coding agent, installs
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
  (`.mcp.json`, `.cursor/mcp.json`, `.gemini/settings.json`, …).
- **Global host config is never modified** without `--scope global` **and** explicit
  consent (`--yes` or an interactive confirmation).
- `--yes` authorizes only documented, non-destructive, project-scoped changes.
- Existing host config must parse before any mutation; specd fails closed and creates
  a timestamped backup before changing an existing file.
- Unrelated MCP servers and settings in a host config are always preserved.

Supported managed adapters (auto-detect + install): **claude-code, codex, cursor,
gemini, vscode**. **antigravity** and **claude-desktop** ship deterministic manual
snippets only (`specd mcp --config <host>`). See
[mcp-guide.md](mcp-guide.md) and [agent-harness-compat.md](agent-harness-compat.md)
for the full host matrix and trust boundaries.

#### Verify and repair

```bash
specd doctor          # scaffold + MCP server + host-registration health, with remedies
specd doctor --fix    # apply safe, project-scoped, specd-owned repairs only
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
3. Set `config.defaultVerify` in `.specd/config.json` to the real test command you
   found (e.g. `go test ./...`, `npm test`, `pytest`).

The harness performs **zero inference**: it scaffolds the skill pack at `init` and
enforces specs at `check`. Perceiving the stack and authoring steering is the
agent's job (the Foundational Split).

---

## The spec lifecycle

A **spec** is a modular directory representing a single feature, task, or bugfix.
Its lifecycle has 5 phases driven by a status machine.

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
                  │ 4. EXECUTE  │   specd next  / specd dispatch
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
| `role` | ✅ | Persona: `investigator`, `builder`, `reviewer`, `verifier` |
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
  - role: builder
  - files: path/to/file.go, path/to/file_test.go
  - contract: Function signature or behavior
  - acceptance: Criteria for completion
  - verify: go test -race ./path/...
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T2 — Dependent task
  - why: Reason for this task
  - role: builder
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
| **Builder / Verifier** | Passing `specd verify` record | `specd task <slug> <id> --status complete` |
| **Investigator / Reviewer** | Manual evidence (read-only roles) | `specd task <slug> <id> --status complete --unverified --evidence "..."` |

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

Set a project-wide default with `verify.sandbox` in `.specd/config.json`. See
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
| `specd serve <slug>` | Live read-only HTML dashboard + `GET /api/report` JSON |
| `specd watch [--once] [--sse <addr>] [--webhook <url>]` | Stream a `FrontierEvent` on every runnable-set change |
| `specd replay <slug>` | Deterministic audit timeline from on-disk records |
| `specd schema` · `specd validate <slug> --schema` | Emit / check against the open-spec-format JSON Schema |
| `specd mcp` | Drive the whole workflow from an MCP client |

### Autonomous Orchestration (Brain/Pinky)

For automated pipelines, `specd` provides an optional, deterministic orchestration layer:
- **The Brain** (`specd brain`) processes state and makes stepping decisions.
- **Pinky** (`specd pinky`) executes worker claims under lease without network or LLM calls in the core.

For configuration and integration details, see the [Agent Integration Guide](./agent-integration.md#brainpinky-orchestration).

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
