# User Guide

Using `specd` inside a target repository — from install through a closed spec.
For the *why*, read [Concepts](./concepts.md); for command-level detail, the
[Command Reference](./command-reference.md).

## Contents

1. [Installation](#installation)
2. [Initializing a project](#initializing-a-project)
3. [The spec lifecycle](#the-spec-lifecycle)
4. [Writing spec artifacts](#writing-spec-artifacts)
5. [Task execution and evidence](#task-execution-and-evidence)
6. [Reporting](#reporting)
7. [Configuration](#configuration)
8. [Troubleshooting](#troubleshooting)

---

## Installation

`specd` builds from source with the Go toolchain — there is no package manager or
install script. It compiles to a single static binary with zero runtime dependencies.

### Build from source

```bash
git clone https://github.com/0xkhdr/specd.git
cd specd

go build -o specd main.go      # produces ./specd in the current directory
```

### Install onto your PATH

```bash
go install github.com/0xkhdr/specd@latest   # installs `specd` into $(go env GOBIN)
```

Ensure `$(go env GOPATH)/bin` (or `GOBIN`) is on your `PATH`. To uninstall, remove the
binary: `rm "$(command -v specd)"`.

### Requirements

- Go 1.26+ to build (no runtime deps once built).
- Linux, macOS, or Windows (amd64 / arm64).
  - *Windows*: verification commands require `sh` or `bash` (e.g. Git for Windows) in
    `PATH`, since `verify:` is executed with `-c`.
- Git — `specd verify` records the current `git HEAD` into evidence, so run inside a
  git repository with at least one commit.

---

## Initializing a Project

Run once in your project root:

```bash
cd your-project
specd init
```

This writes the `.specd/` scaffold:

```
.specd/
├── roles/         # scout.md  craftsman.md  validator.md  auditor.md
└── steering/      # reasoning.md  workflow.md  product.md  tech.md  structure.md
```

It also writes `AGENTS.md` at the repo root (using a marker-based idempotent merge
so it never clobbers your edits).

> **Idempotent**: re-running `specd init` on a healthy project changes zero bytes.

---

## The Spec Lifecycle

### 1. Create a spec

```bash
specd new my-feature
```

Creates:

```
.specd/specs/my-feature/
├── requirements.md   # EARS stub
├── design.md         # Design stub (Modules / On-disk contracts / Invariants)
├── tasks.md          # Task table stub
├── memory.md         # Steering-memory stub
└── state.json        # Machine-truth state (revision 0)
```

### 2. Requirements phase

Edit `.specd/specs/my-feature/requirements.md`. Write EARS-shaped requirements:

```markdown
# Requirements — my-feature

- **R1** When <trigger>, the system shall <response>.
```

Validate, then approve:

```bash
specd check my-feature          # Run all validation gates
specd approve my-feature requirements   # Advance: requirements → design
```

### 3. Design phase

Edit `.specd/specs/my-feature/design.md`. The design stub provides the mandatory
sections:

```markdown
## Modules

## On-disk contracts

## Invariants
```

Approve:

```bash
specd approve my-feature design    # Advance: design → tasks
```

> The `design` gate arms additional validation: it checks that the design file is
> non-empty, non-stub, and has all required headers before allowing approval.

### 4. Tasks phase

Edit `.specd/specs/my-feature/tasks.md`. Author tasks as a Markdown table:

```markdown
# Tasks — my-feature

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | src/foo.go | - | go test ./... | R1 |
| T2 | validator | src/foo.go | T1 | go test ./... | R1 |
```

**Required columns:** `id`, `role`, `files`, `depends-on`, `verify`, `acceptance`.

**Valid roles:** `scout`, `craftsman`, `validator`, `auditor`.

Approve:

```bash
specd approve my-feature tasks     # Advance: tasks → executing
```

### 5. Executing phase

```bash
# See what's runnable (the frontier)
specd next my-feature

# Get the bounded context manifest for a specific task
specd context my-feature T1
specd context my-feature T1 --hud     # operator view: files, bytes, tokens

# Implement the task...

# Run and record verification
specd verify my-feature T1            # runs verify: command, records exit code + git HEAD

# Mark complete (requires a passing verify record)
specd task complete my-feature T1
```

`specd verify` appends an **evidence record** regardless of the outcome (pass or fail).
`specd task complete` checks for a record with `exit_code: 0` pinned to a real git
commit — a warning is emitted if HEAD is unresolved.

> **Read-only roles** (scout, validator, auditor) complete through the same evidence
> gate — there is no bypass flag. Give a read-only task a `verify` line it can pass
> (e.g. `printf ok`) so `specd verify` records a real passing evidence record, then
> `specd task complete` as usual.

### 6. Verifying / Complete

Once all tasks are complete, check and close the spec:

```bash
specd check my-feature
specd approve my-feature verifying    # Advance: executing → verifying
specd approve my-feature complete     # Close the spec
```

---

## Writing Spec Artifacts

### requirements.md — EARS format

Each requirement must follow one of the five EARS patterns (case-insensitive):

| Form | Pattern |
|---|---|
| Event-driven | `WHEN <trigger> THE SYSTEM SHALL <response>` |
| State-driven | `WHILE <state> THE SYSTEM SHALL <response>` |
| Optional feature | `WHERE <feature> THE SYSTEM SHALL <response>` |
| Unwanted behaviour | `IF <condition> THEN THE SYSTEM SHALL <response>` |
| Ubiquitous | `THE SYSTEM SHALL <response>` |

Complex/combined clauses (e.g. `When X, while Y, the system shall Z`) are accepted
because the leading keyword and `THE SYSTEM SHALL` anchor the match.

### design.md — mandatory sections

The design gate checks for these three H2 headers (non-empty, no `TODO` markers):

```markdown
## Modules

## On-disk contracts

## Invariants
```

### tasks.md — table format

The parser is bespoke (`core.ParseTasksMd`) — line-based, round-trip byte-stable.
Each row must provide all six columns. Missing columns cause a gate error.

**`depends-on`**: Use `-` for no dependencies, or a comma-separated list of task IDs:

```
| T2 | craftsman | src/bar.go | T1 | go test ./... | R1 |
| T3 | craftsman | src/baz.go | T1,T2 | go test ./... | R1 |
```

**Task markers** (written by `specd task complete`):

| Marker | Meaning |
|---|---|
| ✅ or `done` / `complete` | Task is complete |
| 🚧 or `running` | Task is in progress |
| ⛔ or `blocked` | Task is blocked |
| *(empty)* | Task is pending |

---

## Task Execution and Evidence

### Evidence records

`specd verify <slug> <task-id>` runs the task's `verify:` shell command and appends
an evidence record to `.specd/specs/<slug>/evidence.jsonl`:

```json
{"task_id":"T1","command":"go test ./...","exit_code":0,"git_head":"abc1234..."}
```

Records are **append-only** — they accumulate across retries. The *latest passing
record* (exit code 0, non-empty git HEAD that resolves to a real commit) is what
counts for task completion.

### Revert on fail

```bash
specd verify my-feature T1 --revert-on-fail
```

On a non-zero exit, restores the working tree to its pre-verify state using
`git diff --binary` + `git apply`.

### Sandboxed verify

```bash
specd verify my-feature T1 --sandbox
specd verify my-feature T1 --sandbox --sandbox-binary=/usr/bin/bwrap
```

Runs the verify command under `bwrap` isolation (fail-closed if the binary is absent
and `--sandbox` is specified).

### Frontier and dispatch

```bash
specd next my-feature              # list IDs of currently runnable tasks
specd next my-feature --json       # machine-readable frontier list
specd next my-feature --waves      # show all wave groups as JSON
specd next my-feature --dispatch   # emit context manifest for the first frontier task
```

A task is on the frontier when all its `depends-on` tasks are complete and it is
not yet complete itself.

---

## Reporting

```bash
specd status my-feature            # human-readable status + task table
specd status my-feature --json     # machine-readable (includes records)
specd report my-feature            # evidence-backed status report
specd report my-feature --pr       # PR-oriented summary
specd report my-feature --metrics  # metrics summary
specd report my-feature --json     # machine-readable report model
```

### Recording decisions and mid-stream requirement changes

```bash
specd decision my-feature --text "Chose X over Y because Z" --scope "architecture"
specd midreq my-feature --text "Added R5 for accessibility" --scope "requirements"
```

These append timestamped, git-HEAD-pinned records to `state.json`. Every record
carries a provenance triple: `timestamp`, `git_head`, `actor` (from `$SPECD_ACTOR`
or the OS user).

---

## Configuration

Configuration lives in **`project.yml` at your repository root**. It is optional — with
no file present, the embedded defaults apply. Environment overrides are the final layer.

### Config cascade

```
Embedded defaults → project.yml (repo root) → SPECD_* env vars
```

> The loader only reads a file with a `.yml` extension; `.yaml` and `.json` are ignored.

### Example `project.yml`

```yaml
version: "1"
agent: codex

context:
  max_tokens: 12000

gates:
  verify: error

orchestration:
  enabled: false
  model: ""
```

### Defaults

| Key | Default | Description |
|---|---|---|
| `version` | `"1"` | Config schema version |
| `agent` | `"codex"` | Selected agent harness |
| `context.max_tokens` | `12000` | Max tokens for context manifests |
| `gates.verify` | `"error"` | Verify gate severity |
| `orchestration.enabled` | `false` | Enable Brain orchestration |
| `orchestration.model` | `""` | Model for orchestration (empty = off) |

### Environment variables

| Variable | Description |
|---|---|
| `SPECD_ACTOR` | Override the actor name stamped on records (default: OS user). |

---

## Troubleshooting

### "missing approval: requirements and design gates must be approved"

`specd next` and `specd verify` require the spec to be in `tasks` or `complete` status,
or have both `approval:requirements` and `approval:design` records. Run:

```bash
specd status my-feature
specd check my-feature
specd approve my-feature requirements
specd approve my-feature design
```

### "approve refused: readiness gates failing"

`specd check` found errors. Fix them, then retry `specd approve`. Common causes:

- **EARS gate**: requirements don't match any EARS pattern.
- **Design gate**: design file is empty, unchanged from the stub, or missing a required section.
- **Task-ids gate**: a task has an empty or duplicate ID.
- **DAG gate**: circular dependency or a task depends on a non-existent ID.
- **Evidence gate**: a task is marked complete but has no passing verify record.
- **Sync gate**: `tasks.md` markers disagree with `state.json` task statuses.

### "state revision conflict"

Two concurrent operations tried to write `state.json`. Retry the failing command —
the CAS (`SaveStateCAS`) retry is safe.

### "task T1 not found in evidence" / task won't complete

```bash
specd verify my-feature T1    # run verify again
specd status my-feature --json | grep T1   # check evidence records
```

If git HEAD is unresolved, the evidence record won't count — ensure you have at
least one commit and are inside a git repo.

### "spec already exists"

```bash
specd new my-feature   # errors if the spec directory exists
```

Either use the existing spec or manually delete `.specd/specs/my-feature/`.

### Config not loading

The config loader accepts **YAML only** and reads `project.yml` at the repo root.
Check that your file:
- Is named `project.yml` at the repository root (not `.specd/config.yml`)
- Has the `.yml` extension (not `.yaml` or `.json`)
- Uses two-space indentation for nested keys
- Has no trailing colons without values at the section level
