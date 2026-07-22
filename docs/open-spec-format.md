# specd — On-Disk Spec Format

Every artifact specd reads and writes lives under `.specd/` in the managed project. The layout
is stable, greppable, and human-editable — the harness owns the writes, but nothing is opaque.
This page documents the on-disk surface so integrators can read it directly.

## Quality evidence declaration

Tasks may use optional `evidence` and `checks` columns. `evidence` contains comma-separated
`<class>/<check-id>` requirements; classes are `test`, `output_eval`, `trajectory_eval`, or
`review`. `checks` lists stable check IDs. Tables omitting both remain valid; existing
`verify` records mean `test` evidence only. Parser preserves author bytes—no companion file or
silent rewrite is required.

Quality context is reference-only: class/check IDs, verify command, artifact refs/digests,
subject revision, freshness labels, and dataset/rubric/output/trace digests. It never embeds
dataset cases, raw outputs, traces, prompts, or secrets.

Governed state transitions may also create `workflow-events.jsonl`. Each fsynced JSON line contains
the transition identity, entity and revision versions, actor and authority provenance, reason,
input digests, impacted identities, and the resulting state projection. Raw source content, prompts,
command output, and secrets are forbidden. A final incomplete line is ignored during recovery;
malformed complete lines, future schemas, duplicate identities, and revision divergence fail closed.

## Layout

```
.specd/
├── specs/
│   └── <slug>/
│       ├── requirements.md   # EARS requirements (author)
│       ├── design.md         # design (author)
│       ├── tasks.md          # the task DAG (author)
│       ├── state.json        # machine truth (harness-owned)
│       ├── revisions/        # content-addressed prior artifact revisions
│       │   └── <artifact>/<sha256>.md
│       └── .lock             # per-spec reentrant lock
├── roles/*.md                # scout / craftsman / validator / auditor
└── steering/*.md             # durable project constitution + memory
AGENTS.md                     # host integration guide (written by init)
```

A *managed project's* runtime state always lives in `.specd/specs/` — the format described
here never applies outside that tree.

## Authoring artifacts

| File | Phase | Gate(s) | Notes |
|---|---|---|---|
| `requirements.md` | perceive | `ears` | Author in EARS syntax. |
| `design.md` | analyze | `design` | Fails closed while still the scaffold stub. |
| `tasks.md` | plan | `task-ids`, `dependencies`, `dag`, `roles`, `files`, `verify` | The acyclic task DAG. |

`tasks.md` is parsed by a **byte-stable** parser (`tasksparser.go`): parse+write round-trips
without reformatting, so hand edits and tool edits coexist without diff noise. Each task
declares an id, a role, the files it may touch, its dependencies, and a **verify command**.
Read-only tasks still carry a trivially-passing verify line (e.g. `printf ok`).

## `revisions/` — preserved artifact revisions

`specd reopen <spec> artifact <requirements|design|tasks>` and `specd reopen <spec> spec`
preserve the artifact bytes **before** anything is mutated, at
`.specd/specs/<slug>/revisions/<artifact>/<sha256>.md`. The file name is the sha256 of its own
contents, so:

- writing the same bytes twice is idempotent — the existing snapshot is reused;
- a snapshot whose bytes do not hash to its own name fails closed rather than being overwritten;
- `<artifact>` is one of `requirements`, `design`, `tasks` and every path component is
  normalized inside the spec's revision directory, so no reopen can write outside it.

Snapshots are additive and never deleted: they are how a prior draft version — and the complete
prior lifecycle cycle after a spec reopen — stays reportable. If snapshot creation fails, the
reopen appends no workflow event and writes no state.

## `state.json` — the machine truth

`state.json` is harness-owned: agents never edit it directly (they drive the CLI; the harness
writes). It carries the per-spec status, phase, and the append-only record ledger. Top-level
shape (`internal/core/state.go`, `type State`):

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | int | On-disk schema (**2**; `1` still loads through the compatibility projection, anything newer fails closed). |
| `slug` | string | Spec slug. |
| `mode` | `default` \| `agent` | Execution mode. |
| `status` | string | Compatibility projection of `stage`/`condition` for schema-1 readers: `requirements → design → tasks → executing → verifying → complete` (+ `blocked`). |
| `cycle` | int | Delivery cycle; every migrated v1 spec is cycle `1`. |
| `stage` | string | Canonical lifecycle stage: `requirements → design → tasks → executing → verifying → complete`. |
| `condition` | string | Canonical condition: `active`, `waiting_approval`, `waiting_clarification`, `paused`, `blocked`, `cancelled`, `complete`. |
| `current_request` | string | Approval request identity a `waiting_approval` condition requires. |
| `phase` | string | Derived phase: `perceive → analyze → plan → execute → verify → reflect`. |
| `revision` | int64 | Monotonic counter; mutations **compare-and-swap** on it. |
| `records` | object | Append-only ledger of stamped records (approvals, decisions, verify records, …). |
| `task_status` | object | Per-task run status — the machine truth the `sync` gate reconciles against `tasks.md` markers. |
| `extra` | object | Forward-compatible escape hatch. |

`core.ValidateStageCondition` is the single owner of the legal `stage`/`condition` pairs — a
combination it rejects (complete plus paused, `waiting_approval` without a `current_request`)
can neither be saved nor loaded — and `core.ProjectStatus` derives `status` from that pair.
Migration maps a v1 spec to cycle 1, preserves `state.v1.json.bak` with the original file
permissions, writes the baseline `workflow-events.jsonl` entry, replays it, proves the
effective meaning is unchanged, and only then activates schema 2. A legacy `blocked` state
cannot reveal the stage it was blocked in, so migration refuses it with a repair diagnostic
instead of guessing.

Every ledger `record` carries a provenance triple — `timestamp` (RFC 3339 UTC), `git_head`,
and `actor` (`$SPECD_ACTOR`, else OS user, else `unknown`). The actor is host-reported and
stored verbatim; it is a label, never trusted as proof. Evidence integrity rests on the
`git_head` + verify exit code, not the actor string.

## Validating the schema

`state.json` is loaded with `DisallowUnknownFields` and validated on every read. To check it
explicitly:

```bash
specd check payments --schema        # run all gates plus schema validation
specd check payments --schema-only   # validate only the state.json schema
```

A malformed or unknown-field `state.json` surfaces as a `schema` gate finding (exit 1) rather
than a silent load. Because the whole surface is plain Markdown + one validated JSON file,
integrators can read spec state without linking against specd.

## Decision contract & task trace/risk metadata (spec 01)

Both extensions are **additive and optional** — existing `design.md` and
`tasks.md` files parse unchanged, and the stricter checks arm only under the production
profile. That profile is a single project-config switch, `profile: production` in
`project.yml` (spec 01 R7): it arms the design-contract and task-trace completeness checks
alongside the criterion, review, and integration/negative-path evidence gates. `profile:
default` (the default) keeps every one of them opt-in.

**`design.md` decision contract.** Declare the trace with labelled bullets:

```markdown
- references: R1.1, R2
- boundaries: <what this design owns / does not own>
- interfaces: <the contracts it exposes>
- invariants: <what stays true>
- failure: <failure modes>
- integration: <integration modes>
- alternatives: <what was weighed>
- disposition: <the chosen option>
- owner: <human accountable>
```

A `references:` entry that names an unknown requirement is **always** refused. The full
contract is required only when the production design profile is on.

**`tasks.md` optional trace/risk columns.** Add any of `refs`, `kind`, `risk`, `context`,
`evidence`, `checks` to the table header — columns are matched by name, so a minimal
six-column table keeps working:

```markdown
| id | role | files | depends-on | verify | acceptance | refs | kind | risk | context | evidence | checks |
```

A declared `refs` requirement that does not resolve, or an unknown `risk` tier
(`low`/`medium`/`high`/`critical`), is always refused. Declaring the full set on every task
is required only under the production planning profile.

**One typed task contract.** Every typed cell is parsed exactly once, by
`core.ParseTaskContract`. Planning gates, routing, evidence policy, review, and context read
that result; none of them re-splits a raw cell. Closed vocabularies:

| column | accepted values |
|---|---|
| `kind` | `chore`, `deferred`, `docs`, `feature`, `fix`, `refactor`, `spike`, `test` |
| `risk` | `low`, `medium`, `high`, `critical` |
| `capabilities` | `context`, `eval`, `review`, `sandbox` |
| `evidence` | `class/check-id` where class is `test`, `output_eval`, `trajectory_eval`, or `review` |

An unrecognized value is refused as `TASK_FIELD_UNKNOWN` naming the task id, the column, the
value, and the accepted set. The capability vocabulary is the same identity routing classes
declare (`routing.class_capabilities`), so a legal task row can always be routed; a
`kind: deferred` row records a deliberate deferral and carries no evidence or edge-check
obligation.

**Delimiters.** The canonical list separator in every list-shaped cell is `,`. The legacy `;`
is still normalized, and the contract carries a stable
`TASK_FIELD_LEGACY_DELIMITER` warning naming the task and column. Rewrite the cell with commas.

## Machine context manifest and receipts

The machine manifest (`kind: context_manifest`, `schema_version: "1"`) is the typed machine
contract; the human-readable renderer remains the default output. Unknown versions, item kinds,
routes, trust labels, and missing required lanes fail closed rather than being reinterpreted.
The machine manifest requires canonical item ordering, source digests, selected-task identity,
driver route/capability metadata, and explicit omission records. Required context overflow is
an error; only optional items may be shed under budget.

A receipt contains digests, token totals, and provenance only—never source bytes, prompts, or
secrets. Compare config, palette, required-context, and selected-skill digests before trusting a
receipt. Any mismatch marks it stale while preserving its historical JSON for audit. Skills and
memory are untrusted advisory data and cannot add tools, widen declared files, approve work, or
change route authority. Hosts must treat route and capability metadata as an exact identity
binding and stop on mismatch.

---

**See also:** [validation-gates.md](validation-gates.md) · [user-guide.md](user-guide.md) ·
[contributor-guide.md](contributor-guide.md)
