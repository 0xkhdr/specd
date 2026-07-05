# specd — User Guide

A walkthrough of running a spec from empty workspace to submitted PR. For the *why*, read
[concepts.md](concepts.md); for every flag, [command-reference.md](command-reference.md).

## Install

`specd` is Go (1.22+), stdlib only, zero runtime dependencies — one static binary.

```bash
go build -o specd .        # single static binary
# or run without building:
go run . help
```

Put the resulting `specd` on your `PATH`.

## Initialize a project

From your target repository's root:

```bash
specd init
```

This scaffolds `.specd/` (role prompts + steering files) and writes `AGENTS.md` into the
project root — the integration guide your agent loads. `specd init` is idempotent; re-run it
to re-sync managed assets:

- `specd init --repair` — restore managed regions that drifted from the templates.
- `specd init --refresh` — update managed regions to the current binary's template version.
- `specd init --dry-run` — print what would change, write nothing.

## Create a spec

```bash
specd new payments
```

Creates `.specd/specs/payments/` with stub `requirements.md`, `design.md`, `tasks.md`, and a
fresh `state.json` in the `requirements` (perceive) phase.

## The phase lifecycle

Each phase has one authoring artifact and one human approval gate. You cannot skip ahead: the
[gates](validation-gates.md) fail closed until the artifact is real, and status only moves
forward.

```
perceive → analyze → plan → execute → verify → reflect
```

### 1. Requirements (perceive)

Author `requirements.md` in **EARS** syntax (Easy Approach to Requirements Syntax). The `ears`
gate validates it. Typical EARS shapes:

```
The system SHALL <response>.
WHEN <trigger>, the system SHALL <response>.
WHILE <state>, the system SHALL <response>.
IF <condition>, THEN the system SHALL <response>.
```

Check and approve:

```bash
specd check payments
specd approve payments requirements
```

### 2. Design (analyze)

Fill `design.md` past its scaffold stub. The `design` gate compares against the stub and
fails closed while it is still boilerplate.

```bash
specd approve payments design
```

### 3. Tasks (plan)

Author `tasks.md` — the acyclic task DAG. Each task declares an id, a role
(scout/craftsman/validator/auditor), files it may touch, dependencies, and a **verify
command**. The `task-ids`, `dependencies`, `dag`, `roles`, `files`, and `verify` gates all
check this file. Read-only tasks still carry a trivially-passing verify line (e.g. `printf ok`).

```bash
specd check payments          # all planning gates must pass
specd approve payments tasks
```

The spec is now `executing`.

## Execute: the verify → complete loop

This is the core loop. The harness will not let a task complete without evidence.

```bash
# 1. What is runnable right now (the frontier / current wave)?
specd next payments

# 2. Optionally build the bounded context for a task:
specd context payments T3 --hud

# 3. Do the work (edit code), then run the task's verify command and record it:
specd verify payments T3
#    → runs the verify line, captures exit code + git HEAD as an evidence record.

# 4. Complete the task — only succeeds if a passing verify record exists:
specd task complete payments T3
```

If `specd verify` exits non-zero, the task does **not** complete. There is no bypass flag.
Repeated verify failures trip the **escalation ratchet** (default 3 consecutive fails) and
block the task until a human clears it:

```bash
specd task T3 --override --reason 'flaky infra, verified manually'
```

`--override` resets the ratchet; it does **not** complete the task — you still need a passing
verify. When a wave's tasks are all complete, `specd next` reveals the next wave.

## Mid-stream changes

Requirements shift. Capture them without breaking the audit trail:

```bash
specd midreq payments --text 'add refund path' --scope requirements
specd decision payments --text 'defer webhooks to v2' --scope design
```

Both are stamped into `state.json` and replay in `specd report payments --history`.

## Inspect progress

```bash
specd status payments            # current phase + task state
specd status payments --json     # machine-readable
specd status --program           # cross-spec program view (all specs, links, frontier)
specd report payments            # deterministic status report
specd report payments --metrics  # metrics summary
specd report payments --history  # full audit trail replay
```

## Finish: review and submit

If your project arms the review gate (`config.review.required = true`), scaffold and fill the
auditor's report first:

```bash
specd review payments
# fill review_report.md with an approve verdict at the current HEAD
```

Then submit — `submit` runs **every** gate and streams the PR summary to your configured
submit command:

```bash
specd submit payments
specd submit payments --resubmit   # re-submit at the same HEAD
```

## When you get stuck

See [troubleshooting.md](troubleshooting.md): blocked tasks, the escalation ratchet, lock
contention, CAS conflicts, and verify/sandbox failures.
