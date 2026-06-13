# 2. Getting Started

This guide takes you through a **complete spec lifecycle** — from an empty repo to a verified,
complete feature with a deterministic report. Every command here is real and the output is accurate.

## Prerequisites

- **Node ≥18.** That is the only requirement. specd has zero runtime dependencies.

## Install

```sh
# Run directly (recommended):
npx specd <command>

# Or from a clone:
git clone https://github.com/0xkhdr/specd.git
cd specd
npm install && npm run build
node dist/cli.js <command>
```

Confirm it works:

```sh
npx specd --version
npx specd --help
```

## Step 0 — Scaffold the harness

Run this once at the root of the repo you want to manage:

```sh
specd init
```

This is **idempotent** — it writes the harness files and skips any that already exist (use `--force`
to overwrite). It creates:

```
.specd/
├── config.json                # tunables (defaultVerify, promotionThreshold, roles, report)
├── steering/                  # the "constitution" — durable context across all specs
│   ├── reasoning.md           #   the six-phase thinking architecture
│   ├── workflow.md            #   the spec lifecycle and gate rules
│   ├── product.md             #   domain constraints, user context
│   ├── tech.md                #   stack, patterns, conventions
│   ├── structure.md           #   file organization, module boundaries
│   └── memory.md              #   promoted learnings across specs
└── roles/                     # the four execution personas
    ├── investigator.md        #   read-only research
    ├── builder.md             #   write exactly one task
    ├── reviewer.md            #   read-only defect audit
    └── verifier.md            #   run checks, capture evidence
AGENTS.md                      # repo-root prompt file that teaches the host agent the workflow
```

`AGENTS.md` is read automatically by Claude Code, Cursor, Aider, Codex, and others. It is the
portability mechanism — see [Agent Integration](agent-integration.md).

## Step 1 — Create a spec

```sh
specd new my-feature --title "My Feature"
```

The slug must match `^[a-z0-9][a-z0-9-]*$`. This scaffolds `.specd/specs/my-feature/` with six
artifact stubs plus a CLI-owned `state.json`:

```
.specd/specs/my-feature/
├── requirements.md        # EARS-formatted user stories + acceptance criteria
├── design.md              # architecture, components, data, errors, verify strategy, risks
├── tasks.md               # the wave DAG of executable tasks
├── decisions.md           # numbered ADRs (Architecture Decision Records)
├── memory.md              # source-attributed learnings
├── mid-requirements.md    # log of in-flight requirement changes
└── state.json             # machine truth — never hand-edit
```

The new spec starts at `status: requirements`, `phase: analyze`.

## Step 2 — ANALYZE: write requirements (EARS)

Open `requirements.md` and author acceptance criteria in **EARS** form. Each requirement needs a
`**User story:**` line and at least one criterion, and every criterion must match an EARS pattern:

```markdown
## Requirement 1 — Config loading

**User story:** As a developer, I want config loaded from a file so settings persist.

**Acceptance criteria:**
1. WHEN the CLI starts THE SYSTEM SHALL read config.json from the working directory.
2. IF config.json is missing THEN THE SYSTEM SHALL fall back to documented defaults.
3. THE SYSTEM SHALL reject a config.json with an unknown schemaVersion.
```

The five EARS patterns (see [Validation Gates](validation-gates.md) for details):

| Pattern | Shape |
|---------|-------|
| Ubiquitous | `THE SYSTEM SHALL …` |
| Event-driven | `WHEN … THE SYSTEM SHALL …` |
| State-driven | `WHILE … THE SYSTEM SHALL …` |
| Optional-feature | `WHERE … THE SYSTEM SHALL …` |
| Unwanted | `IF … THEN THE SYSTEM SHALL …` |

Now gate it:

```sh
specd check my-feature
```

`specd check` runs all seven validation gates and exits `0` only if valid. Fix any reported
violation (each is printed as `fail  location: message (gate)`), then approve:

```sh
specd approve my-feature        # requirements → design
```

`approve` is the **human boundary**. It advances the planning phase only once the gate for *that
phase's* artifact is green.

## Step 3 — PLAN (design): write the design

Fill every required H2 section of `design.md` — each must be non-empty and free of `TODO` markers:

```
## Overview
## Architecture
## Components and interfaces
## Data models
## Error handling
## Verification strategy
## Risks and open questions
```

Gate and approve:

```sh
specd check my-feature
specd approve my-feature        # design → tasks
```

## Step 4 — PLAN (tasks): write the wave DAG

Author `tasks.md` as a DAG grouped by wave. **Every task carries seven mandatory keys** plus an
optional `requirements` back-reference:

```markdown
## Wave 1
- [ ] T1 — Parse config.json
  - why: requirement 1 needs config loaded at startup
  - role: builder
  - files: src/config.ts
  - contract: read config.json from cwd; return typed Config; do NOT touch CLI args
  - acceptance: config.test.ts passes; unknown schemaVersion throws
  - verify: npm test -- config
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T2 — Wire config into startup
  - why: requirement 1 requires the loaded config to take effect
  - role: builder
  - files: src/cli.ts
  - contract: load config at boot; pass into command dispatch
  - acceptance: e2e startup test reads a custom value from config.json
  - verify: npm test -- startup
  - depends: T1
  - requirements: 1
```

Rules enforced by the DAG and task-schema gates:

- Seven mandatory keys: `why, role, files, contract, acceptance, verify, depends`.
- `role` ∈ `{investigator, builder, reviewer, verifier}`.
- `verify` must be a real command unless the role is read-only (`investigator`/`reviewer`).
- The dependency graph must be **acyclic**, have **no orphan deps**, and every dep must live in an
  **earlier-or-equal wave**.

> **Do not hand-edit the checkboxes.** Flip them only with `specd task`.

Gate and approve into execution:

```sh
specd check my-feature
specd approve my-feature        # tasks → executing
```

## Step 5 — EXECUTE: the build loop

The execution loop is: orient → get a task → build it → prove it → record.

```sh
specd context my-feature        # phase-scoped briefing: what to load now + the next action
specd next my-feature           # the single next runnable task, as a paste-ready prompt block
```

`specd next` prints a focused block — title, role, why, files, contract, acceptance, verify, depends.
Adopt the assigned role (see [Agent Integration](agent-integration.md)), implement **only** that
task, run its `verify` line, and capture the result.

Then flip the task — this is the evidence gate:

```sh
specd task my-feature T1 --status complete --evidence "commit a1b2c3d; npm test -- config PASS (12/12)"
```

- `--status complete` **requires** non-empty `--evidence`, or it exits `1`.
- It will refuse if any dependency is still incomplete (exit `1`).
- It dual-writes `tasks.md` (checks the box, annotates evidence) and `state.json` atomically.

Other task transitions:

```sh
specd task my-feature T3 --status running                         # mark in-progress
specd task my-feature T3 --status blocked --reason "API key missing in CI"
```

`blocked` requires a `--reason`; the workflow rule is **one retry, then block and surface it.** Keep
looping `specd next` until it reports all tasks complete.

To fan work out in parallel, use the runnable frontier:

```sh
specd next my-feature --all     # every currently-runnable task, for parallel dispatch
```

## Step 6 — VERIFY: the spec-level gate

When the **last** task completes, the spec does not auto-finish. It enters `status: verifying`
(phase VERIFY). This is a spec-level mirror of the per-task evidence gate: every task is done, but a
human must still confirm the feature actually works as a whole.

```sh
specd context my-feature        # tells you to run the config defaultVerify and check acceptance
# run your full verification (e.g. npm test), confirm acceptance criteria hold, then:
specd approve my-feature        # verification accepted → status complete (phase REFLECT)
```

`approve` is the **only** command that advances `verifying → complete`. specd never auto-completes a
spec.

## Step 7 — REFLECT: capture and report

Record durable learnings, then snapshot:

```sh
specd memory my-feature add --key "config-fallback" \
  --pattern "missing config → documented defaults" \
  --body "Loader returns defaults rather than throwing; tested in config.test.ts" \
  --source "T1 commit a1b2c3d" --criticality important

specd memory my-feature promote --key "config-fallback"   # promote across specs (threshold-gated)

specd report my-feature                     # deterministic markdown snapshot
specd report my-feature --format html --out report.html   # single dependency-free HTML file
```

## The full lifecycle at a glance

```
new ─▶ requirements ─check/approve─▶ design ─check/approve─▶ tasks ─check/approve─▶ executing
 │        (ANALYZE)                   (PLAN)                  (PLAN)                  (EXECUTE)
 │                                                                                       │
 │                                          next / next --all ◀──────────────────────────┘
 │                                          task <id> --status complete --evidence "..."
 │                                                              │
 │                                       all tasks complete ────▼
 │                                                          verifying ─approve─▶ complete
 │                                                           (VERIFY)            (REFLECT)
 │                                                                                  │
 └──────────────────────── specd report / specd memory promote ────────────────────┘
```

## Mid-flight changes

If the user sends new input mid-execution, log it rather than silently absorbing it:

```sh
specd midreq my-feature "Also support YAML config" --impact high \
  --interpretation "Add a YAML branch to the loader" \
  --changes "New task T4 under Wave 1"
```

`high`/`critical` impact sets `gate = awaiting-approval` — `specd next` and `specd task` then refuse
to hand out or flip work until a human runs `specd approve` to clear the gate. `low`/`medium` just
log a turn entry. See [CLI Reference](cli-reference.md#midreq).

## Next steps

- [Core Concepts](concepts.md) — the vocabulary behind everything above.
- [CLI Reference](cli-reference.md) — every command and flag.
- [The Spec Folder](spec-anatomy.md) — the exact format of every artifact.
