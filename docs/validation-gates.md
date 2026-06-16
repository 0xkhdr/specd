# Validation Gates

`specd check <slug>` runs the validation gates on a spec. A gate failure exits
`1` and blocks the relevant `specd approve` transition. **Seven core gates always
run**; three further gates (`acceptance`, `scope`, custom) are opt-in and no-ops
by default, so the baseline behaviour is unchanged.

## The 7 core gates

### Gate 1 — EARS
- **Source:** `internal/core/ears.go`
- **Checks:** Every requirement has a user story; every acceptance criterion matches one of the five EARS patterns.
- **Fails on:** Invalid grammar, missing user story, malformed criteria.

**Recognized EARS forms** (`MatchEars`, case-insensitive, checked in order):

| Form | Pattern | Shape |
|---|---|---|
| Unwanted behaviour | `unwanted` | `IF <condition> THEN THE SYSTEM SHALL <response>` |
| Event-driven | `event-driven` | `WHEN <trigger> THE SYSTEM SHALL <response>` |
| State-driven | `state-driven` | `WHILE <state> THE SYSTEM SHALL <response>` |
| Optional feature | `optional-feature` | `WHERE <feature> THE SYSTEM SHALL <response>` |
| Ubiquitous | `ubiquitous` | `THE SYSTEM SHALL <response>` |

Notes:
- Matching is case-insensitive, so `when …`/`When …` are accepted.
- A criterion is the text after `N.` inside the **Acceptance criteria:** block;
  leading whitespace before the number is allowed (`criterionRe`).
- Complex/combined clauses (e.g. `When X, while Y, the system shall Z`) satisfy
  the event-driven form because the leading keyword and `THE SYSTEM SHALL`
  anchor the match; the embedded `while` is part of the trigger text.
- Ubiquitous is matched last so a conditional form is never mis-tagged as
  ubiquitous.

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

## Opt-in gates (no-op by default)

These run after the seven core gates only when enabled. With their defaults,
`specd check` output is byte-identical to a build without them.

### Gate 8 — Acceptance
- **Source:** `internal/core/gates.go` (`GateAcceptance`)
- **Config:** `config.gates.acceptance` — `off` (default), `warn`, or `error`.
- **Checks:** For tasks that declare an `acceptance:` mapping (e.g.
  `acceptance: 1.1, 1.2`), every referenced criterion exists in
  `requirements.md`, and a complete task has a recorded **pass** for each mapped
  criterion (via `specd verify <slug> --criterion <r>.<n> --status pass`).
- **Fails on:** A mapping to an undefined criterion, or a complete task with a
  mapped criterion that has no recorded pass. Enforcement only — no LLM judgment.

### Gate 9 — Scope (diff-scope evidence)
- **Source:** `internal/core/gates.go` (`GateScope`)
- **Config:** `config.gates.scope` — `off`/`*`/unset = no-op, else `warn`/`error`.
- **Checks:** Files changed during `specd verify` (captured into the verify
  record) fall within the task's declared `files:` contract.
- **Fails on:** A task that touched a file outside its declared `files:` glob set.
  Coverage is recorded as **evidence**, not enforced as a numeric floor.

### Custom gates
- **Source:** `internal/core/customgate.go`
- **Config:** `config.gates.custom` — a list of `{name, command, severity}`.
- **Checks:** Each is an external program run after the core pipeline via the
  verify shell with a scrubbed env and bounded timeout (no Go plugins, no
  network). Findings map to violations (`error`) or warnings (`warn`).
- See [Custom Gates](./custom-gates.md) for the stdin/stdout JSON contract.

## Steering is agent-authored, not gated

There are no repo-global freshness gates. Bootstrapping the steering constitution
(`product.md`, `structure.md`, `tech.md`, and `config.defaultVerify`) is agent work,
driven by the `specd-steering` skill — the agent inspects the repo and authors these
files itself. The harness scaffolds (`init`) and enforces specs (`check`); it does
not perceive the stack or police steering freshness.

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
  *Note on Custom Gates*: Custom gates configured in `.specd/config.json` also run as external commands under the verify shell, but **do not run within the verify sandbox (bubblewrap or containers)**. Ensure custom gate commands are trusted before running `specd check`.
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
