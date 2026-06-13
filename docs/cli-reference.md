# 5. CLI Reference

Complete reference for every `specd` command. All examples assume you are inside a repo that has a
`.specd/` directory (created by `specd init`).

## Synopsis

```
specd <command> [args] [flags]
specd --help | -h | help
specd --version | -v | version
```

## Global behavior

- **Argument parsing** is positional + `--key value` / `--flag`. Three flags are boolean (no value):
  `--force`, `--json`, `--all`. Any other `--key` consumes the next token as its value (unless that
  token starts with `--`, in which case the key is treated as a bare boolean).
- **Root discovery:** every repo-touching command walks up from cwd to find `.specd/`. Not found →
  exit `3`.
- **`--json`** is available on the read/inspect commands (`status`, `context`, `check`, `next`,
  `approve`, `waves`) for machine consumption.

## Exit codes

These are a hard contract — CI and agents branch on them. Defined in `src/core/exit.ts`.

| Code | Name | Meaning |
|------|------|---------|
| `0` | OK | success / valid |
| `1` | GATE | gate or validation failure |
| `2` | USAGE | malformed invocation |
| `3` | NOTFOUND | spec or `.specd/` root not found |

## Command summary

| Command | Purpose |
|---------|---------|
| [`init`](#init) | Scaffold `.specd/` + `AGENTS.md`. Idempotent. |
| [`new`](#new) | Create a spec folder with six artifacts + `state.json`. |
| [`status`](#status) | The durable ledger / board: counts, wave graph, blockers, next. |
| [`context`](#context) | Minimal phase-scoped briefing: what to load now + the next action. |
| [`check`](#check) | Run all seven validation gates. |
| [`next`](#next) | The next runnable task (`--all` = the whole frontier). |
| [`task`](#task) | The evidence gate. Dual-writes `tasks.md` + `state.json`. |
| [`approve`](#approve) | Advance the planning phase or clear an `awaiting-approval` gate. |
| [`decision`](#decision) | Append a numbered ADR. |
| [`midreq`](#midreq) | Log mid-flight feedback; gate on high/critical. |
| [`memory`](#memory) | Add source-attributed learnings; promote to steering. |
| [`report`](#report) | Deterministic snapshot (markdown or single-file HTML). |
| [`waves`](#waves) | The wave DAG, critical path, and blockers. |

---

## `init`

```
specd init [--force]
```

Scaffolds the harness into the current directory: `.specd/steering/*`, `.specd/roles/*`,
`.specd/config.json`, and the repo-root `AGENTS.md`.

- **Idempotent.** Existing files are skipped (reported as `· skipped`); only missing files are
  written (`+ wrote`).
- `--force` overwrites existing files.

```
$ specd init
specd init: wrote 12 file(s):
  + .specd/steering/reasoning.md
  + .specd/steering/workflow.md
  ...
  + AGENTS.md
```

**Exit:** always `0`.

---

## `new`

```
specd new <slug> [--title "..."]
```

Creates `.specd/specs/<slug>/` with the six artifact stubs and a fresh `state.json` (`status:
requirements`, `phase: analyze`).

- `<slug>` must match `^[a-z0-9][a-z0-9-]*$`.
- `--title` defaults to a title-cased version of the slug.

```
$ specd new my-feature --title "My Feature"
specd new: created spec 'my-feature' (My Feature)
  .specd/specs/my-feature/ — six artifacts + state.json (status: requirements)
Next: write requirements.md (EARS), then `specd check my-feature`.
```

**Exit:** `0` ok · `2` invalid/missing slug · `1` spec already exists.

---

## `status`

```
specd status [<slug>] [--json]
```

Orientation — "where am I". With no slug, lists every spec as a one-line board. With a slug, prints
the full ledger for that spec: counts, the wave graph with status glyphs, blockers, the next task,
and any active gate.

```
$ specd status
my-feature  [executing]  2/5 done · next: T3 — Wire config

$ specd status my-feature
# My Feature (my-feature)
status: executing · phase: execute · gate: none · turn: 1
tasks: 2 complete · 1 running · 2 pending · 0 blocked · 5 total

Wave 1
  ✓ T1  Parse config.json
  ✓ T2  Validate schema
Wave 2
  ◐ T3  Wire config
  ○ T4  Add YAML branch
Wave 3
  ○ T5  Docs

Critical path: T1 → T3 → T5

Next: T3 — Wire config
```

Status glyphs: `✓` complete · `◐` running · `○` pending · `⚠` blocked.

`--json` emits the full `state` object plus `counts` and the `next` classification.

**Exit:** `0` (or `3` if the named slug does not exist).

---

## `context`

```
specd context <slug> [--json]
```

The **context-engineering primitive**. Emits the *minimal* phase-scoped briefing the agent needs
right now — orientation, exactly which files to load for the current phase, the focus, and the single
next action. The opposite of dumping every doc into the window every turn.

`reasoning.md` and `workflow.md` are always loaded; everything else is phase-scoped:

| Phase | LOAD NOW (besides the two always-on steering files) |
|-------|------------------------------------------------------|
| ANALYZE (`requirements`) | `requirements.md`, `steering/product.md` |
| PLAN design (`design`) | `requirements.md`, `design.md`, `steering/tech.md`, `steering/structure.md` |
| PLAN tasks (`tasks`) | `design.md`, `tasks.md` |
| EXECUTE (`executing`) | `tasks.md`, `memory.md` |
| EXECUTE blocked (`blocked`) | `tasks.md` |
| VERIFY (`verifying`) | `tasks.md`, `requirements.md` |
| REFLECT (`complete`) | `memory.md`, `decisions.md` |

```
$ specd context my-feature
=== CONTEXT: my-feature ===
My Feature · status executing · phase execute · turn 1
tasks: 2/5 done · next: T3 — Wire config

PHASE EXECUTE — Build one task at a time, evidence-gated.

LOAD NOW (minimal — don't dump the rest):
  - .specd/steering/reasoning.md
  - .specd/steering/workflow.md
  - .specd/specs/my-feature/tasks.md
  - .specd/specs/my-feature/memory.md

FOCUS: Run the next runnable task only: T3 — Wire config
NEXT:  specd next my-feature
```

If the spec is gated (`awaiting-approval`), `context` says so and the next action becomes
`specd approve <slug>` — work is frozen.

**Exit:** `0` (or `3` if not found).

---

## `check`

```
specd check <slug> [--json]
```

Runs **all seven validation gates** and prints each violation as
`fail  <location>: <message> (<gate>)`. Warnings (traceability) print as `warn  …`. See
[Validation Gates](validation-gates.md) for what each gate enforces.

```
$ specd check my-feature
✓ check passed — all gates green for 'my-feature'

$ specd check broken
fail  tasks.md:14: T3: invalid role 'maker' (task-schema)
fail  tasks.md: dependency cycle: T3 → T4 → T3 (dag)

✗ 2 violation(s) across gates.
```

`--json` emits `{ ok, violations[], warnings[] }`.

**Exit:** `0` all gates green (warnings allowed) · `1` one or more violations · `3` spec not found.

---

## `next`

```
specd next <slug> [--all] [--json]
```

Hands out work.

**Default** — the *single* next runnable task (lowest wave, then lowest id ordinal) as a paste-ready
prompt block:

```
$ specd next my-feature
=== NEXT TASK: T3 ===
title:        Wire config
role:         builder
why:          requirement 1 requires the loaded config to take effect
files:        src/cli.ts
contract:     load config at boot; pass into command dispatch
acceptance:   e2e startup test reads a custom value from config.json
verify:       npm test -- startup
depends:      T1
requirements: 1
==============================
When done: specd task my-feature T3 --status complete --evidence "<proof>"
```

When nothing is runnable, `next` classifies why:

- `✓ all tasks complete` — and, if `status: verifying`, prompts you to `specd approve`.
- `⚠ all remaining tasks blocked: …` — lists the blockers.
- `… waiting — frontier gated by incomplete deps: …`.

**`--all`** — the whole **runnable frontier** for parallel dispatch:

```
$ specd next my-feature --all
=== RUNNABLE FRONTIER (2) — dispatch in parallel ===
  T3  [wave 2]  Wire config  (builder)
  T4  [wave 2]  Add YAML branch  (builder)
==============================
Each: specd next my-feature (focused) or complete with specd task my-feature <id> --status complete --evidence "<proof>"
```

**Gate behavior:** if the spec is `awaiting-approval`, `next` refuses (`⛔ gate awaiting-approval`)
and exits `1` unless `--force` is passed.

`--json` emits the structured result (`{ kind: "task", task: {...} }`, `frontier`, `all-complete`,
`all-blocked`, `waiting`, or `gated`).

**Exit:** `0` normally · `1` when gated (non-force) · `3` not found.

---

## `task`

```
specd task <slug> <id> --status <complete|blocked|running|pending> [--evidence "..."] [--reason "..."] [--force]
```

**The evidence gate and the integrity core.** The only way to change a task's status. It holds the
per-spec lock across load → mutate → dual-write, so a parallel builder on another task of the same
spec cannot clobber the update.

| `--status` | Required flag | Effect |
|------------|---------------|--------|
| `complete` | `--evidence "<non-empty>"` | checks the box, stores evidence, stamps `finishedAt`; **rejected if any dependency is not complete** |
| `blocked` | `--reason "<text>"` | records a blocker, surfaces it in `state.blockers` |
| `running` | — | marks in progress, stamps `startedAt` |
| `pending` | — | resets to not-started |

After any flip, the spec status is re-derived: once any task leaves `pending` the spec is
`executing`; if every remaining task is blocked it becomes `blocked`; when all tasks complete it
becomes `verifying`.

```
$ specd task my-feature T1 --status complete --evidence "commit a1b2c3d; npm test PASS (12/12)"
task T1 → complete
  evidence: commit a1b2c3d; npm test PASS (12/12)
```

**Gate behavior:** refuses while the spec is `awaiting-approval` (override with `--force`).

**Exit:** `0` ok · `1` evidence/dependency/gate violation · `2` bad/missing `--status` · `3` task or
spec not found.

---

## `approve`

```
specd approve <slug> [--json]
```

The human approval primitive — the **only** sanctioned way to advance a planning phase or clear a
gate. It does one of three jobs depending on state:

1. **Clear a gate** — if `gate == awaiting-approval` (raised by a high/critical `midreq`), clear it
   and resume at the paused status.
2. **Accept VERIFY** — if `status == verifying`, advance `verifying → complete` (phase REFLECT).
3. **Advance the planning ratchet** — `requirements → design → tasks → executing`, but only if the
   gate for the *current* artifact is green. Otherwise it prints the blocking violations and exits
   `1`.

```
$ specd approve my-feature
approve: 'requirements' approved → status 'design' (phase plan).

$ specd approve my-feature     # when verifying
approve: verification accepted → status 'complete' (phase reflect).
```

**Exit:** `0` advanced/cleared · `1` gate not green / nothing to approve · `3` not found.

---

## `decision`

```
specd decision <slug> "<text>" [--supersedes <ADR-id>]
```

Appends an auto-numbered ADR (`ADR-001`, `ADR-002`, …) to `decisions.md`. The numbering read +
append is locked so two concurrent decisions cannot mint the same id.

```
$ specd decision my-feature "Use JSON, not YAML, for config" --supersedes ADR-001
decision: appended ADR-002 to decisions.md
```

**Exit:** `0` ok · `2` missing args · `3` spec not found.

---

## `midreq`

```
specd midreq <slug> "<verbatim input>" --impact <low|medium|high|critical> [--interpretation "..."] [--changes "..."]
```

Logs mid-flight user feedback to `mid-requirements.md` and bumps the turn counter. **`high` and
`critical` raise the `awaiting-approval` gate**, which freezes `next`/`task` until `specd approve`.
`--interpretation` and `--changes` default to `TODO` if omitted.

```
$ specd midreq my-feature "Also support YAML config" --impact high \
    --interpretation "Add a YAML branch to the loader" --changes "New task T4"
midreq: logged Turn 2 (impact: high)
⛔ gate set to awaiting-approval — stop, present the revised plan, wait for approval.
```

**Exit:** `0` ok · `2` bad/missing `--impact` · `3` spec not found.

---

## `memory`

```
specd memory <slug> add --key <k> --pattern "..." --body "..." --source "..." --criticality <minor|important|critical> [--related a,b]
specd memory <slug> promote --key <k> [--force]
```

**`add`** appends a source-attributed learning to the spec's `memory.md`. `--related` is a
comma-separated list rendered as `[[key]]` links.

**`promote`** lifts an entry into global `steering/memory.md`, but only once the pattern has appeared
in at least `promotionThreshold` specs (default 3). `--force` overrides the threshold.

```
$ specd memory my-feature add --key config-fallback \
    --pattern "missing config → documented defaults" \
    --body "Loader returns defaults rather than throwing" \
    --source "T1 commit a1b2c3d" --criticality important
memory: added 'config-fallback' to my-feature/memory.md

$ specd memory my-feature promote --key config-fallback
memory: promoted 'config-fallback' from my-feature to steering/memory.md (seen in 3 spec(s), threshold 3)
```

**Exit:** `0` ok · `1` key not found / below threshold (no `--force`) · `2` bad args · `3` spec not
found.

---

## `report`

```
specd report <slug> [--format md|html] [--out <path>]
```

Renders a **deterministic** snapshot of the entire spec state, assembled from `state.json` and the
markdown artifacts — never from an agent's memory. Default format comes from `config.json`
(`report.format`, default `md`). HTML is a single dependency-free file. Without `--out`, output goes
to stdout.

```
$ specd report my-feature
# (markdown snapshot to stdout)

$ specd report my-feature --format html --out report.html
# (single self-contained HTML file)
```

**Exit:** `0` ok · `3` spec not found.

---

## `waves`

```
specd waves <slug> [--json]
```

Renders the wave DAG (tasks grouped by wave with status glyphs), the critical path, and any blockers
gating downstream waves.

```
$ specd waves my-feature
Wave 1
  ✓ T1  Parse config.json
Wave 2
  ◐ T3  Wire config
  ⚠ T4  Add YAML branch  (blocked: needs yaml lib decision)

Critical path: T1 → T3 → T5

Blockers gating downstream waves:
  ⚠ T4: needs yaml lib decision
```

`--json` emits `{ waves[], criticalPath[], blockers[] }`.

**Exit:** `0` ok · `3` spec not found.
