# Validation Gates

`specd check <slug>` runs the validation gate registry against a spec. A gate
failure exits `1` and blocks the relevant `specd approve` transition.

**Twelve core gates always run.** One additional gate (`security`) is opt-in
and a no-op by default.

---

## Core Gates

All twelve core gates are registered in `CoreRegistry()` (`internal/core/gates/core.go`).
They run as pure functions of the `CheckCtx` — no disk access inside a gate;
the caller reads files before building the context.

### Gate: `task-ids`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | All task IDs are non-empty and unique within the spec. |
| **Fails on** | Empty task ID; duplicate task ID. |

```
error task-ids: task id is required
error task-ids: duplicate task id T1
```

---

### Gate: `dependencies`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | Every entry in `depends-on` references a task ID that exists in the same spec. |
| **Fails on** | `T2 depends on missing task T99`. |

---

### Gate: `dag`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` + `internal/core/dag.go` |
| **Checks** | The dependency graph is acyclic (`core.NewTaskDAG`). |
| **Fails on** | Any cycle detected in the dependency graph. |

---

### Gate: `roles`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | Every task has a non-empty `role` field. |
| **Fails on** | A task with a blank `role`. |
| **Valid roles** | `scout`, `craftsman`, `validator`, `auditor` |

---

### Gate: `files`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | Every task has a non-empty `files` field. |
| **Fails on** | A task with a blank `files` field. |

---

### Gate: `verify`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | Every task has a non-empty `verify` command. |
| **Fails on** | A task with a blank `verify` field. |

> **Note:** Every task — including read-only roles — needs a non-empty `verify`
> command. Give a read-only task a trivially passing line (e.g. `printf ok`) so it can
> record real evidence; there is no bypass flag.

---

### Gate: `evidence`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | No task marked complete (`✅` / `done` / `complete` marker) lacks a passing verify record. |
| **Fails on** | `T1 is complete without passing evidence`. |
| **Passing evidence** | An evidence record with `exit_code: 0` and a non-empty, resolved `git_head`. |

A task's evidence is looked up in `evidence.jsonl`. The *latest* passing record wins
(`core.HasPassingEvidence`).

---

### Gate: `context-budget`

| | |
|---|---|
| **Source** | `internal/core/gates/contextbudget.go` |
| **Checks** | Estimated token cost of context manifests stays within `config.context.max_tokens`. |
| **Fails on** | Projected token count exceeds the configured budget. |
| **Default budget** | 12,000 tokens |

---

### Gate: `ears`

| | |
|---|---|
| **Source** | `internal/core/gates/ears.go` |
| **Checks** | Requirements follow a recognized EARS pattern (case-insensitive). |
| **Fails on** | A requirement that does not match any EARS form. |

**Recognized EARS forms:**

| Form | Pattern |
|---|---|
| Event-driven | `WHEN <trigger> THE SYSTEM SHALL <response>` |
| State-driven | `WHILE <state> THE SYSTEM SHALL <response>` |
| Optional feature | `WHERE <feature> THE SYSTEM SHALL <response>` |
| Unwanted behaviour | `IF <condition> THEN THE SYSTEM SHALL <response>` |
| Ubiquitous | `THE SYSTEM SHALL <response>` |

Notes:
- Matching is case-insensitive — `when …` / `When …` / `WHEN …` are all accepted.
- Ubiquitous is matched **last** so a conditional form is never mis-tagged.
- Complex/combined clauses (e.g. `When X, while Y, the system shall Z`) satisfy
  the event-driven form because the leading keyword and `THE SYSTEM SHALL` anchor
  the match.

---

### Gate: `approval`

| | |
|---|---|
| **Source** | `internal/core/gates/approval.go` |
| **Checks** | Approval records in `state.json` are consistent (records exist for all previously approved gates). |
| **Fails on** | Approval state inconsistency or sequence violation. |

---

### Gate: `sync`

| | |
|---|---|
| **Source** | `internal/core/gates/sync.go` |
| **Checks** | Task markers in `tasks.md` agree with the `task_status` map in `state.json`. |
| **Fails on** | Any task whose `tasks.md` marker disagrees with `state.json`. |

This invariant ensures that the two truth sources never drift. `specd task complete`
writes both atomically (under lock + CAS) to preserve agreement.

---

### Gate: `design`

| | |
|---|---|
| **Source** | `internal/core/gates/core.go` |
| **Checks** | (Active only when `approve design` is in progress) — verifies that `design.md` differs meaningfully from the scaffold stub and contains the required H2 sections. |
| **Fails on** | Design file unchanged from stub; missing `## Modules`, `## On-disk contracts`, or `## Invariants` sections. |

This gate is **armed by the caller** (`approveTarget == "design"`) — it is a no-op
during plain `specd check` unless the approve target is `design`.

---

## Opt-in Gate

### Gate: `security`

| | |
|---|---|
| **Source** | `internal/core/gates/security/` |
| **Enable** | `specd check <slug> --security` |
| **Checks** | Policy-level security checks (not content analysis). |
| **Fails on** | Security policy violations. |

Opt-in only; never runs by default. Add `--security` to `specd check` to include it.

---

## Gate Findings Format

Each gate emits zero or more findings:

```go
type Finding struct {
    Severity Severity  // "error" or "warn"
    Gate     string    // gate name
    Message  string    // human-readable description
}
```

**JSON output** (`--json`): an array of finding objects.

**Text output**: `<severity> <gate>: <message>` per line.

`specd check` exits `1` if any finding has `severity: error`.
`specd approve` blocks on any error-severity finding.

---

## Gate Composition and Extension

The gate registry (`gates.Registry`) is a slice of `Gate` interface values:

```go
type Gate interface {
    Name() string
    Run(CheckCtx) []Finding
}
```

Gates are pure functions of `CheckCtx` — they never read disk. The caller
(`buildCheckCtx` in `internal/cmd/registry.go`) reads all required files before
constructing `CheckCtx` and passing it to the registry.

To add a new gate, implement the `Gate` interface and call `registry.Register(gate)`
before running. See [contributor-guide.md](./contributor-guide.md) for the full
recipe.
