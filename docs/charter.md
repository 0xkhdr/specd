# specd charter

specd is a deterministic harness component for agent work. Its product thesis is
`Agent = Model + Harness`: models produce candidate judgment, while the harness
owns state, gates, task flow, evidence, and reproducible command behavior.

## Principles

1. Determinism first: no model or network call controls harness decisions.
2. Evidence integrity: completion needs verifier output tied to the current git
   `HEAD`.
3. Atomic state: writes use durable replacement and compare-and-swap revisions.
4. Minimal surface: keep verbs that protect the core loop, cut accretion.
5. Local truth: on-disk specs, tasks, and state are the source of record.
6. Reentrant safety: per-spec locks protect concurrent harness actions.
7. Zero runtime dependencies: ship as one static Go binary using only stdlib.
8. Context discipline: commands expose focused state, not transcript sprawl.

## Verb map

| Verb | Harness component | Principle |
|---|---|---|
| `init` | workspace bootstrap | Local truth |
| `new` | spec creation | Local truth |
| `auth` | requirement authorization | Evidence integrity |
| `plan` | task DAG planning | Determinism first |
| `tasks` | task ledger | Local truth |
| `waves` | wave scheduler | Determinism first |
| `claim` | lock and ownership | Reentrant safety |
| `release` | lock and ownership | Reentrant safety |
| `verify` | gate runner | Evidence integrity |
| `complete` | state transition | Evidence integrity |
| `check` | invariant checker | Determinism first |
| `status` | state reader | Context discipline |
| `report` | observability report | Context discipline |
| `doctor` | environment diagnostics | Zero runtime dependencies |
| `help` | CLI usage | Minimal surface |
| `version` | binary identity | Minimal surface |
