# 4. The Spec Folder

Everything `specd` knows lives under `.specd/`. This document is the reference for every file: its
purpose, its format, and which parts you author vs. which the CLI owns.

## Directory layout

```
.specd/
├── config.json                    # tunables (CLI-read)
├── steering/                      # the constitution — durable across all specs (you author)
│   ├── reasoning.md
│   ├── workflow.md
│   ├── product.md
│   ├── tech.md
│   ├── structure.md
│   └── memory.md
├── roles/                         # execution personas (you author / customize)
│   ├── investigator.md
│   ├── builder.md
│   ├── reviewer.md
│   └── verifier.md
└── specs/<slug>/                  # one folder per feature
    ├── requirements.md            # you author (EARS)
    ├── design.md                  # you author
    ├── tasks.md                   # you author the prose; CLI flips the checkboxes
    ├── decisions.md               # CLI appends (specd decision)
    ├── memory.md                  # CLI appends (specd memory)
    ├── mid-requirements.md        # CLI appends (specd midreq)
    └── state.json                 # CLI-owned machine truth — never hand-edit
```

**Portability:** `.specd/` is self-contained. Removing it removes all harness state with no side
effects outside the directory. The repo-root `AGENTS.md` is prompt-only and carries no durable state.

## Authored truth vs. machine truth

| You author (intent) | CLI owns (status) |
|---------------------|-------------------|
| `requirements.md`, `design.md`, the prose/metadata of `tasks.md`, the steering and role files | `state.json`, the *checkboxes and annotations* in `tasks.md`, and the appended entries in `decisions.md` / `memory.md` / `mid-requirements.md` |

The CLI keeps `tasks.md` checkboxes and `state.json` in sync on every load (`reconcile()`); drift is
gate-6 failure.

---

## The six spec artifacts

### `requirements.md` — EARS user stories

The ANALYZE output. Each requirement is an `## Requirement N` block with a `**User story:**` line and
an `**Acceptance criteria:**` numbered list. Every criterion must match an EARS pattern.

```markdown
## Requirement 1 — Config loading

**User story:** As a developer, I want config loaded from a file so settings persist.

**Acceptance criteria:**
1. WHEN the CLI starts THE SYSTEM SHALL read config.json from the working directory.
2. IF config.json is missing THEN THE SYSTEM SHALL fall back to documented defaults.
```

**EARS patterns** (case-insensitive; checked by the EARS gate):

| Pattern | Grammar |
|---------|---------|
| Ubiquitous | `THE SYSTEM SHALL <response>` |
| Event-driven | `WHEN <trigger> THE SYSTEM SHALL <response>` |
| State-driven | `WHILE <state> THE SYSTEM SHALL <response>` |
| Optional-feature | `WHERE <feature> THE SYSTEM SHALL <response>` |
| Unwanted | `IF <condition> THEN THE SYSTEM SHALL <response>` |

The gate fails if: a requirement has no `**User story:**` line, has zero criteria, a criterion
matches no pattern, or the file has no `## Requirement N` sections at all.

### `design.md` — the architecture

The PLAN (design) output. It must contain **all seven** of these H2 sections, each non-empty and free
of `TODO` markers:

```
## Overview
## Architecture
## Components and interfaces
## Data models
## Error handling
## Verification strategy
## Risks and open questions
```

The design gate reports any missing, empty, or TODO-bearing section with a line number.

### `tasks.md` — the wave DAG

The PLAN (tasks) output and the heart of execution. Tasks are checkbox items grouped under `## Wave N`
headers, each followed by a metadata block. **Seven keys are mandatory**; `requirements` is optional
but recommended (it powers the traceability gate).

```markdown
## Wave 1
- [ ] T1 — short imperative title
  - why: the reason this task exists (tie to a requirement)
  - role: investigator | builder | reviewer | verifier
  - files: path/a.ts, path/b.ts
  - contract: exactly what to do and what NOT to do
  - acceptance: observable criteria that make this done
  - verify: shell command (or N/A only for read-only roles)
  - depends: T-ids, or —
  - requirements: 1, 2
```

Rules enforced by the task-schema and DAG gates:

- **Seven mandatory keys:** `why, role, files, contract, acceptance, verify, depends`.
- **Valid role:** one of `investigator, builder, reviewer, verifier`.
- **`verify` must be a command** unless the role is read-only (`investigator`/`reviewer`), in which
  case `N/A` (or empty) is allowed.
- **Acyclic** dependency graph; **no orphan deps** (every `depends` id must exist); every dependency
  must live in an **earlier-or-equal wave**.

> **Never hand-edit the `[ ]` / `[x]` checkbox or the evidence annotation.** The CLI dual-writes them
> via `specd task`. The parser guarantees round-trip stability, so your prose and comments survive
> every flip.

### `decisions.md` — ADRs

Append-only via `specd decision <slug> "<text>" [--supersedes ADR-NNN]`. Each entry is auto-numbered:

```markdown
## ADR-001 — <decision text> · 2026-06-13
**Context:** TODO
**Decision:** <decision text>
**Consequences:** TODO
**Supersedes:** —
```

The `Context` and `Consequences` fields are stubbed `TODO` for you to fill in.

### `memory.md` — source-attributed learnings

Append via `specd memory <slug> add`. Each entry:

```markdown
## <key>
**Pattern:** <one-line pattern>
**Detail:** <body>
**Source:** <where it came from, e.g. a commit or task>
**Criticality:** minor | important | critical
**Related:** [[other-key]], …
```

`specd memory <slug> promote --key <key>` lifts an entry into `steering/memory.md` once it has
recurred across `promotionThreshold` specs (default 3; `--force` overrides).

### `mid-requirements.md` — the feedback log

Append-only via `specd midreq`. Each entry:

```markdown
## Turn 3 — 2026-06-13T14:22 — impact: high
**User input (verbatim):** "Also support YAML config"
**Interpretation:** Add a YAML branch to the loader
**Impact:** high
**Changes made:** New task T4 under Wave 1
**Notes / open questions:** TODO
```

---

## `state.json` — machine truth (do not hand-edit)

The CLI-owned durable ledger. It is the single source of truth for status; the markdown files are
the source of truth for intent. Shape (current `schemaVersion` is **2**):

```jsonc
{
  "schemaVersion": 2,
  "revision": 7,                 // optimistic-concurrency counter, bumped on every save
  "spec": "my-feature",
  "title": "My Feature",
  "status": "executing",         // requirements|design|tasks|executing|verifying|complete|blocked
  "phase": "execute",            // derived from status — never set independently
  "gate": "none",                // none | awaiting-approval
  "turn": 3,                     // bumped by midreq
  "createdAt": "2026-06-13T12:00:00.000Z",
  "updatedAt": "2026-06-13T14:30:00.000Z",
  "tasks": {
    "T1": {
      "id": "T1",
      "title": "Parse config.json",
      "role": "builder",
      "wave": 1,
      "depends": [],
      "requirements": [1],
      "status": "complete",
      "startedAt": "2026-06-13T13:00:00.000Z",
      "finishedAt": "2026-06-13T13:20:00.000Z",
      "evidence": "commit a1b2c3d; npm test -- config PASS (12/12)",
      "blocker": null
    }
  },
  "blockers": [
    { "task": "T5", "reason": "API key missing in CI", "since": "Turn 3" }
  ]
}
```

Key fields:

- **`revision`** — bumped on every `saveState`. Before writing, the CLI compares the on-disk revision
  to the one it loaded; a mismatch means a concurrent write slipped in, so the save aborts (exit 1)
  rather than clobber. Defense-in-depth behind the per-spec lock.
- **`phase`** — always derived from `status` via `phaseForStatus()`. Stored for convenience, never
  authoritative on its own.
- **`gate`** — `awaiting-approval` is set by a high/critical `midreq` and cleared by `specd approve`.
- **`tasks[*].evidence`** — the proof captured when a task was completed.

### Schema migration

`SCHEMA_VERSION` is bumped when the shape changes, with a `migrate()` branch that upgrades older files
in place. v1 → v2 added `revision` (defaulting to `0`). A file whose `schemaVersion` is *newer* than
the running CLI is rejected with a "upgrade specd" error rather than misread.

---

## `config.json` — tunables

Written by `specd init`. Defaults:

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3
}
```

| Key | Meaning |
|-----|---------|
| `defaultVerify` | the spec-level verification command suggested in the VERIFY phase (`specd context`) |
| `report.format` | default `specd report` format (`md` or `html`) |
| `report.autoRefreshSeconds` | reserved for live-report refresh |
| `roles.subagentMode` | `inline` (run roles in the same agent) or `delegate` (use the host's native subagents) — see [Agent Integration](agent-integration.md) |
| `promotionThreshold` | how many specs a pattern must appear in before `memory promote` accepts it (≤1 disables the gate) |

---

## Root discovery

Every command that touches a repo calls `requireSpecdRoot()`, which walks **up** from the current
working directory looking for a `.specd/` folder — so you can run `specd` from any subdirectory of
the repo, exactly like `git`. If none is found it exits `3` (not found). All other paths
(`steeringDir`, `rolesDir`, `specsDir`, `specDir`) are derived from that root.
