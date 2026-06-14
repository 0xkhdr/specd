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

## Security model

`tasks.md` is **agent-authored input**, not trusted config. The harness treats
every `verify:` line and every env var as hostile until validated.

- **Shell execution.** `specd verify` runs each task's `verify:` line via
  `sh -c` (override with `SPECD_VERIFY_SHELL`) as the invoking user. Pipelines
  are intentional, so this is real code execution — **only run `specd verify`
  on a `tasks.md` you trust.** Mitigations: the child environment is scrubbed
  to an allowlist (`PATH`, `HOME`, `LANG`, `LC_ALL`, `TMPDIR`, and `SPECD_*`),
  dropping inherited secrets; commands containing a NUL byte are rejected; the
  exact command and cwd are printed before execution for audit.
- **Path inputs.** Spec slugs are validated against `^[a-z0-9][a-z0-9-]*$`
  (`internal/core/slug.go`), so `..`, `/`, `\`, and leading `-` are rejected —
  no path traversal.
- **Self-update / install.** `specd update` and `install.sh` download
  `SHA256SUMS` from the same release and verify the archive digest before
  replacing any binary. Both **fail closed** on a missing or mismatched
  checksum. `install.sh --no-verify` skips the check with a loud warning.
- **Lock file.** A spec's `.lock` holds `PID epochMillis` only — it is
  **non-secret** and created `0644`. Written artifacts (`state.json`,
  `tasks.md`) land as `0644` minus umask.
- **Env vars.** All `SPECD_*` integer vars are parsed through
  `core.EnvInt(name, def, min, max)`, which clamps to range and emits one
  warning on malformed input instead of silently falling back.
