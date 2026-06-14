# Validation Gates

`specd check <slug>` runs 7 strict gates on a spec. Two further gates run on the
whole repository. A gate failure exits `1` and blocks the relevant `specd approve`
transition.

## The 7 spec gates

### Gate 1 — EARS
- **Source:** `internal/core/ears.go`
- **Checks:** Every requirement has a user story; every acceptance criterion matches one of the five EARS patterns.
- **Fails on:** Invalid grammar, missing user story, malformed criteria.

### Gate 2 — Design
- **Source:** `internal/core/phases.go`
- **Checks:** All 7 mandatory H2 headers present, non-empty, no `TODO` markers.
- **Fails on:** Missing header, empty section, placeholder text.

### Gate 3 — Task schema
- **Source:** `internal/core/tasksparser.go`
- **Checks:** Every task has the 7 mandatory keys (`why`, `role`, `files`, `contract`, `acceptance`, `verify`, `depends`).
- **Fails on:** Missing key; a `builder`/`verifier` task with `verify: N/A`.

### Gate 4 — DAG
- **Source:** `internal/core/dag.go`
- **Checks:** Acyclic dependencies, no orphan deps, valid wave numbering.
- **Fails on:** Circular dependency, missing task ID, wave violation.

### Gate 5 — Evidence
- **Source:** `internal/cmd/check.go`, `internal/core/state.go`
- **Checks:** No task is complete without evidence; non-read-only tasks require a passing verify record.
- **Fails on:** A complete task with no verify record.

### Gate 6 — Sync
- **Source:** `internal/core/specfiles.go`
- **Checks:** Markdown checkbox statuses match `state.json` task statuses.
- **Fails on:** Mismatch between `tasks.md` and `state.json`.

### Gate 7 — Traceability
- **Source:** `internal/core/specfiles.go`
- **Checks:** Every requirement ID referenced in tasks exists in `requirements.md`.
- **Severity:** Controlled by `config.gates.traceability` (`warn` or `error`).

## Repo-global freshness gates

These run on the whole repository (no spec slug).

| Gate | Command | Source | Checks |
|---|---|---|---|
| **Boot-freshness** | `specd check --boot` | `internal/core/boot.go` | `boot.json` still matches the repo — source files exist, no detection drift |
| **Enrich-freshness** | `specd check --enrich` | `internal/core/enrich_evidence.go` | Agent-authored steering enrichment is present, complete, and not drifted from `boot` |
