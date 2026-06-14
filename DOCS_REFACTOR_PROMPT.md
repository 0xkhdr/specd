# specd — Developer-Friendly Documentation & Refactor Prompt

> **Agent-agnostic, spec-driven coding harness CLI.**  
> *The agent reasons. The harness enforces.*

---

## Table of Contents

1. [What is specd?](#1-what-is-specd)
2. [Core Philosophy & Principles](#2-core-philosophy--principles)
3. [Architecture Overview](#3-architecture-overview)
4. [Installation & Setup](#4-installation--setup)
5. [The Spec Lifecycle](#5-the-spec-lifecycle)
6. [Writing Spec Artifacts](#6-writing-spec-artifacts)
7. [Command Reference](#7-command-reference)
8. [Validation Gates](#8-validation-gates)
9. [Task Execution & Evidence](#9-task-execution--evidence)
10. [Agent Integration](#10-agent-integration)
11. [Cross-Spec Programs](#11-cross-spec-programs)
12. [Configuration & Environment](#12-configuration--environment)
13. [Troubleshooting](#13-troubleshooting)
14. [Contributor Guide](#14-contributor-guide)
15. [Use Cases & Workflows](#15-use-cases--workflows)

---

## 1. What is specd?

`specd` is a **spec-driven coding harness CLI** that fuses structured spec workflows with rigid thinking discipline for AI coding agents. It shifts the burden of process enforcement from the LLM's non-deterministic context to a strict, local, tool-gated pipeline.

### Key Capabilities

| Feature | Description |
|---------|-------------|
| **Planning Ratchet** | Enforces human-approved phase gates (Analyze → Plan → Execute → Verify → Reflect) |
| **7 Validation Gates** | Programmatic checks (`specd check`) for EARS syntax, design completeness, acyclic DAGs, and sync |
| **DAG-Based Execution** | Computes concurrent runnable frontier of waves for parallel task execution |
| **Evidence-Gated Completion** | Tasks complete only against passing `verify:` records — never on free-text claims |
| **Frontier Dispatch** | Emits ready-to-run packets for parallel subagents with role prompts + contracts |
| **Agent-Agnostic** | Works with Claude Code, Cursor, Aider, or any command-running agent |
| **Deterministic Reporting** | Generates markdown and self-contained HTML reports without LLM dependencies |

### The Foundational Split

```
┌─────────────────┐     ┌─────────────────┐
│  AGENT (LLM)    │     │  HARNESS (specd)│
│  ─────────────  │     │  ─────────────  │
│  • Reasons      │     │  • Enforces     │
│  • Creates      │◄───►│  • Validates    │
│  • Designs      │     │  • Gates        │
│  • Implements   │     │  • Records      │
└─────────────────┘     └─────────────────┘
```

---

## 2. Core Philosophy & Principles

`specd` is built on eight core principles designed to make AI software engineering reliable, structured, and predictable:

### The Eight Principles

1. **The Foundational Split**  
   The agent does the creative thinking; the harness enforces process integrity.

2. **Specs as the Source of Truth**  
   The active plan lives as versioned Markdown on disk, not floating in the LLM's context window.

3. **Evidence Gates Every State Change**  
   *Trust is recorded, not assumed.* Status changes require verifiable proof.

4. **Waves, Not Lines**  
   Work is structured as a Directed Acyclic Graph (DAG) of concurrent batches (waves) rather than flat todo lists.

5. **Agent-Agnostic by Design**  
   Standardized command interface integrated via role prompt injection.

6. **Human Gates at Phase Boundaries**  
   Semantic transitions require explicit human approval (`specd approve`).

7. **Deterministic Reporting**  
   Reports are generated programmatically from `state.json` and artifact files.

8. **Steering as Constitution**  
   Durable steering files outlive individual chat sessions.

### Design Philosophy in Practice

```
Traditional AI Coding          specd-Driven Coding
─────────────────────         ───────────────────
❌ Free-form prompts           ✅ Structured spec artifacts
❌ Context window as source    ✅ Markdown files as source of truth
❌ "Trust me, it works"        ✅ Evidence-gated completion
❌ Linear todo lists           ✅ DAG-based wave execution
❌ Agent-specific workflows    ✅ Agent-agnostic CLI interface
```

---

## 3. Architecture Overview

### Repository Structure

> **Implementation language:** Go (1.22+), standard library only — zero external dependencies. Ships as a single static binary with all templates embedded via `go:embed`.

```
specd/
├── main.go                       # Entry point, arg router, dispatch switch
├── main_test.go                  # Entry-point / global-flag tests
├── internal/
│   ├── cli/
│   │   ├── args.go               # Flag/positional parser (Args)
│   │   └── args_test.go
│   ├── cmd/                      # One file per CLI command (Run<Command>)
│   │   ├── init.go               # Scaffold .specd/ + AGENTS.md
│   │   ├── boot.go               # Auto-detect tech stack, populate .specd/
│   │   ├── enrich.go             # AI-enrich steering stubs (plan/apply/status)
│   │   ├── new.go                # Create new spec
│   │   ├── check.go              # Run validation gates (+ --boot / --enrich)
│   │   ├── approve.go            # Planning ratchet + phase transitions
│   │   ├── next.go               # Next runnable task / frontier
│   │   ├── dispatch.go           # Emit subagent packets
│   │   ├── verify.go             # Run task verify command + record proof
│   │   ├── task.go               # Evidence gate + dual-write
│   │   ├── decision.go           # ADR append
│   │   ├── midreq.go             # Mid-flight requirement updates
│   │   ├── memory.go             # Learnings + promote
│   │   ├── report.go             # md/html snapshot generation
│   │   ├── waves.go              # Wave DAG view
│   │   ├── program.go            # Cross-spec DAG
│   │   ├── status.go             # Progress dashboard
│   │   ├── context.go            # Phase-scoped briefing
│   │   ├── update.go             # Self-update
│   │   └── helpers.go            # Shared command helpers
│   ├── core/                     # Domain logic
│   │   ├── paths.go              # .specd root locator (FindSpecdRoot)
│   │   ├── io.go                 # Atomic writes (AtomicWrite), O_APPEND ledger
│   │   ├── lock.go               # Per-spec advisory lock (WithSpecLock)
│   │   ├── state.go              # state.json load/save + CAS
│   │   ├── phases.go             # Phase ↔ status mapping, design gate
│   │   ├── tasksparser.go        # Line-based tasks.md parser (ParseTasksMd)
│   │   ├── dag.go                # Wave DAG, frontier, critical path
│   │   ├── ears.go               # EARS requirements linter
│   │   ├── report.go             # Report assembler
│   │   ├── specfiles.go          # Artifact accessors, sync + traceability gates
│   │   ├── render.go             # Wave graph text renderer
│   │   ├── boot.go               # Boot manifest + boot-freshness gate
│   │   ├── boot_detectors.go     # Deterministic stack detectors (Go/Node/Py/Rust)
│   │   ├── enrich.go             # Enrich plan/apply contract
│   │   ├── enrich_evidence.go    # Enrich freshness evidence + gate
│   │   ├── agents.go             # AGENTS.md marker-based merge
│   │   ├── commands.go           # CommandMeta registry (help/JSON schema)
│   │   ├── help.go               # Help renderer (text + JSON)
│   │   ├── program.go            # program.json cross-spec graph
│   │   ├── slug.go               # Slug validation
│   │   ├── md.go                 # Markdown helpers
│   │   ├── ui.go                 # Output/JSON-mode + color helpers
│   │   ├── exit.go               # Exit code constants + SpecdError
│   │   ├── embed.go              # go:embed of embed_templates/
│   │   └── embed_templates/      # Shipped templates (embedded in binary)
│   │       ├── AGENTS.md         # Agent prompt pack for user repos
│   │       ├── config.json       # Default config scaffold
│   │       ├── steering/         # Constitution files
│   │       │   ├── reasoning.md
│   │       │   ├── workflow.md
│   │       │   ├── product.md
│   │       │   ├── tech.md
│   │       │   ├── structure.md
│   │       │   └── memory.md
│   │       ├── roles/            # Role persona prompts
│   │       │   ├── investigator.md
│   │       │   ├── builder.md
│   │       │   ├── reviewer.md
│   │       │   └── verifier.md
│   │       ├── specStubs/        # Spec artifact stubs
│   │       │   ├── requirements.md
│   │       │   ├── design.md
│   │       │   ├── tasks.md
│   │       │   ├── decisions.md
│   │       │   ├── mid-requirements.md
│   │       │   └── memory.md
│   │       └── skills/
│   │           └── specd-enrich/SKILL.md   # Enrich companion skill
│   └── testharness/              # Deterministic test infrastructure
│       ├── harness.go            # Sandbox repo + CLI runner
│       ├── runner.go             # In-process command runner
│       ├── clock.go              # FakeClock (deterministic time)
│       ├── spec_builder.go       # Fluent spec fixture builder
│       ├── asserter.go           # Assertions
│       └── git.go                # Throwaway git repo helper
│   # Unit tests are co-located as *_test.go beside each source file
│   # (core.test, dag_test, ears_test, tasksparser_test, check via
│   #  commands_test/lifecycle_test, state_cas_test, concurrency_test, …).
├── scripts/
│   ├── install.sh                # curl | bash installer
│   ├── uninstall.sh              # Remove binary + PATH
│   └── stress.sh                 # Concurrency stress harness
├── docs/
│   ├── README.md                 # Documentation index
│   ├── user-guide.md             # Using specd in target repos
│   ├── agent-integration.md      # Wiring agents to specd
│   └── contributor-guide.md      # Hacking on specd itself
├── README.md                     # Project overview
├── AGENTS.md                     # Agent guide for this repo
├── TESTING.md                    # Test-harness guide
├── Makefile                      # build / install / test / lint / clean
├── go.mod                        # Module definition (stdlib only)
├── LICENSE                       # MIT
└── .goreleaser.yml               # Multi-platform release builds
```

### Target Repository Structure (After `specd init`)

```
your-project/
├── .specd/
│   ├── config.json               # Project configuration
│   ├── boot.json                 # Detected stack manifest (specd boot)
│   ├── program.json              # Cross-spec dependencies
│   ├── state.json                # Machine state (auto-managed)
│   ├── skills/                   # Companion skills (e.g. specd-enrich)
│   ├── steering/                 # Constitution (durable rules)
│   │   ├── reasoning.md
│   │   ├── workflow.md
│   │   ├── product.md
│   │   ├── tech.md
│   │   ├── structure.md
│   │   └── memory.md
│   ├── roles/                    # Role prompts
│   │   ├── investigator.md
│   │   ├── builder.md
│   │   ├── reviewer.md
│   │   └── verifier.md
│   └── specs/
│       └── my-feature/
│           ├── state.json        # Spec-specific state
│           ├── requirements.md   # EARS requirements
│           ├── design.md         # Design document
│           ├── tasks.md          # Task DAG
│           ├── decisions.md      # ADRs
│           ├── mid-requirements.md # Requirement updates
│           └── memory.md         # Local learnings
└── AGENTS.md                     # Agent workflow guide
```

---

## 4. Installation & Setup

### Quick Install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash
```

After install, restart your shell (or `source ~/.bashrc` / `source ~/.zshrc`).

### Install Options

```bash
# Force reinstall / upgrade
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --force

# Pin a specific version
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --version 0.2.0
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | bash
```

### Update

```bash
specd update
specd update --force
```

### Requirements

- Linux or macOS (amd64 / arm64)
- Git (optional — tarball fallback available)
- **Zero runtime dependencies** — single binary

### Initialize a Project

```bash
# In your project root
specd init
```

This scaffolds the `.specd/` directory with default templates, steering files, roles, and `AGENTS.md`.

### Bootstrap Project Context (Optional but Recommended)

After `init`, seed the steering constitution from the real repository:

```bash
# Deterministic, AI-free stack detection → boot.json + steering/tech.md + config.defaultVerify
specd boot
specd boot --dry-run   # preview without writing

# AI companion: have the agent author the remaining steering sections
specd enrich plan --json                       # brief: which sections to write + evidence to read
specd enrich apply --target product < out.md   # accept the agent's authored markdown
specd enrich status                            # check freshness (also: specd check --enrich)
```

`boot` performs **zero LLM calls** — every detected fact is traceable to a source file. `enrich` performs no inference either: it owns the contract and the freshness gate while the calling agent does the writing.

---

## 5. The Spec Lifecycle

A **spec** is a modular directory representing a single feature, task, or bugfix. The lifecycle consists of 5 phases driven by a status machine.

### Status → Phase Mapping

| Spec Status (`state.json`) | Derived Phase | Primary Activities |
|---------------------------|---------------|-------------------|
| `requirements` | `analyze` | Author EARS requirements in `requirements.md` |
| `design` | `plan` | Specify architecture in `design.md` |
| `tasks` | `plan` | Define execution waves in `tasks.md` |
| `executing` | `execute` | Implement tasks in sequence |
| `blocked` | `execute` | Execution halted; tasks blocked |
| `verifying` | `verify` | Run overall verification; prepare sign-off |
| `complete` | `reflect` | Spec closed; learnings promoted; reports generated |

### Lifecycle Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ 1. REQUIRE  │────►│ 2. DESIGN   │────►│ 3. TASKS    │
│   (analyze) │     │   (plan)    │     │   (plan)    │
└─────────────┘     └─────────────┘     └─────────────┘
      │                   │                   │
      ▼                   ▼                   ▼
  specd check         specd check         specd check
  specd approve       specd approve       specd approve
      │                   │                   │
      └───────────────────┴───────────────────┘
                          │
                          ▼
                  ┌─────────────┐
                  │ 4. EXECUTE  │
                  │  (execute)  │
                  └─────────────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
        specd next   specd verify  specd task
        (frontier)   (evidence)    (status)
              │           │           │
              └───────────┴───────────┘
                          │
                          ▼
                  ┌─────────────┐
                  │ 5. VERIFY   │
                  │  (verify)   │
                  └─────────────┘
                          │
                          ▼
                    specd approve
                          │
                          ▼
                  ┌─────────────┐
                  │ 6. COMPLETE │
                  │  (reflect)  │
                  └─────────────┘
```

### The Planning Ratchet

To prevent skipping process steps, the CLI enforces a planning ratchet using `check` and `approve`:

```
requirements.md ──► design.md ──► tasks.md ──► Code/Tests
     │                  │              │            │
  Gate 1: EARS      Gate 2: Design  Gate 3&4:     Execution
  specd approve     specd approve   Task Schema   specd next
                                    DAG           specd verify
                                    specd approve specd task
```

---

## 6. Writing Spec Artifacts

The core philosophy: **intent lives in Markdown, status lives in state.json**.

### 6.1 requirements.md (Analyze Phase)

Requirements must conform to the **EARS (Easy Approach to Requirements Syntax)** grammar.

#### EARS Patterns

| Pattern | Syntax | Use Case |
|---------|--------|----------|
| **Ubiquitous** | `THE SYSTEM SHALL <action>` | Always active behavior |
| **Event-driven** | `WHEN <event> THE SYSTEM SHALL <action>` | Triggered by event |
| **State-driven** | `WHILE <state> THE SYSTEM SHALL <action>` | Active while in state |
| **Optional-feature** | `WHERE <feature> THE SYSTEM SHALL <action>` | Active if feature present |
| **Unwanted** | `IF <condition> THEN THE SYSTEM SHALL <action>` | Error/exceptional case |

#### Template

```markdown
# Requirements — Feature Name

## REQ-001 — Feature Title
**User story:** As a <role>, I want <goal> so that <benefit>.

**Acceptance criteria:**
1. WHEN <event> THE SYSTEM SHALL <action>.
2. IF <condition> THEN THE SYSTEM SHALL <action>.
3. THE SYSTEM SHALL <action>.
```

#### Example

```markdown
# Requirements — JWT Authentication

## REQ-001 — Token Issuance
**User story:** As an API client, I want to authenticate with credentials so I can obtain a secure JWT.

**Acceptance criteria:**
1. WHEN a user submits valid credentials to /login THE SYSTEM SHALL return an HTTP 200 with a JWT.
2. IF a user submits invalid credentials THEN THE SYSTEM SHALL return an HTTP 401 Unauthorized.
3. THE SYSTEM SHALL expire tokens after 1 hour.
```

### 6.2 design.md (Plan Phase)

Must contain all **7 mandatory H2 headers**, non-empty, and free of `TODO` placeholders:

```markdown
# Design — Feature Name

## Overview
High-level description of the implementation.

## Architecture
How the feature fits into the application structure.

## Components and interfaces
Signatures and file locations for all components.

## Data models
Structure of data objects and schemas.

## Error handling
Exceptional scenarios and recovery strategies.

## Verification strategy
Unit tests, integration tests, and E2E tests.

## Risks and open questions
Security risks, performance considerations, unknowns.
```

### 6.3 tasks.md (Plan Phase)

Tasks are Markdown checklist items grouped under `## Wave N` headers.

#### Task Metadata Keys

| Key | Required | Description |
|-----|----------|-------------|
| `why` | ✅ | Architectural reason for this task |
| `role` | ✅ | Persona: `investigator`, `builder`, `reviewer`, `verifier` |
| `files` | ✅ | Comma-separated files modified or researched |
| `contract` | ✅ | Technical signature or behavior contract |
| `acceptance` | ✅ | Test or user criteria for completion |
| `verify` | ✅ | Shell command to verify this task (or `N/A` for read-only) |
| `depends` | ✅ | Comma-separated task IDs, or `—` |
| `requirements` | ❌ | Comma-separated requirement numbers |

#### Template

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

#### Checkbox ↔ State Mapping

| tasks.md | state.json | Meaning |
|----------|-----------|---------|
| `- [ ] T1` | `"status": "pending"` | Not started / dependencies not cleared |
| `- [/] T1` | `"status": "running"` | Work initiated |
| `- [x] T1` | `"status": "complete"` | Complete with evidence |
| `- [!] T1` | `"status": "blocked"` | Blocked with reason |

**⚠️ Do not hand-edit `state.json` or manually check markdown boxes. Use `specd` commands only.**

---

## 7. Command Reference

### Lifecycle Commands

| Command | Description | Exit Codes |
|---------|-------------|------------|
| `specd init [--force]` | Scaffold `.specd/` structure in project root | `0` success, `2` usage error, `3` `.specd/` exists (no `--force`) |
| `specd boot [--dry-run] [--force] [--json]` | Auto-detect tech stack (AI-free detectors) → `boot.json`, managed block in `steering/tech.md`, `config.defaultVerify` | `0` success, `1` write error, `3` no `.specd/` |
| `specd enrich [plan\|apply\|status]` | AI companion to `boot`: brief the agent (`plan`), accept authored steering (`apply --target <product\|structure\|tech>`), gate freshness (`status`) | `0` success, `1` gate/write fail, `3` no `boot.json` |
| `specd new <slug> [--title "..."]` | Create new spec with six artifact stubs | `0` success, `2` usage, `3` no `.specd/` root or spec exists |
| `specd check <slug>` | Run all 7 validation gates on a spec | `0` pass, `1` fail, `3` not found |
| `specd check --boot` | Run repo-global boot-freshness gate | `0` pass, `1` drift, `3` no `boot.json` |
| `specd check --enrich` | Run repo-global enrich-freshness gate | `0` pass, `1` missing/drift, `3` not found |
| `specd approve <slug>` | Advance to next phase / clear approval gate (human gate) | `0` success, `1` gates failed |

### Execution Commands

| Command | Description | Exit Codes |
|---------|-------------|------------|
| `specd next <slug>` | Get next runnable task | `0` found, `1` gated, `3` not found |
| `specd next <slug> --all` | Get entire runnable frontier | `0` found, `1` gated |
| `specd dispatch <slug> --json` | Emit subagent packets for frontier | `0` success |
| `specd verify <slug> <task-id>` | Run task's verify command + record proof (exit code, output tail, duration, git HEAD) | `0` pass, `1` fail, `3` not found |
| `specd verify <slug> --criterion <r>.<n> --status pass\|fail --evidence "..."` | Record a per-acceptance-criterion proof | `0` success, `1` fail, `2` usage |
| `specd task <slug> <task-id> --status <state> [--evidence "..."] [--reason "..."] [--unverified] [--force]` | Evidence-gated status flip | `0` success, `1` rejected, `2` usage, `3` not found |

> On verify timeout, the **task's recorded** exit code is `124` and the record is marked `verified: false`. The `specd verify` process itself still exits `1` (failed verification).

### Inspection Commands

| Command | Description |
|---------|-------------|
| `specd status <slug>` | Progress dashboard |
| `specd waves <slug>` | Wave graph, critical paths, blockers |
| `specd context <slug>` | Phase briefing + load list + signals |
| `specd report <slug> --format html --out file.html` | Generate snapshot report |

### Record Commands

| Command | Description |
|---------|-------------|
| `specd decision <slug> "..."` | Append ADR to `decisions.md` |
| `specd midreq <slug> "..." --impact high --interpretation "..." --changes "..."` | Log requirement update |
| `specd memory <slug> add --key "..." --pattern "..." --body "..." --source "T1" --criticality important` | Record learning |
| `specd memory <slug> promote --key "..."` | Promote to global steering |

### Program Commands

| Command | Description |
|---------|-------------|
| `specd program link <spec> --on <dependency>` | Declare inter-spec dependency |
| `specd program unlink <spec> --on <dependency>` | Remove inter-spec dependency |
| `specd program [status]` | Render spec-level DAG + runnable frontier |
| `specd program [status] --json` | JSON output for orchestrators |

### Meta Commands

| Command | Description |
|---------|-------------|
| `specd update [--force]` | Self-update to latest release |
| `specd version` | Show version |
| `specd help [--json]` | Show help / JSON schema |

---

## 8. Validation Gates

The `specd check` command runs 7 strict verification checks:

### Gate 1: EARS Gate
- **File**: `internal/core/ears.go`
- **Checks**: Every requirement contains a user story; all acceptance criteria match one of five EARS patterns
- **Fail**: Invalid grammar, missing user story, malformed criteria

### Gate 2: Design Gate
- **File**: `internal/core/phases.go`
- **Checks**: All 7 mandatory H2 headers present, non-empty, no `TODO` markers
- **Fail**: Missing header, empty section, placeholder text

### Gate 3: Task-Schema Gate
- **File**: `internal/core/tasksparser.go`
- **Checks**: All tasks have 7 mandatory keys (`why`, `role`, `files`, `contract`, `acceptance`, `verify`, `depends`)
- **Fail**: Missing key, builder/verifier with `verify: N/A`

### Gate 4: DAG Gate
- **File**: `internal/core/dag.go`
- **Checks**: Acyclic dependencies, no orphan deps, valid wave numbering
- **Fail**: Circular dependency, missing task ID, wave violation

### Gate 5: Evidence Gate
- **Files**: `internal/cmd/check.go`, `internal/core/state.go`
- **Checks**: No task complete without evidence; non-read-only tasks require passing verify record
- **Fail**: Complete task without verify record

### Gate 6: Sync Gate
- **File**: `internal/core/specfiles.go`
- **Checks**: Markdown checkbox statuses match `state.json` task statuses
- **Fail**: Mismatch between `tasks.md` and `state.json`

### Gate 7: Traceability Gate
- **File**: `internal/core/specfiles.go`
- **Checks**: Every requirement ID referenced in tasks exists in `requirements.md`
- **Severity**: Controlled by `config.gates.traceability` (`warn` or `error`)

### Repo-Global Freshness Gates

Two gates run on the whole repository (no spec slug) rather than a single spec:

| Gate | Command | File | Checks |
|------|---------|------|--------|
| **Boot-freshness** | `specd check --boot` | `internal/core/boot.go` | `boot.json` still matches the repo — source files exist, no detection drift |
| **Enrich-freshness** | `specd check --enrich` | `internal/core/enrich_evidence.go` | Agent-authored steering enrichment is present, complete, and not drifted from `boot` |

---

## 9. Task Execution & Evidence

### The Verify → Complete Flow

```bash
# 1. Get next task
specd next my-feature
# Output: T1 — Create token generation utility

# 2. Implement the task (agent does the work)
# ... write code ...

# 3. Run verification (specd runs the task's verify command)
specd verify my-feature T1
# Records: exit code, output tail, duration, git HEAD

# 4. Mark complete (only allowed if verify passed)
specd task my-feature T1 --status complete
```

### Evidence Requirements

| Task Type | Evidence Required | Command |
|-----------|------------------|---------|
| **Builder/Verifier** | Passing `specd verify` record | `specd task <slug> <id> --status complete` |
| **Investigator/Reviewer** | Manual evidence (read-only roles) | `specd task <slug> <id> --status complete --unverified --evidence "..."` |

### Verification Timeout

- Default: `600000ms` (10 minutes)
- Override: `SPECD_VERIFY_TIMEOUT_MS`
- Timeout records as `verified: false` with exit `124`

### Blocking Tasks

```bash
specd task my-feature T1 --status blocked --reason "Underlying database client lacks connection pooling"
```

---

## 10. Agent Integration

### The Two AGENTS.md Files

| File | Location | Purpose |
|------|----------|---------|
| **Root AGENTS.md** | `0xkhdr/specd` repo root | Guide for agents **developing specd itself** |
| **Template AGENTS.md** | `internal/core/embed_templates/AGENTS.md` | Guide written to **user repos** by `specd init` |

### Steering Constitution (`.specd/steering/`)

Durable rules that outlive chat sessions:

| File | Purpose |
|------|---------|
| `reasoning.md` | Six-phase thinking discipline + backpropagation protocol |
| `workflow.md` | Spec lifecycle transitions + validation gates |
| `product.md` | Domain rules, target audience, business constraints |
| `tech.md` | Approved stack, languages, dependencies, testing frameworks |
| `structure.md` | File organization, directory structures, module boundaries |
| `memory.md` | Promoted learnings across specs |

### Role Personas (`.specd/roles/`)

| Role | Permissions | Responsibilities |
|------|-------------|-----------------|
| 🔍 `investigator` | Read-only | Explore code, trace paths, find integration points. Reports exact file/line refs. |
| 🛠️ `builder` | Write-only | Implement task contract. Modifies designated files + tests. Runs verify. |
| 🧪 `verifier` | Read-only | Runs tests independently. Captures output as evidence. |
| 🛡️ `reviewer` | Read-only | Audits git diffs. Logs issues with severity tags + exact locations. |

### Subagent Coordination Modes

Set in `.specd/config.json` via `roles.subagentMode`:

#### `inline` Mode (Default)
- Host agent performs work, swapping persona context inline
- **Pros**: Simple, works with any agent
- **Cons**: Context bloat from full chat history

#### `delegate` Mode
- Host spawns specialized subagents per role
- **Pros**: Isolated context, reduced token consumption
- **Cons**: Requires agent-spawning capabilities (Claude Code, etc.)

### Context Engineering (`specd context`)

Controls what enters the agent's context window:

```bash
specd context my-feature
```

Output sections:
1. **Phase Briefing**: Active phase rules (e.g., "You are in PLAN phase. Do not edit code.")
2. **Load List**: Minimal file list for context window
3. **Signals**:
   - Blockers: Currently blocked tasks + reasons
   - Awaiting Approval: Mid-req change locks
   - Uncovered Requirements: Requirements with no task mappings

---

## 11. Cross-Spec Programs

For multi-spec efforts, declare dependencies between specs:

```bash
# Declare dependency: 'api' spec waits for 'auth' spec
specd program link api --on auth

# Remove dependency
specd program unlink api --on auth

# View program-level DAG
specd program

# JSON output for orchestrators
specd program --json
```

Edges stored in `.specd/program.json`. Self-edges and cycles are rejected.

### Program-Level DAG

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│  auth   │────►│  api    │────►│  web    │
│ (Wave 1)│     │ (Wave 2)│     │ (Wave 3)│
└─────────┘     └─────────┘     └─────────┘
```

---

## 12. Configuration & Environment

### Environment Variables

| Variable | Default | Effect |
|----------|---------|--------|
| `SPECD_JSON` | `0` | Output structured JSON for all commands (also enabled by the `--json` flag) |
| `SPECD_LOCK_TIMEOUT_MS` | `5000` | Max wait for spec advisory lock |
| `SPECD_LOCK_STALE_MS` | `30000` | Age to auto-reclaim orphaned `.lock` files |
| `SPECD_VERIFY_TIMEOUT_MS` | `600000` | Per-run timeout for `specd verify` |
| `NO_COLOR` | — | Disable ANSI colors |

> The `--json` flag is equivalent to `SPECD_JSON=1` and is the recommended way to request machine-readable output.

### Exit Code Semantics

| Code | Meaning |
|------|---------|
| `0` | Success / validation passed |
| `1` | Validation gate failure / check failed |
| `2` | Usage error / CLI argument error |
| `3` | Root `.specd/` or spec slug not found |

### Config File (`.specd/config.json`)

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3,
  "gates": { "traceability": "warn", "acceptance": "off" }
}
```

| Key | Default | Effect |
|-----|---------|--------|
| `defaultVerify` | `npm test` | Fallback `verify:` command; `specd boot` overwrites it with the detected stack's test command |
| `report.format` | `md` | Default `specd report` format (`md` or `html`) |
| `report.autoRefreshSeconds` | `0` | HTML report auto-refresh interval (`0` = off) |
| `roles.subagentMode` | `inline` | `inline` or `delegate` subagent coordination |
| `promotionThreshold` | `3` | Recurrences before a learning is suggested for `memory promote` |
| `gates.traceability` | `warn` | `warn` or `error` — severity of the traceability gate |
| `gates.acceptance` | `off` | `off`, `warn`, or `error` — per-criterion acceptance gate |

---

## 13. Troubleshooting

### Common Errors

| Symptom | Cause & Fix |
|---------|-------------|
| `--status complete requires a passing specd verify` | No verify record. Run `specd verify <slug> <task>` first, or use `--unverified --evidence` for read-only tasks. |
| `verification is stale` | The `verify:` line changed since recording. Re-run `specd verify`. |
| `spec is gated (awaiting-approval)` | A `high`/`critical` `midreq` froze the spec. Review changes, then `specd approve`. |
| `exit 3` on any command | `.specd/` root or spec slug not found. Run `specd init`/`specd new`, or run from target repo. |
| `dependency cycle` / `depends on missing task` | DAG error in `tasks.md`. Fix `depends:` keys. Use `specd check` and `specd waves` to pinpoint. |
| CAS / revision write abort (`exit 1`) | Concurrent write clobber prevented. Re-read state and retry. |

### Agent-Specific Tips

- **Set `SPECD_JSON=1`** (or pass `--json`) for structured output parsing
- **Use `specd help --json`** to discover command schema programmatically
- **All state mutations are atomic and versioned** — safe for concurrent agent access
- **Never hand-edit `state.json`** — always use CLI commands

---

## 14. Contributor Guide

### Building from Source

No dependencies to install — Go stdlib only; templates are embedded via `go:embed`.

```bash
# Build the single binary (templates embedded, version stamped)
make build          # → ./specd   (go build -ldflags "-s -w -X main.version=...")

# Install into $GOBIN / $GOPATH/bin
make install

# Run the full test suite with the race detector
make test           # go test -race ./...

# Static analysis
make lint           # go vet ./...

# Run from source without building
go run . <command>  # e.g. go run . status
```

### Key Code Contracts

| File | Contract |
|------|----------|
| `internal/core/paths.go` | `FindSpecdRoot` walks up from cwd looking for `.specd/`. Callers return `NotFoundError` (exit `3`) if absent. |
| `internal/core/state.go` | `state.json` is machine truth. Load with `LoadState`, write with `SaveState` (atomic + CAS on `revision`). Never hand-edit. |
| `internal/core/io.go` | `AtomicWrite` (temp + fsync + rename) for every file write; ledgers append with `O_APPEND`. |
| `internal/core/lock.go` | `WithSpecLock[T]` wraps every mutating command in a reentrant per-spec advisory lock. |
| `internal/cmd/task.go` | Evidence gate. `--status complete` requires a passing `specd verify` record (or `--unverified --evidence` for read-only roles) AND all deps `complete`. Dual-writes `tasks.md` + `state.json` atomically. |
| `internal/core/tasksparser.go` | Bespoke line parser (`ParseTasksMd`). No external libs. Round-trip byte-stability tested. Returns `SpecdError(1)` with line number on errors. |

### Adding a Command

1. Create `internal/cmd/mycommand.go` with a handler:
   ```go
   func RunMyCommand(args cli.Args) int {
       // Implementation
       return core.ExitOK
   }
   ```
2. Register it in the `dispatch` switch in `main.go`
3. Add a `CommandMeta` entry in `internal/core/commands.go` (drives help + `--json` schema)
4. Add a co-located `mycommand_test.go` (or extend `internal/cmd/lifecycle_test.go`)

### Adding a Validation Gate

1. Write validation logic in the appropriate `internal/core/*.go` file
2. Wire it into `internal/cmd/check.go`
3. Add the gate transition in `internal/cmd/approve.go` if it blocks a phase
4. Add a test (`internal/cmd/commands_test.go` / `lifecycle_test.go`, or a core `*_test.go`)

### Code Style Invariants

1. **Zero Runtime Dependencies**: `go.mod` must list no `require` deps — stdlib only
2. **Atomic Writes**: Use `core.AtomicWrite` (temp + fsync + rename), never raw `os.WriteFile`
3. **Optimistic Concurrency**: Load `revision`, verify match, increment on write (CAS)
4. **Reentrant Locks**: Wrap mutating commands in `core.WithSpecLock`
5. **Round-Trip Stability**: `ParseTasksMd` must maintain 100% byte equivalence
6. **Embedded Templates**: Ship assets via `go:embed`, never read from disk relative to the binary

---

## 15. Use Cases & Workflows

### Use Case 1: Single Developer, Single Feature

```bash
# 1. Initialize project
specd init

# 2. Create spec
specd new auth --title "Implement JWT Authentication"

# 3. Write requirements
# Edit .specd/specs/auth/requirements.md
specd check auth
specd approve auth

# 4. Write design
# Edit .specd/specs/auth/design.md
specd check auth
specd approve auth

# 5. Write tasks
# Edit .specd/specs/auth/tasks.md
specd check auth
specd approve auth

# 6. Execute
specd next auth
# ... implement T1 ...
specd verify auth T1
specd task auth T1 --status complete

# 7. Close
specd approve auth
```

### Use Case 2: Parallel Subagent Execution

```bash
# 1. Get dispatch packets
specd dispatch my-feature --json

# 2. For each packet, spawn subagent with:
#    - rolePrompt
#    - contract, files, acceptance, verify
#    - completion command

# 3. Subagent runs:
#    - Implement task
#    - specd verify my-feature T1
#    - specd task my-feature T1 --status complete

# 4. Orchestrator monitors frontier and dispatches next wave
```

### Use Case 3: Multi-Spec Program

```bash
# 1. Create multiple specs
specd new backend-api --title "Backend API"
specd new frontend --title "React Frontend"
specd new auth --title "Auth Service"

# 2. Declare dependencies
specd program link backend-api --on auth
specd program link frontend --on backend-api

# 3. View program DAG
specd program

# 4. Execute in dependency order
# Auth → Backend API → Frontend
```

### Use Case 4: CI/CD Integration

```bash
# In CI pipeline:
specd check my-feature      # Validate spec structure
specd verify my-feature T1  # Run task verification
specd report my-feature --format html --out report.html  # Generate report

# Exit codes branch CI:
# 0 = continue, 1 = fail build, 2 = config error, 3 = missing files
```

### Use Case 5: Knowledge Management

```bash
# Record architectural decision
specd decision my-feature "Use Redis over Memcached for session storage"

# Log mid-flight requirement change
specd midreq my-feature "Add OAuth2 support" --impact high   --interpretation "Need third-party auth flow"   --changes "Add task T5, update design.md §Components"

# Record and promote learning
specd memory my-feature add --key "redis-pipeline"   --pattern "Batch Redis commands for performance"   --body "Use pipeline() for >3 commands in sequence"   --source "T3" --criticality important

specd memory my-feature promote --key "redis-pipeline"
```

---

## Appendix: File Reference

### Critical Files

| File | Purpose | Mutable By |
|------|---------|-----------|
| `.specd/state.json` | Global machine state | CLI only |
| `.specd/config.json` | Project configuration | Human |
| `.specd/boot.json` | Detected stack manifest | CLI only (`specd boot`) |
| `.specd/program.json` | Cross-spec dependencies | CLI only |
| `.specd/skills/**` | Companion skills (e.g. `specd-enrich`) | CLI (scaffold) |
| `.specd/specs/<slug>/state.json` | Spec-specific state | CLI only |
| `.specd/specs/<slug>/requirements.md` | EARS requirements | Human |
| `.specd/specs/<slug>/design.md` | Design document | Human |
| `.specd/specs/<slug>/tasks.md` | Task DAG | CLI dual-write |
| `.specd/specs/<slug>/decisions.md` | ADRs | CLI append |
| `.specd/specs/<slug>/mid-requirements.md` | Requirement updates | CLI append |
| `.specd/specs/<slug>/memory.md` | Local learnings | CLI append |
| `.specd/steering/*.md` | Constitution | Human |
| `.specd/roles/*.md` | Role prompts | Human |
| `AGENTS.md` | Agent workflow guide | Human (initial) |

---

*Generated for specd — the spec-driven coding harness.*  
*The agent reasons. The harness enforces.*
