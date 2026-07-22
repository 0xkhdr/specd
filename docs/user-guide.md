# specd — User Guide

> **Status:** Normative documentation for current `specd` behavior.

A walkthrough of running a spec from empty workspace to submitted PR. For the *why*, read
[concepts.md](concepts.md); for every flag, [command-reference.md](command-reference.md).

## Install

Install latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh
```

Update:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh -s -- --update
```

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | sh
```

Useful installer flags:

```text
--version <tag>
--install-dir <dir>
--update
--force
--dry-run
```

Environment variables:

```text
SPECD_VERSION
SPECD_INSTALL_DIR
```

Installer downloads the release archive for Linux/macOS amd64/arm64, verifies
`checksums.txt`, and installs `specd` into `/usr/local/bin` unless overridden.
Unsupported platforms fail before download.

`specd` is Go (1.26+), stdlib only, zero runtime dependencies — one static binary.

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
specd approve payments
```

A successful check names the exact lifecycle transition, state revision, plan and config
digests, armed gate count, and artifact digests that were inspected. `specd check payments
--json` returns the versioned readiness envelope (plan plus findings). During the compatibility
window, `specd check payments --json=legacy` retains the previous bare findings array. Approval
rebuilds and consumes that same plan under the spec lock before its compare-and-swap update.

### 2. Design (analyze)

Fill `design.md` past its scaffold stub. The `design` gate compares against the stub and
fails closed while it is still boilerplate.

```bash
specd approve payments
```

### 3. Tasks (plan)

Author `tasks.md` — the acyclic task DAG. Each task declares an id, a role
(scout/craftsman/validator/auditor), files it may touch, dependencies, and a **verify
command**. The `task-ids`, `dependencies`, `dag`, `roles`, `files`, and `verify` gates all
check this file. Read-only tasks still carry a trivially-passing verify line (e.g. `printf ok`).

```bash
specd check payments          # all planning gates must pass
specd approve payments
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
specd complete-task payments T3
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

## Undo the last workflow event

An operator can compensate the **latest** workflow event of a spec — and only that one:

```bash
specd status payments --json          # read the current revision
specd undo payments --reason 'approved the wrong stage' --expect-revision 7
```

Undo never deletes or rewrites history. It appends a **compensation event** that references the
original, re-projects the prior effective state at a higher revision, and records the reason,
actor and authority digest, impact digest, affected identities, and every consumption guard it
checked. The original event stays in the ledger, visible and unchanged.

It refuses — mutating nothing — when the target is not the latest event, when the transition is
irreversible (submission, release, deployment, archive, schema baseline, or a previous
compensation), or when anything consumed it: passing evidence, a delivery ledger, or a
delegation. The refusal names the consuming record and, for externally consumed work, points at
a linked successor instead of an in-place repair.

`--expect-revision` is the revision the undo was previewed against. If the revision moved (a
concurrent repair, an approval), undo refuses with the fresh revision to re-run against, so at
most one racing repair ever wins. There is no event-id flag: older history is not undoable.

## Reopen a task for repair

A defect found after a task completed is repaired in a **new attempt**, not by editing history:

```bash
specd status payments --json          # read the current revision
specd reopen payments task T7 --reason 'rounding defect found in review' --expect-revision 12
```

Reopen is operator-only and works on a completed, failed, or cancelled task. It appends an
attempt event carrying the task id, the new attempt number, its plan and scope revision, a fresh
current git baseline, a fresh authority digest, pending activity with a derived readiness, and
links to both the prior attempt and the completed descendants the repair makes stale. The
tasks.md marker is not rewritten — the pending activity is projected, so the file stays
byte-stable.

Evidence is bound to the attempt it was recorded under. After a reopen, the prior attempt's
verify record no longer completes the task **even when its command, files, and git HEAD are
identical** — the record stays in the ledger as history, and `complete-task` refuses until you
run `specd verify` again against the new attempt.

Two things refuse the reopen outright, mutating nothing:

- **A live lease or mission owns the task.** The refusal names the holder and the exact
  `--revoke-lease <id>` that authorizes surrendering it inside the same transaction. A lease you
  hold yourself is released automatically.
- **The repair spans the task's declared files.** Approve the bounded amendment in the same
  transaction with `--scope internal/pricing.go,internal/tax.go`. The amendment is durable: the
  new attempt's write scope is the task's declared files plus what you approved here.

As with undo, `--expect-revision` is the revision the reopen was previewed against; a moved
revision or a moved impact plan refuses and asks for a fresh preview.

## Open questions

An unresolved question is recorded, not guessed. An agent may open one; only a human resolves
it:

```bash
specd clarification open payments --question 'which currency rounds up?' --entity task:T3 --blocking
specd clarification answer payments C1 --answer 'round half up'
specd clarification withdraw payments C2 --reason 'duplicate'
specd clarification expire payments C3 --reason 'no answer in time'
```

`--entity` takes `<spec|task|artifact>:<id>` and defaults to the spec. Only a **blocking**,
task-scoped question changes readiness: that task reports `waiting_clarification` in
`specd status --json`, and `specd context` refuses it until the question is resolved. Every other
task is untouched.

Records are immutable. A resolution is appended beside the question, never written over it, and a
changed question takes a new id. If the task's contract is revised after it was answered, the
answer is kept as history and the task reports a `clarification_stale` wait naming the review it
now needs.

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
