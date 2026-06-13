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
      "blocker": null,
      "verification": {
        "command": "npm test",
        "exitCode": 0,
        "verified": true,
        "timedOut": false,
        "stdoutTail": "...",
        "stderrTail": "...",
        "durationMs": 1234,
        "ranAt": "ISO-TIMESTAMP",
        "gitHead": "abc1234"
      }
    }
  },
  "acceptance": {
    "1.2": {
      "requirement": 1,
      "criterion": 2,
      "status": "pass | fail",
      "evidence": "...",
      "ranAt": "ISO-TIMESTAMP"
    }
  },
  "blockers": []
}
```
*   `revision`: CAS counter incremented on every write to prevent concurrent write clobbering.
*   `gate`: Bumps to `awaiting-approval` when high/critical `midreq` is recorded, freezing work.
*   `tasks.<id>.verification`: The `VerificationRecord` written by `specd verify <slug> <id>`. specd spawns the task's `verify:` command, captures the OS exit code, output tails, duration, and git HEAD. `specd task --status complete` requires `verified: true` and a `command` matching the current `verify:` line (otherwise the record is **stale** and must be re-run). `null` until verified.
*   `acceptance.<req>.<n>`: Per-criterion proofs written by `specd verify <slug> --criterion <req>.<n>`. Consulted by `specd approve` only when `config.gates.acceptance` is `required`.

---

### `config.json` (Tunables)
Created during `specd init`.
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
*   `defaultVerify`: Global fallback verify command used during the spec-level VERIFY phase.
*   `roles.subagentMode`: `inline` (same agent, persona swap) or `delegate` (spawn subagents).
*   `promotionThreshold`: Count of specs where a memory key must appear before promotion to steering.
*   `gates.traceability`: `warn` (default) or `error` — severity when a requirement has no task referencing it.
*   `gates.acceptance`: `off` (default) or `required` — when `required`, `specd approve` refuses to close a `verifying` spec until every requirement has a passing per-criterion proof (see `specd verify --criterion`).

---

## 4. Validation Gates (`specd check`)

`specd check <slug>` executes seven validation gates. Warnings do not cause exit code `1`, but failures do.

| Gate | Target | Severity | Checks Enforced |
|---|---|---|---|
| **1. EARS** | `requirements.md` | Fail | Every requirement has `**User story:**`, ≥1 acceptance criteria; all criteria conform to EARS grammar. |
| **2. Design** | `design.md` | Fail | All 7 mandatory H2 sections are present, non-empty, and contain no `TODO` markers. |
| **3. Task-schema** | `tasks.md` | Fail | ≥1 tasks present; every task has all 7 mandatory keys; `role` is valid; `verify` is defined (cannot be `N/A` for builder/verifier roles). |
| **4. DAG** | `tasks.md` | Fail | Dependency graph is acyclic, contains no orphan dependencies, and dependencies live in an earlier-or-equal wave. |
| **5. Evidence** | `state.json` | Fail | No task is `complete` with empty/missing `evidence`; and no non-read-only task is `complete` without a passing `verification` record (run `specd verify`). |
| **6. Sync** | `tasks.md` ↔ `state.json` | Fail | Checkboxes (`[x]` ↔ `complete`) and blocked annotations match `state.json`. |
| **7. Traceability** | `requirements.md` ↔ `tasks.md` | Fail / Warn | Fail if a task references a non-existent requirement. Forward direction (requirement with no task) severity is set by `config.gates.traceability` — `warn` (default) or `error`. |

> **Spec-level acceptance (optional gate):** When `config.gates.acceptance` is `required`, `specd approve` will not advance a `verifying` spec to `complete` until every requirement has a passing per-criterion proof recorded via `specd verify <slug> --criterion <req>.<n> --status pass`. This is enforced at the `approve` boundary, not by `specd check`.

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
*   `specd dispatch <slug> [--json]`
    *   Emits ready-to-run **dispatch packets** for the runnable frontier — each packet bundles the resolved role prompt, contract, files, acceptance, verify command, and the exact completion command. `--json` is the full fan-out payload for an orchestrator spawning parallel subagents; text mode prints a compact summary. Honors the `awaiting-approval` gate (override with `--force`).
*   `specd verify <slug> <id>`
    *   Deterministically **runs** the task's `verify:` command (via the shell, in the repo root), capturing exit code, output tails, duration, and git HEAD into the task's `verification` record. specd makes zero judgment — exit 0 → `verified: true`. Required before `specd task <id> --status complete` for any task with a runnable verify line. Exit `0` if verified, `1` if the command failed/timed out. Timeout via `SPECD_VERIFY_TIMEOUT_MS` (default 600s).
*   `specd verify <slug> --criterion <req>.<n> --status <pass|fail> --evidence "..."`
    *   Records a **per-criterion acceptance proof** into `state.acceptance` (the spec-level VERIFY beat). The requirement must exist in `requirements.md`. `--evidence` is mandatory. Exit `0` for `pass`, `1` for `fail`. Consulted by `specd approve` when `config.gates.acceptance` is `required`.
*   `specd task <slug> <id> --status <complete|blocked|running|pending> [--evidence "..."] [--unverified] [--reason "..."] [--force]`
    *   Sets task status. `--status complete` requires all dependencies complete **and** a passing `specd verify` record whose command still matches the current `verify:` line; otherwise pass `--unverified --evidence "<proof>"` (the manual escape hatch for read-only roles or genuinely manual proofs). `--status blocked` requires `--reason`. Refuses while the spec gate is `awaiting-approval` unless `--force`.
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
*   `specd program [status] [--json]`
    *   The **cross-spec / program view**. Projects every spec as a node in a spec-level DAG (edges from `.specd/program.json`) and answers "which whole specs are runnable across the program right now?". Renders waves, runnable frontier, critical path, orphan edges, and cycles. Exit `1` if a cycle exists.
*   `specd program link <spec> --on <dep>` / `specd program unlink <spec> --on <dep>`
    *   Adds/removes a cross-spec dependency edge in `.specd/program.json`. Both specs must exist; self-edges and edges that would form a cycle are rejected (exit `1`/`2`).

### Environment Variables
*   `SPECD_LOCK_TIMEOUT_MS` (default `5000`): Max wait to acquire a spec advisory lock before failing.
*   `SPECD_LOCK_STALE_MS` (default `30000`): Age past which an orphaned lockfile is reclaimed.
*   `SPECD_VERIFY_TIMEOUT_MS` (default `600000`): Per-run timeout for `specd verify`; a timed-out run records `verified: false` with exit `124`.

---

## 9. Internal Architecture & Code Map

### Codebase Layout
```
src/
├── cli.ts               # Command routing, usage info, exit codes
├── commands/            # One file per command:
│   │                    #   init, new, status, context, check, next, dispatch,
│   │                    #   program, verify, task, approve, decision, midreq,
│   │                    #   memory, report, waves
└── core/
    ├── paths.ts         # Path helpers, walks up to find .specd/ root
    ├── io.ts            # Atomic write and O_APPEND file handling
    ├── lock.ts          # Reentrant advisory locking
    ├── state.ts         # Schema validation, save/load state, CAS, migrations, VerificationRecord/CriterionRecord
    ├── phases.ts        # Phase transitions, H2 design section checks
    ├── tasksParser.ts   # Custom tasks.md markdown parser/serializer
    ├── dag.ts           # DAG construction, next-runnable, frontier, critical path, cycle checks
    ├── program.ts       # Cross-spec program model: projects specs as a spec-level DAG
    ├── ears.ts          # EARS requirements regex parser
    ├── report.ts        # Markdown/HTML report generation
    ├── specFiles.ts     # Spec-directory accessor, reconciler, role/artifact readers
    ├── render.ts        # Output formatters, acceptance/requirement helpers
    ├── md.ts            # Shared markdown scanning helpers
    ├── output.ts        # Redirectable raw stdout sink (capture-safe)
    ├── templates.ts     # Template loader
    └── exit.ts          # Error classes & codes
```

> The `.specd/` directory also gains a root `program.json` manifest (cross-spec dependency edges) once `specd program link` is used. Per-task `verification` records and the per-criterion `acceptance` ledger live inside each spec's `state.json`.

### Common Extension File Map

| Developer Task | Files to Edit |
|---|---|
| Add or adjust a validation gate | `src/commands/check.ts`, `test/check.test.ts` |
| Edit task evidence/dependency check rules | `src/commands/task.ts`, `test/task.test.ts` |
| Modify phase-status logic or the VERIFY gate | `src/core/phases.ts`, `src/commands/task.ts`, `src/commands/approve.ts` |
| Adjust the `context` file-loading list | `src/commands/context.ts` |
| Fix `.specd/` folder root discovery | `src/core/paths.ts` |
| Add a new CLI command | `src/commands/<cmd>.ts`, `src/cli.ts` (routing + `USAGE`), `test/` (new test suite) |
| Change the verify runner or VerificationRecord | `src/commands/verify.ts`, `src/core/state.ts`, `test/verify.test.ts` |
| Change dispatch packet shape | `src/commands/dispatch.ts`, `test/dispatch.test.ts` |
| Edit cross-spec program DAG / edges | `src/core/program.ts`, `src/commands/program.ts`, `test/program.test.ts` |
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

## 10. Documentation Map & Status

The repository documentation is split by audience. This file (`DOCS_OVERVIEW.md`) is the consolidated source-of-truth reference; the `docs/` guides are the task-focused entry points.

| Document | Audience | Covers |
|---|---|---|
| `README.md` | Everyone | Identity, features, quick start, doc map. |
| `docs/README.md` | Everyone | Index/navigation across the three guides. |
| `docs/user-guide.md` | Users of specd in a target repo | Lifecycle, writing artifacts, verify→complete flow, dispatch/program, troubleshooting. |
| `docs/agent-integration.md` | Agent/harness integrators | The two `AGENTS.md` files, steering, roles, subagent + dispatch orchestration, context engineering. |
| `docs/contributor-guide.md` | specd contributors | Codebase walkthrough, gate pipeline, concurrency, parser internals, extension recipes. |
| `CLAUDE.md` | Contributors/agents on the repo | Build/test commands and the invariants checklist. |
| `AGENTS.md` (root) | Agents developing specd | Same as CLAUDE.md, agent-shaped. |

### Recommendations status (formerly redesign backlog)
1.  **Audience split** — ✅ Done: `docs/` is split into user / agent-integration / contributor guides, each single-topic.
2.  **Stale references** — ✅ Done: `CLAUDE.md` now exists at the root; the two `AGENTS.md` files are distinguished in `docs/agent-integration.md §1`.
3.  **Visual elements** — ✅ Done: Mermaid flowcharts for status/phase transitions, the lock flow, and the gate pipeline live in the guides; a checkbox↔`state.json` table is in `docs/user-guide.md §4`.
4.  **Examples & exit codes** — ✅ Done: CLI examples in the user guide annotate exit-code transitions, with EARS, design, and task-schema templates inline.

### Remaining opportunities
*   Add an end-to-end recorded walkthrough (asciinema/GIF) of a full spec from `new` to `complete`.
*   Document `delegate` subagent mode against a concrete host harness once a reference integration exists.
