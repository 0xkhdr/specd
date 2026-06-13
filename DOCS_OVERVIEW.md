# specd — Comprehensive Docs Overview & Knowledge Base

This document is a consolidated, complete reference of the architecture, design, philosophy, usage, CLI interface, and developer details of `specd`. It serves as the single source of truth and a blueprint for rebuilding the documentation.

---

## Table of Contents
1. [Core Identity & Mental Model](#1-core-identity--mental-model)
2. [The 8 Core Principles (Philosophy)](#2-the-8-core-principles-philosophy)
3. [The `.specd/` Directory Structure & Artifacts](#3-the-specd-directory-structure--artifacts)
4. [Validation Gates (`specd check`)](#4-validation-gates-specd-check)
5. [The Lifecycle & Status/Phase Mapping](#5-the-lifecycle--statusphase-mapping)
6. [Roles & Task Execution Personas](#6-roles--task-execution-personas)
7. [Concurrency & Durability Model](#7-concurrency--durability-model)
8. [CLI Command Reference](#8-cli-command-reference)
9. [Internal Architecture & Code Map](#9-internal-architecture--code-map)
10. [Documentation Redesign Recommendations](#10-documentation-redesign-recommendations)

---

## 1. Core Identity & Mental Model

`specd` is an **agent-agnostic, spec-driven coding harness**. It combines a structured spec workflow (requirements → design → tasks → execution) with a rigid thinking architecture. It is built around three elements:
1. **A directory convention** under `.specd/` inside target repositories that holds all configuration, specs, steering, role definitions, and state.
2. **A deterministic CLI** written in TypeScript that handles all bookkeeping, validation gates, task transitions, and report rendering with **zero LLM calls** and **zero runtime dependencies**.
3. **A prompt pack** (`AGENTS.md` + steering + role prompts) that teaches a host coding agent the workflow and constrains it to mutate state exclusively via the CLI.

### The Foundational Split
> **The agent reasons. The harness enforces.**

This split ensures that while the agent's work remains non-deterministic, process control remains strictly deterministic.

| The Agent Owns | The Harness (CLI) Owns |
|---|---|
| Understanding requirements & user intent | Recording the request's lifecycle |
| Writing requirements, designs, and tasks | Validating that artifacts are well-formed |
| Authoring code and unit tests | Refusing "done" without evidence |
| Deciding *what* to implement next | Computing *which task is runnable* in the DAG |
| Creative problem solving and judgment | Process integrity, state transitions, and durable truth |

---

## 2. The 8 Core Principles (Philosophy)

1. **The Foundational Split**: The agent reasons, the harness enforces. The harness behaves like a safety-critical control system, not an assistant.
2. **Specs Are the Source of Truth**: The plan does not sit in the agent's context window. It lives on disk as durable, versioned, human-readable markdown files. Markdown represents *intent*; `state.json` represents *status*.
3. **Evidence Gates Every State Change**: No task can be marked complete without providing non-empty evidence (e.g., commit SHA, test run outputs). *Trust is recorded, not assumed.*
4. **Waves, Not Lines**: Work is structured as a Directed Acyclic Graph (DAG) of concurrent batches (waves) rather than a flat todo list. The agent only works on the currently runnable frontier.
5. **Agent-Agnostic by Design**: Works with any agent capable of running shell commands (Claude Code, Cursor, Aider, etc.). Integration is managed purely via prompt files and steering.
6. **Human Gates at Phase Boundaries**: Automation checks syntactic correctness (gates); humans verify semantic intent via manual approval (`specd approve`).
7. **Deterministic Reporting**: Reports are generated programmatically from `state.json` and the markdown files, never from the agent's context or conversation history.
8. **Steering as Constitution**: Durable steering files (`.specd/steering/`) outlive individual sessions and align the agent structurally rather than conversationally.

---

## 3. The `.specd/` Directory Structure & Artifacts

All configuration, steering, roles, specs, and state are nested under `.specd/` in the target repository.

```
.specd/
├── config.json                    # Configuration tunables
├── steering/                      # Durable global context (the "Constitution")
│   ├── reasoning.md               # Six-phase thinking discipline
│   ├── workflow.md                # Spec lifecycle & gate rules
│   ├── product.md                 # Domain constraints and user context
│   ├── tech.md                    # Tech stack, patterns, and conventions
│   ├── structure.md               # File organization & module boundaries
│   └── memory.md                  # Promoted learnings across specs
├── roles/                         # Execution personas
│   ├── investigator.md            # Read-only research
│   ├── builder.md             # Code implementation (one task at a time)
│   ├── reviewer.md            # Read-only diff audit
│   └── verifier.md            # Non-modifying test runner
└── specs/<slug>/                  # One folder per spec/feature
    ├── requirements.md            # EARS-formatted user requirements
    ├── design.md                  # Architecture, interfaces, error handling
    ├── tasks.md                   # Wave DAG of tasks (CLI manages checkboxes)
    ├── decisions.md               # Numbered ADRs (Architecture Decision Records)
    ├── memory.md                  # Learnings logged for this spec
    ├── mid-requirements.md        # Feedback log for in-flight requirement changes
    └── state.json                 # Machine truth for state tracking (do not hand-edit)
```

### The Six Spec Artifacts
#### 1. `requirements.md` (Analyze Output)
Contains requirements organized in blocks. Each requirement has a `**User story:**` and a numbered list of `**Acceptance criteria:**` conforming to EARS grammar.
*   **Ubiquitous**: `THE SYSTEM SHALL <response>`
*   **Event-driven**: `WHEN <trigger> THE SYSTEM SHALL <response>`
*   **State-driven**: `WHILE <state> THE SYSTEM SHALL <response>`
*   **Optional-feature**: `WHERE <feature> THE SYSTEM SHALL <response>`
*   **Unwanted**: `IF <condition> THEN THE SYSTEM SHALL <response>`

#### 2. `design.md` (Plan Design Output)
Must contain all seven mandatory H2 headers, non-empty and free of `TODO` markers:
*   `## Overview`
*   `## Architecture`
*   `## Components and interfaces`
*   `## Data models`
*   `## Error handling`
*   `## Verification strategy`
*   `## Risks and open questions`

#### 3. `tasks.md` (Plan Tasks Output)
Defines the tasks DAG. A task is a checkbox item (`- [ ]`) under a `## Wave N` header, followed by a metadata block.
*   **Mandatory keys**: `why`, `role`, `files`, `contract`, `acceptance`, `verify`, `depends`.
*   **Optional key**: `requirements` (comma-separated requirement numbers).
*   *Note*: The CLI modifies checkboxes and appends evidence comments upon completion. The custom parser guarantees round-trip stability.

#### 4. `decisions.md` (ADRs)
Appends numbered ADRs using `specd decision`. Format:
*   `## ADR-NNN — <decision> · <date>`
*   `**Context:**` (Stubs `TODO`)
*   `**Decision:** <decision>`
*   `**Consequences:**` (Stubs `TODO`)
*   `**Supersedes:** —` (or another ADR identifier)

#### 5. `memory.md` (Spec Learnings)
Maintains learnings using `specd memory add`. Format:
*   `## <key>`
*   `**Pattern:** <one-line pattern>`
*   `**Detail:** <body>`
*   `**Source:** <commit/task source>`
*   `**Criticality:** minor | important | critical`
*   `**Related:** [[other-key]]`

#### 6. `mid-requirements.md` (Mid-flight Requirements Log)
Appends changes using `specd midreq`. Format:
*   `## Turn N — <timestamp> — impact: <low|medium|high|critical>`
*   `**User input (verbatim):** "..."`
*   `**Interpretation:** ...`
*   `**Impact:** ...`
*   `**Changes made:** ...`
*   `**Notes / open questions:** TODO`

---

### `state.json` (Machine Truth)
A JSON ledger updated only by the CLI. Current schema version is `2`.
```json
{
  "schemaVersion": 2,
  "revision": 0,
  "spec": "slug",
  "title": "Title",
  "status": "requirements | design | tasks | executing | verifying | complete | blocked",
  "phase": "analyze | plan | execute | verify | reflect",
  "gate": "none | awaiting-approval",
  "turn": 1,
  "createdAt": "ISO-TIMESTAMP",
  "updatedAt": "ISO-TIMESTAMP",
  "tasks": {
    "T1": {
      "id": "T1",
      "title": "Title",
      "role": "investigator | builder | reviewer | verifier",
      "wave": 1,
      "depends": [],
      "requirements": [1],
      "status": "pending | running | complete | blocked",
      "startedAt": null,
      "finishedAt": null,
      "evidence": null,
      "blocker": null
    }
  },
  "blockers": []
}
```
*   `revision`: CAS counter incremented on every write to prevent concurrent write clobbering.
*   `gate`: Bumps to `awaiting-approval` when high/critical `midreq` is recorded, freezing work.

---

### `config.json` (Tunables)
Created during `specd init`.
```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3
}
```
*   `defaultVerify`: Global fallback verify command used during the spec-level VERIFY phase.
*   `roles.subagentMode`: `inline` (same agent, persona swap) or `delegate` (spawn subagents).
*   `promotionThreshold`: Count of specs where a memory key must appear before promotion to steering.

---

## 4. Validation Gates (`specd check`)

`specd check <slug>` executes seven validation gates. Warnings do not cause exit code `1`, but failures do.

| Gate | Target | Severity | Checks Enforced |
|---|---|---|---|
| **1. EARS** | `requirements.md` | Fail | Every requirement has `**User story:**`, ≥1 acceptance criteria; all criteria conform to EARS grammar. |
| **2. Design** | `design.md` | Fail | All 7 mandatory H2 sections are present, non-empty, and contain no `TODO` markers. |
| **3. Task-schema** | `tasks.md` | Fail | ≥1 tasks present; every task has all 7 mandatory keys; `role` is valid; `verify` is defined (cannot be `N/A` for builder/verifier roles). |
| **4. DAG** | `tasks.md` | Fail | Dependency graph is acyclic, contains no orphan dependencies, and dependencies live in an earlier-or-equal wave. |
| **5. Evidence** | `state.json` | Fail | No task has status `complete` with empty or missing `evidence`. |
| **6. Sync** | `tasks.md` ↔ `state.json` | Fail | Checkboxes (`[x]` ↔ `complete`) and blocked annotations match `state.json`. |
| **7. Traceability** | `requirements.md` ↔ `tasks.md` | Fail / Warn | Fail if a task references a non-existent requirement. Warn if a requirement has no task referencing it. |

---

## 5. The Lifecycle & Status/Phase Mapping

```
INTAKE  ── specd new  ──▶  requirements  ── check/approve ──▶  design  ── check/approve ──▶  tasks  ── check/approve ──▶  executing
(no state)                   (ANALYZE)                        (PLAN)                         (PLAN)                      (EXECUTE)
                                                                                                                             │
                                                                     specd next / task complete ◀────────────────────────────┘
                                                                                             │
                                                                          all tasks complete ▼
                                                                                          verifying  ── approve  ──▶  complete
                                                                                           (VERIFY)    human accept    (REFLECT)
```

The CLI maps `status` in `state.json` to the current `phase` via the single source of truth `phaseForStatus()` in `src/core/phases.ts`:

| Spec `status` | Derived `phase` | Meaning / Activity |
|---|---|---|
| `requirements` | `analyze` | Authoring/refining requirements.md |
| `design` | `plan` | Authoring design.md |
| `tasks` | `plan` | Authoring tasks.md DAG |
| `executing` | `execute` | Executing tasks (at least one task has left `pending`) |
| `blocked` | `execute` | Execution halted; all remaining tasks are blocked |
| `verifying` | `verify` | All tasks complete; awaiting human sign-off (`specd approve`) |
| `complete` | `reflect` | Spec is closed; memory promotion and final snapshot reports are run |

### Planning Ratchet & Human Boundaries
Transitions between planning phases are enforced by the human running `specd approve`. The CLI will reject approval unless the gate for the active status is clean:
*   `requirements` → `design` requires EARS gate to pass.
*   `design` → `tasks` requires Design gate to pass.
*   `tasks` → `executing` requires Task-schema and DAG gates to pass.
*   `verifying` → `complete` requires human verification.

---

## 6. Roles & Task Execution Personas

During execution (`executing`), tasks are assigned to one of four personas via the `role` key in `tasks.md`.

```
[investigator] ──▶ maps extension point (read-only)
[builder]      ──▶ implements the changes (writes code, runs verify)
[verifier]     ──▶ runs tests, collects output (read-only)
[reviewer]     ──▶ audits diff for bugs (read-only)
```

### Role Definitions & Rules

#### 1. `investigator` (Read-only)
*   **Purpose**: Research, code tracing, and finding integration points.
*   **Rules**: Cannot write or modify code/files. Reports findings with exact `file:line` references.
*   **Verify**: Allowed `N/A` or empty.

#### 2. `builder` (Write-only)
*   **Purpose**: Code implementation.
*   **Rules**: Implements exactly the contract of one task. Touches only designated files and their test files. If blocked, stops after one retry and sets task to `blocked`. Captures verification output or hands it to the verifier.

#### 3. `verifier` (Read-only)
*   **Purpose**: Independent test execution and evidence collection.
*   **Rules**: Cannot modify code. Runs the exact `verify` command of the task. Reports pass/fail counts and verbatim output. Output forms the task's `--evidence`.

#### 4. `reviewer` (Read-only)
*   **Purpose**: Code audit.
*   **Rules**: Cannot modify code. Audits the diff. Logs issues with severity tags (`critical|high|medium|low`) in the format: `path:line: <severity>: <problem>. <fix>.`

---

## 7. Concurrency & Durability Model

specd supports parallel execution (e.g. running multiple builders on different frontier tasks of the same wave) through two layers:

1.  **Per-Spec Advisory Lock (`withSpecLock`)**:
    *   Mutation commands (`task`, `approve`, `midreq`, `memory`, `decision`) wrap their critical section in an `O_EXCL` lockfile at `.specd/specs/<slug>/.lock`.
    *   Lock acquisition has a timeout (`SPECD_LOCK_TIMEOUT_MS`, default 5s).
    *   Orphaned locks older than `SPECD_LOCK_STALE_MS` (default 30s) are reclaimed automatically.
    *   The lock is reentrant within the same process.
2.  **Optimistic Concurrency Control (CAS)**:
    *   `state.json` contains a `revision` counter.
    *   On write, the CLI checks if the revision on disk matches the revision loaded in memory. If they drift, the CLI aborts (exit `1`) to prevent lost updates.
3.  **Atomic Ledger Appends**:
    *   ADRs, memories, and mid-flight changes write directly to file descriptors using the OS `O_APPEND` flag. This guarantees that concurrent writes append cleanly without race conditions.
4.  **Atomic File Overwrites (`atomicWrite`)**:
    *   State files and task lists are written to a temp file, synced to disk (`fsync`), and atomically renamed (`rename`) over the target file, ensuring zero corruption on crashes.

---

## 8. CLI Command Reference

### Exit Code Contract
*   `0`: Success / valid
*   `1`: Gate or validation failure
*   `2`: Usage / CLI invocation error
*   `3`: Root `.specd/` directory or target spec not found

### Commands

*   `specd init [--force]`
    *   Scaffolds `.specd/steering/*`, `.specd/roles/*`, `.specd/config.json`, and root `AGENTS.md`. Idempotent. `--force` overwrites.
*   `specd new <slug> [--title "..."]`
    *   Scaffolds the spec directory structure. Status initialized to `requirements`.
*   `specd status [<slug>] [--json]`
    *   Without slug: lists all specs and their statuses. With slug: prints progress, wave graph with glyphs (`✓`, `◐`, `○`, `⚠`), blockers, and active gates.
*   `specd context <slug> [--json]`
    *   Context-engineering primitive. Prints phase briefing, indicating exactly which files the agent needs to load for the current phase to prevent context pollution.
*   `specd check <slug> [--json]`
    *   Executes all 7 validation gates.
*   `specd next <slug> [--all] [--json]`
    *   Default: prints the single next runnable task (lowest wave, then lowest ID) as a copy-pasteable prompt block. `--all`: prints the entire runnable frontier of tasks.
*   `specd task <slug> <id> --status <complete|blocked|running|pending> [--evidence "..."] [--reason "..."] [--force]`
    *   Sets task status. `--status complete` requires `--evidence` and all dependencies completed. `--status blocked` requires `--reason`.
*   `specd approve <slug> [--json]`
    *   Advances the planning ratchet if gates pass, clears `awaiting-approval` midreq gates, or signs off on `verifying` specs.
*   `specd decision <slug> "<text>" [--supersedes <ADR-id>]`
    *   Appends a numbered ADR to `decisions.md`.
*   `specd midreq <slug> "<verbatim>" --impact <low|medium|high|critical> [--interpretation "..."] [--changes "..."]`
    *   Logs in-flight changes. `high`/`critical` sets spec gate to `awaiting-approval`.
*   `specd memory <slug> add --key <k> --pattern "..." --body "..." --source "..." --criticality <minor|important|critical>`
    *   Appends learning to `memory.md`.
*   `specd memory <slug> promote --key <k> [--force]`
    *   Lifts local learning to `.specd/steering/memory.md` once it has occurred in enough specs (threshold default 3).
*   `specd report <slug> [--format md|html] [--out <path>]`
    *   Generates a deterministic summary report. HTML output is self-contained.
*   `specd waves <slug> [--json]`
    *   Renders the wave DAG, critical path list, and active blockers.

---

## 9. Internal Architecture & Code Map

### Codebase Layout
```
src/
├── cli.ts               # Command routing, usage info, exit codes
├── commands/            # One file per command (init, new, task, approve, check, etc.)
└── core/
    ├── paths.ts         # Path helpers, walks up to find .specd/ root
    ├── io.ts            # Atomic write and O_APPEND file handling
    ├── lock.ts          # Reentrant advisory locking
    ├── state.json       # Schema validation, save/load state, migrations
    ├── phases.ts        # Phase transitions, H2 design section checks
    ├── tasksParser.ts   # Custom tasks.md markdown parser/serializer
    ├── dag.ts           # DAG construction, next-runnable, cycle checks
    ├── ears.ts          # EARS requirements regex parser
    ├── report.ts        # Markdown/HTML report generation
    ├── specFiles.ts     # Spec-directory accessor, reconciler
    ├── render.ts        # Output formatters
    ├── templates.ts     # Template loader
    └── exit.ts          # Error classes & codes
```

### Common Extension File Map

| Developer Task | Files to Edit |
|---|---|
| Add or adjust a validation gate | `src/commands/check.ts`, `test/check.test.ts` |
| Edit task evidence/dependency check rules | `src/commands/task.ts`, `test/task.test.ts` |
| Modify phase-status logic or the VERIFY gate | `src/core/phases.ts`, `src/commands/task.ts`, `src/commands/approve.ts` |
| Adjust the `context` file-loading list | `src/commands/context.ts` |
| Fix `.specd/` folder root discovery | `src/core/paths.ts` |
| Add a new CLI command | `src/commands/<cmd>.ts`, `src/cli.ts` (routing), `test/` (new test suite) |
| Alter `state.json` schema | `src/core/state.ts` (increment schema version, write `migrate()` logic) |
| Fix markdown task list parsing | `src/core/tasksParser.ts`, `test/tasksParser.test.ts` |
| Modify DAG, waves, or critical path engine | `src/core/dag.ts`, `test/dag.test.ts` |
| Edit lock acquisition or CAS | `src/core/lock.ts`, `src/core/state.ts`, `test/concurrency.test.ts` |
| Edit file templates distributed to users | `src/templates/*` (*Note*: Rebuild to copy to `dist/templates/` before testing) |
| Adjust snapshot report layout | `src/core/report.ts`, `src/commands/report.ts`, `test/report.test.ts` |

### Key Developer Invariants
*   **Evidence gate constraint**: Completing a task requires both evidence and complete dependencies. Do not bypass.
*   **Atomic writes**: State and task lists must use `atomicWrite` (temp file → fsync → rename) to prevent corruption.
*   **Round-trip parsing stability**: Serialized tasks markdown must be byte-equivalent to parsed markdown.
*   **Sync constraint**: Box check state in markdown must align with status in JSON.
*   **Zero runtime dependencies**: Keep `dependencies` empty in `package.json`.
*   **Spec-level VERIFY constraint**: Completion of all tasks triggers `verifying` status. No auto-complete; require `approve`. Complete specs cannot be regressed.
*   **Single writer constraint**: State updates must go through `saveState` under lock.
*   **Exit code semantics**: Keep exit codes stable to prevent breaking CI/CD or wrapper scripts.

---

## 10. Documentation Redesign Recommendations

When rebuilding the repository documentation, address the following areas to improve clarity, organization, and developer experience:

1.  **Structure and Modularity**:
    *   Group documentation into three main target audiences:
        *   **User Guides**: Getting started, lifecycle walkthrough, and writing spec artifacts.
        *   **Agent Integration**: Setting up `AGENTS.md` and writing custom role prompts.
        *   **Contributor Guides**: Code base walkthrough, lock/state concurrency, and extending the harness (adding commands or validation gates).
    *   Keep documents focused on one topic rather than mixing internal architectures with basic usage.
2.  **Fix Stale References**:
    *   Several files refer to `CLAUDE.md` in the root repository. However, `CLAUDE.md` does not exist in the root of the source tree. Either restore it with correct developer context (e.g., test instructions, file mapping) or clean up links pointing to it.
    *   Clearly distinguish the two `AGENTS.md` files: the one at the root of the specd repository (how to develop specd) versus the template `src/templates/AGENTS.md` (emitted into target repos for agents to run specd).
3.  **Visual Elements and Diagrams**:
    *   Include Mermaid flowcharts for the status/phase transitions, concurrency lock flow, and the pipeline of validation gates.
    *   Provide side-by-side visual comparisons of `tasks.md` checkboxes next to `state.json` task statuses.
4.  **Formatting and Interactive Examples**:
    *   Ensure all CLI examples highlight the exit code transitions.
    *   Provide templates and examples for writing EARS criteria and task schemas directly.
