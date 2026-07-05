# Validation Gates

`specd check <slug>` runs the validation gate registry against a spec. A gate
failure exits `1` and blocks the relevant `specd approve` transition.

**Thirteen core gates are registered.** Most always run; a few (`design`,
`criteria`) are caller-armed and inert unless a matching `approve` transition is
in progress. One additional gate (`security`) is opt-in and a no-op by default.

---

## Core Gates

All thirteen core gates are registered in `CoreRegistry()` (`internal/core/gates/core.go`).
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

### Gate: `criteria`

| | |
|---|---|
| **Source** | `internal/core/gates/approval.go` |
| **Checks** | (Active only when `criteria.required` is on **and** `approve complete` is in progress) — every acceptance criterion has a current passing record. |
| **Fails on** | Any criterion with no passing record recorded after the last requirements approval. |

This is the opt-in per-acceptance-criterion ratchet (spec 04). It is doubly
armed: `config criteria.required = true` **and** the completion transition
(`approveTarget == "complete"`). Default off, so existing flows are unbroken.

**Evidence asymmetry.** A criterion record is *operator-supplied* — it carries
evidence text or a path and runs **no command**, unlike a task verify record
which executes the task's `verify:` line and pins the resulting exit code. The
two are stored separately (`criteria.jsonl` vs `evidence.jsonl`) and a criterion
record can **never** substitute for a task's passing verify. This gate therefore
only *strengthens* the evidence story; it introduces no bypass. "Current" means
recorded after the latest requirements approval — re-approving requirements
invalidates stale attestations by construction, no mutation needed.

---

### Gate: `review`

| | |
|---|---|
| **Source** | `internal/core/gates/review.go` |
| **Checks** | (Active only when `review.required` is on **and** `approve complete` is in progress) — `review_report.md` carries an `approve` verdict recorded at the current git HEAD. |
| **Fails on** | Missing/malformed report, a `reject`/`needs-changes` verdict, or an `approve` verdict pinned to a stale HEAD. |

This is the opt-in review ratchet (spec 09), doubly armed like `criteria`:
`config review.required = true` **and** the completion transition. Default off.

**HEAD freshness** is the load-bearing detail. An `approve` verdict counts only
when the report's `Git HEAD` line matches the commit the code is at now, exactly
as a verify record pins its exit code to a HEAD. A stale approval — carried over
from an older commit — does **not** count; re-review at the current commit. A
missing report, a missing/unknown verdict, or a missing HEAD line **fails closed**
and is never read as approve. `reject`/`needs-changes` surfaces the report's
findings section in the gate output.

**Who fills it.** The **auditor** role fills `review_report.md` (scaffold it with
`specd review <spec>`). A craftsman reviewing its own work is an anti-pattern —
the harness cannot verify reviewer identity, so the gate checks *that a review
exists and approves at this HEAD*, not *who* wrote it. That identity discipline
is the operator's to enforce. No LLM runs in the gate: the host agent writes the
report, the harness only checks it.

---

## Opt-in Gate

### Gate: `security`

| | |
|---|---|
| **Source** | `internal/core/gates/security/` |
| **Enable** | `specd check <slug> --security`, or `specd check --security` (repo-wide, no slug) |
| **Checks** | Three deterministic scanners over tracked files: **secrets**, **injection**, **slopsquat**. |
| **Fails on** | Any non-allowlisted finding whose per-scanner severity is `error`. |

Opt-in only; never runs by default. Add `--security` to `specd check` to include
it. With no slug (`specd check --security`) it runs the scanners repo-wide,
independent of any spec.

**Scanners** (each pure over tracked file contents + embedded rule data + the
allowlist — no network, no LLM, stable finding order):

- **secrets** — known-format credentials (`AKIA…`, `ghp_…`, PEM private-key
  blocks) plus high-entropy string literals (Shannon entropy over base64/hex
  tokens, length ≥ 24, thresholds base64 ≥ 4.6 / hex ≥ 3.6). Excerpts are
  redacted to the first and last 4 characters — the scanner never prints a
  candidate secret in full. Default severity **error**.
- **injection** — prompt-injection heuristics in tracked text/markdown:
  imperative override phrases, `you are now …` role overrides,
  hidden-instruction HTML comments, tool-exfiltration phrasing, and
  zero-width/bidi control-character smuggling. Versioned rule set. Default
  severity **warn**.
- **slopsquat** — parses `go.mod` and flags module paths within a small
  Damerau-Levenshtein distance (≤ 1 short, ≤ 2 for names ≥ 8 chars) of an
  embedded popular-package list; exact matches are never flagged. Default
  severity **warn**.

**Severity config** (`project.yml`): `security.secrets`, `security.injection`,
`security.slopsquat`, each `off|warn|error`. `error` findings fail the gate
(exit 1); `warn` findings print but pass; `off` skips the scanner entirely.

**Allowlist workflow** — a finding you have judged benign is waived in
`.specd/security/allow.json`, an array of `{ "fingerprint": "…", "reason": "…" }`
entries. The fingerprint is the SHA-256 of (rule id + relative path + matched
content), printed nowhere secret; every entry **must** carry a non-empty reason,
and a reason-less or malformed entry invalidates the whole allowlist (fail
closed). Because the fingerprint pins the matched content, moving the match to
another line keeps the waiver valid, while editing the matched text invalidates
it and re-surfaces the finding — the point of a *reasoned* allowlist.

**Tracked-files boundary** — the gate scans `git ls-files` output only (untracked
scratch never fails CI), and excludes dependency checksum manifests (`go.sum`,
`package-lock.json`, …), the harness's own `.specd/` runtime state, and
`testdata/`, `vendor/`, `reference/`, `.git/` trees — those hold fixtures,
checksums, or vendored copies that yield only false positives. Working tree only;
git history is not scanned.

Findings (including allowlisted ones) are recorded under `state.security` so
`report` and `report --history` can consume them.

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
