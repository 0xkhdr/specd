# specd — Validation Gates

> The agent reasons. The harness enforces. **Gates are the enforcement.**

`specd check <spec>` runs the gate registry against a spec's on-disk `.specd/` state and
returns findings. Gates are **pure functions** of `CheckCtx` (`internal/core/gates/core.go`):
the caller reads files and state, the gate bodies never touch disk, and **no LLM sits in any
gate path**. A gate with zero-valued inputs is disabled (an empty `CheckCtx` yields no
findings), so opt-in gates stay dormant until their config arms them.

`specd approve <spec> <gate>` advances a lifecycle phase **only** when the relevant gates
pass. `specd submit` runs every gate before streaming a PR.

Quality declarations use optional task-table `evidence` and `checks` columns. Legacy `verify`
evidence satisfies only class `test`; it never satisfies output, trajectory, or review proof.
Quality gates stay offline and consume validated local records only.

Quality packets and reports keep passed, missing, stale, score, and review labels separate.
Review contracts expose integration, error, concurrency, and rollback risks; approval cannot
replace required current test evidence. Learning records are redacted, append-only, and
source-digest pinned.

## Severity & exit codes

| Severity | Effect |
|---|---|
| `Error` | Fails the gate. `check`/`approve`/`submit` exit non-zero. |
| `Warn` | Printed, but the gate passes. |

`specd check` exits `0` when no error findings, `1` on a gate failure, `2` on usage/fail-closed.
Add `--json` for machine-readable findings.

---

## The 17 core gates

Registered by `CoreRegistry()` in the order they run:

| # | Gate | What it checks |
|---|---|---|
| 1 | `task-ids` | Every task has a non-empty id; no duplicate ids. |
| 2 | `dependencies` | Every `depends_on` reference points at a task that exists. |
| 3 | `dag` | The task graph is acyclic (`core.NewTaskDAG` builds without error). |
| 4 | `roles` | Each task declares a known role (scout/craftsman/validator/auditor). |
| 5 | `files` | Task file declarations are well-formed / present as required. |
| 6 | `verify` | Every task carries a verify command (read-only tasks use a trivially-passing line, e.g. `printf ok`). |
| 7 | `evidence` | A task marked complete is backed by a passing verify record (exit 0 pinned to a real git HEAD). |
| 8 | `context-budget` | The per-task context manifest fits within `context.max_tokens`. |
| 9 | `ears` | `requirements.md` uses valid EARS syntax (Easy Approach to Requirements Syntax). |
| 10 | `approval` | The lifecycle transition being approved has its prerequisite human approvals recorded. |
| 11 | `sync` | Machine truth (`state.json`) and marker truth (`tasks.md`) agree on task status. |
| 12 | `design` | `design.md` is filled past its scaffold stub (armed when approving the `design` gate). |
| 13 | `criteria` | *(opt-in)* Every acceptance criterion has a current passing evidence record. |
| 14 | `review` | *(opt-in)* `review_report.md` carries an `approve` verdict recorded at the current git HEAD. |
| 15 | `task-trace` | A task's declared requirement `refs` resolve and its `risk` tier is known; *(opt-in)* under the production planning profile every task declares refs/kind/risk/context/evidence/checks. |

### Notes on individual gates

- **`evidence` (7)** is the non-negotiable core: there is **no bypass flag**. A task completes
  only against a verify record whose exit code is 0 and whose git HEAD still resolves. See
  [concepts.md](concepts.md). Set `verify.timeout_seconds` (or `SPECD_VERIFY_TIMEOUT_SECONDS`)
  to bound a single verify command; a timeout is recorded as a **failing** evidence record
  (exit 124), never a hang. Default `0` is unbounded.
- **`sync` (11)** catches the two-source drift: `tasks.md` markers say one thing, `state.json`
  says another. It fails closed so a hand-edited marker can't fake completion.
- **`design` (12)** only fires when the gate under approval is `design`; it compares
  `design.md` against the scaffold stub (single-source stub comparison, no hard-coded prose),
  and refuses a design whose declared `references:` name an unknown requirement. The full
  decision contract (boundaries/interfaces/invariants/failure/integration/alternatives/
  disposition/owner) is required only under the production design profile.
- **`task-trace` (15)** always refuses an unresolvable requirement reference or an unknown
  risk tier declared on a task; the full trace/risk contract is required only under the
  production planning profile, so legacy six-column `tasks.md` tables keep planning.
- **`criteria` (13)** is armed by `config.criteria.required = true` or the production lifecycle
  profile (`config.profile = production`). Until then it is dormant. Record criterion evidence
  with `specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>`.
- **`review` (14)** is armed by `config.review.required = true` or the production lifecycle
  profile (`config.profile = production`). A missing or malformed `review_report.md`, a stale
  HEAD, or a non-approve verdict all fail closed. Scaffold the report with `specd review <spec>`.
- **Production profile.** `config.profile = production` (spec 01 R7.2) is the single switch that
  raises the whole bar: it arms the `criteria` and `review` ratchets and the integration /
  negative-path evidence policy together, so each need not be enabled by hand. `default` (R7.1)
  keeps every one of them opt-in for backward compatibility. The effective policy is surfaced as
  a `policy_digest` on the bootstrap handshake so a later approval can detect that the judgment
  policy has moved.
- **program links** — when approving the execution transition, incomplete cross-spec
  dependencies (`specd link`) refuse the approval. Planning phases are never program-gated.

---

## Security gates (opt-in)

Run with `specd check <spec> --security`. Off by default. The gate scans **git-tracked**
working-tree files and resolves each scanner's severity from `config.security` (`off` |
`warn` | `error`). Defaults are tuned so a real secret blocks while noisier heuristics warn.

| Scanner | Config field | Finds |
|---|---|---|
| `secrets` | `security.secrets` | High-entropy / known-shape credentials committed to tracked files. |
| `injection` | `security.injection` | Prompt-injection markers in content the agent may ingest. |
| `slopsquat` | `security.slopsquat` | Dependency names that look like typosquats of popular packages. |

**Severity semantics:** `error` findings fail the gate (exit 1); `warn` findings print but
pass; `off` skips the scanner entirely.

### Allowlist

Findings can be suppressed by fingerprint via an allowlist. Allowlisted findings drop out of
the gate result (they don't fail the build) but are still recorded separately for reports.
A load error in the allowlist **fails closed** (surfaces as an error finding).

### Scan boundary

The scanner deliberately excludes files that only yield false positives, so they never fail
your build:

- Dependency checksum manifests: `go.sum`, `package-lock.json`, `yarn.lock`,
  `pnpm-lock.yaml`, `Cargo.lock`.
- Directories: `testdata/` (synthetic fixtures), `.specd/` (the harness's own runtime state,
  which stores fingerprints/digests), `reference/` (frozen v1 museum), `vendor/`, `.git/`.

---

## Fixing a failing gate

1. `specd check <spec> --json` to read the exact findings.
2. Address the root cause in the artifact the gate names (`requirements.md`, `design.md`,
   `tasks.md`, or task evidence) — do **not** hand-edit `state.json`.
3. For evidence/criteria failures, run the real `specd verify` so a passing record is written.
4. Re-run `specd check`; then `specd approve <spec> <gate>` to advance the phase.

See [troubleshooting.md](troubleshooting.md) for blocked tasks, the verify-failure ratchet,
and `task --override`.
