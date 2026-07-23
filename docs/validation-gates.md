# specd — Validation Gates

> The agent reasons. The harness enforces. **Gates are the enforcement.**

`specd check <spec>` runs the gate registry against a spec's on-disk `.specd/` state and
returns findings. Gates are **pure functions** of `CheckCtx` (`internal/core/gates/core.go`):
the caller reads files and state, the gate bodies never touch disk, and **no LLM sits in any
gate path**. A gate with zero-valued inputs is disabled (an empty `CheckCtx` yields no
findings), so opt-in gates stay dormant until their config arms them.

`specd approve <spec> <gate>` advances a lifecycle phase **only** when the relevant gates
pass. `specd submit` runs every gate before streaming a PR.

Quality declarations use optional task-table `evidence` and `checks` columns. Plain `verify`
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

## The 26 core gates

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
| 16 | `coverage` | *(opt-in)* Requirements, design, and task acceptance coverage has no configured gap. |
| 17 | `evidence-policy` | *(opt-in)* Declared integration boundaries carry required integration and negative-path evidence. |
| 18 | `intake` | *(opt-in)* `provenance.json` supplies every configured typed-intake field; empty and `unknown` both fail readiness. |
| 19 | `governance` | *(opt-in)* Required decisions are accepted and active; expired blocking exceptions fail closed with owner/review action. |
| 20 | `memory-lint` | *(production profile)* Active memory has no duplicate normalized keys, explicit critical contradictions, or unowned forced promotions. |
| 21 | `quality-declaration` | Each task's `evidence=` declaration is well-formed `class/check-id` (valid classes: test/output_eval/trajectory_eval/review); a non-test class also warns that a plain verify cannot satisfy it and names the `specd eval import` producer. |
| 22 | `dispatch-parity` | Every task row parses through the same `core.ParseTaskContract` the dispatcher uses, so a closed-vocabulary field (e.g. `kind`) outside the canonical set is rejected at the tasks gate rather than at dispatch; every nonconforming row is reported. |
| 23 | `palette-scope` | A row declaring a CLI handler under `internal/cmd/` must also declare the command palette (`internal/core/commands.go`) and the gendocs source (`tools/gendocs/main.go`); the arg parser separately rejects any flag absent from a command's palette, fail-closed, so a functional-but-undocumented flag cannot ship. |
| 24 | `acceptance-reach` | *(warning + scope error)* Warns when a cited requirement id is already referenced in Go sources only outside the row's declared files (the criterion may be unreachable within declared scope); errors when a production-kind row's acceptance names a Go path no declared file can produce (a distinct scope-versus-acceptance finding). |
| 25 | `verify-lint` | A write task's verify command is not a trivially-passing no-op; a `go test -run` selector must declare a matching test file and name a real `func Test`. |
| 26 | `steering-applicability` | *(warning)* Warns when every `.specd/steering/*.md` is dropped from the machine manifest for missing `specd-context` metadata; per-file omission stays silent. |

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
  production planning profile, so minimal six-column `tasks.md` tables keep planning.
- **`intake` (18)** is armed by a non-empty `required_fields` list in versioned
  `provenance.json`. Missing files and empty policies leave feature-spec behavior unchanged;
  malformed provenance and configured fields whose value is empty or `unknown` fail closed.
- **`governance` (19)** is armed only when governance policy is configured. It rejects missing or
  proposed required decisions and expired blocking exceptions, naming owner and review action;
  unconfigured projects remain unchanged. Arming takes **both** conditions: `config.profile =
  production` *and* at least one governance record on disk — outside the production profile the
  records are never even loaded, so a malformed or expired one stays silent. Governance decisions
  are a **declared input** you author at `.specd/specs/<slug>/decisions.json`, not CLI output —
  there is deliberately no verb that writes them, and an absent file means governance is
  unconfigured rather than failing. Note that `specd decision` is a different, unrelated surface:
  it appends a free-text record to `state.json` for the audit trail and never touches this file.
  **Every** decision in the file is treated as required, so a record left `proposed`, or one whose
  `expires_at` has passed, blocks approval until its owner accepts or supersedes it.
  The shape is a bare JSON array of records, each requiring `id`, `status`
  (`proposed|accepted|superseded|expired|revoked`), `owner`, and RFC3339 `created_at`,
  `review_at`, and `expires_at`; optional `supersedes` must name an existing, not-yet-superseded
  id, and `affected_invariants` links the decision to drift invariants. **A decision counts as
  active only while `status: accepted` and `expires_at` is in the future** — an accepted decision
  with a past `expires_at` silently stops applying, so keep `review_at` ahead of it.

  ```json
  [{"id": "D1", "status": "accepted", "owner": "0xkhdr", "created_at": "2026-07-19T00:00:00Z",
    "review_at": "2026-10-19T00:00:00Z", "expires_at": "2027-01-19T00:00:00Z",
    "affected_invariants": ["INV-EVIDENCE"]}]
  ```

  Governance **exceptions** live beside them at `.specd/specs/<slug>/exceptions.json`, same array
  shape and same required fields, plus `blocking`. Only `blocking: true` exceptions are checked,
  and only to fail closed once expired; a non-blocking exception is inert. Beware the name
  collision: `specd exception approve|revoke` does **not** write this file — it appends to the
  separate security ledger `.specd/security/exceptions.jsonl` (a different record shape:
  `finding`/`action`/`reason`/`ticket`/`owner`/`scope`/`revision`/`environment`/`issued_at`/
  `expires_at`/`compensating_control`/`approver`), which suppresses security findings by
  fingerprint and whose mere presence switches the security gate off `.specd/security/allow.json`.
  That verb refuses to waive evidence integrity or worker authority.

  `specd drift` reads the same declared-input model from `.specd/specs/<slug>/drift.json` — an
  object `{"schema_version": 1, "invariants": [...]}` whose entries need `id`, a
  workspace-relative `path`, an `evidence_task` naming the task whose verify evidence proves the
  invariant, and `severity` (`unknown|low|medium|high|critical`). It projects those against
  evidence and writes nothing; with no `drift.json` it reports a single `none` finding.
- **`criteria` (13)** is armed by `config.criteria.required = true` or the production lifecycle
  profile (`config.profile = production`). Until then it is dormant. Record criterion evidence
  with `specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>`.
- **`review` (14)** is armed by `config.review.required = true` or the production lifecycle
  profile (`config.profile = production`). A missing or malformed `review_report.md`, a stale
  HEAD, or a non-approve verdict all fail closed. Scaffold the report with `specd review <spec>`.
- **Production profile.** `config.profile = production` (spec 01 R7.2) is the single switch that
  raises the whole bar: it arms the `criteria` and `review` ratchets and the integration /
  negative-path evidence policy together, so each need not be enabled by hand. `default` (R7.1)
  keeps every one of them opt-in. The effective policy is surfaced as
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
  which stores fingerprints/digests), `vendor/`, `.git/`.

---

## Fixing a failing gate

1. `specd check <spec> --json` to read the exact findings.
2. Address the root cause in the artifact the gate names (`requirements.md`, `design.md`,
   `tasks.md`, or task evidence) — do **not** hand-edit `state.json`.
3. For evidence/criteria failures, run the real `specd verify` so a passing record is written.
4. Re-run `specd check`; then `specd approve <spec> <gate>` to advance the phase.

See [troubleshooting.md](troubleshooting.md) for blocked tasks, the verify-failure ratchet,
and `task --override`.
