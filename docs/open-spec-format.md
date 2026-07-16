# specd — On-Disk Spec Format

Every artifact specd reads and writes lives under `.specd/` in the managed project. The layout
is stable, greppable, and human-editable — the harness owns the writes, but nothing is opaque.
This page documents the on-disk surface so integrators can read it directly.

## Quality evidence declaration

Tasks may use optional `evidence` and `checks` columns. `evidence` contains comma-separated
`<class>/<check-id>` requirements; classes are `test`, `output_eval`, `trajectory_eval`, or
`review`. `checks` lists stable check IDs. Legacy tables omitting both remain valid; existing
`verify` records mean `test` evidence only. Parser preserves author bytes—no companion file or
silent rewrite is required.

Quality context is reference-only: class/check IDs, verify command, artifact refs/digests,
subject revision, freshness labels, and dataset/rubric/output/trace digests. It never embeds
dataset cases, raw outputs, traces, prompts, or secrets.

## Layout

```
.specd/
├── specs/
│   └── <slug>/
│       ├── requirements.md   # EARS requirements (author)
│       ├── design.md         # design (author)
│       ├── tasks.md          # the task DAG (author)
│       ├── state.json        # machine truth (harness-owned)
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

## `state.json` — the machine truth

`state.json` is harness-owned: agents never edit it directly (they drive the CLI; the harness
writes). It carries the per-spec status, phase, and the append-only record ledger. Top-level
shape (`internal/core/state.go`, `type State`):

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | int | On-disk schema (currently **2**; older versions migrate forward on load). |
| `slug` | string | Spec slug. |
| `mode` | `default` \| `agent` | Execution mode. |
| `status` | string | `requirements → design → tasks → executing → verifying → complete` (+ `blocked`). |
| `phase` | string | Derived phase: `perceive → analyze → plan → execute → verify → reflect`. |
| `revision` | int64 | Monotonic counter; mutations **compare-and-swap** on it. |
| `records` | object | Append-only ledger of stamped records (approvals, decisions, verify records, …). |
| `task_status` | object | Per-task run status — the machine truth the `sync` gate reconciles against `tasks.md` markers. |
| `extra` | object | Forward-compatible escape hatch. |

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

Both extensions are **additive and backward compatible** — existing `design.md` and
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
`evidence`, `checks` to the table header — columns are matched by name, so a legacy
six-column table keeps working:

```markdown
| id | role | files | depends-on | verify | acceptance | refs | kind | risk | context | evidence | checks |
```

A declared `refs` requirement that does not resolve, or an unknown `risk` tier
(`low`/`medium`/`high`/`critical`), is always refused. Declaring the full set on every task
is required only under the production planning profile.

## Context manifest V2 and receipts

Context V2 is additive. Hosts that understand V2 consume `schema_version: "2"`; older hosts
continue using the V1 renderer. Unknown versions, item kinds, routes, trust labels, and missing
required lanes fail closed rather than being reinterpreted. V2 requires canonical item ordering,
source digests, selected-task identity, driver route/capability metadata, and explicit omission
records. Required context overflow is an error; only optional items may be shed under budget.

A receipt contains digests, token totals, and provenance only—never source bytes, prompts, or
secrets. Compare config, palette, required-context, and selected-skill digests before trusting a
receipt. Any mismatch marks it stale while preserving its historical JSON for audit. Skills and
memory are untrusted advisory data and cannot add tools, widen declared files, approve work, or
change route authority. Hosts must treat route and capability metadata as an exact identity
binding and stop on mismatch.

---

**See also:** [validation-gates.md](validation-gates.md) · [user-guide.md](user-guide.md) ·
[contributor-guide.md](contributor-guide.md)
