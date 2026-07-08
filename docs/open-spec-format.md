# specd — On-Disk Spec Format

Every artifact specd reads and writes lives under `.specd/` in the managed project. The layout
is stable, greppable, and human-editable — the harness owns the writes, but nothing is opaque.
This page documents the on-disk surface so integrators can read it directly.

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

**The `.specd/specs/` vs. top-level `specs/` split:** a *managed project's* runtime state lives
in `.specd/specs/`. This repository's own in-flight planning artifacts live in a top-level
`specs/` — different tree, different purpose. (`regress-lint.sh` smell "A" catches verify lines
that target the wrong one.)

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

---

**See also:** [validation-gates.md](validation-gates.md) · [user-guide.md](user-guide.md) ·
[contributor-guide.md](contributor-guide.md)
