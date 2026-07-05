# Command Reference

All commands, flags, exit codes, and environment variables for `specd`.

> **Machine-readable:** `specd help --json` emits the full command palette as JSON.
> `specd help <command>` shows per-command detail.

---

## Command Palette

| Command | Description |
|---|---|
| [`specd version`](#specd-version) | Print build version metadata. |
| [`specd help`](#specd-help) | Show command help. |
| [`specd init`](#specd-init) | Initialize specd project state. |
| [`specd new`](#specd-new) | Create a new spec workspace. |
| [`specd approve`](#specd-approve) | Record human approval for a lifecycle gate. |
| [`specd check`](#specd-check) | Run the validation gate registry against a spec. |
| [`specd next`](#specd-next) | Select the next eligible task or wave. |
| [`specd verify`](#specd-verify) | Run and record task verification. |
| [`specd task`](#specd-task) | Show task details or mark a task complete. |
| [`specd status`](#specd-status) | Report current spec and task state. |
| [`specd context`](#specd-context) | Build the bounded context manifest for a task. |
| [`specd report`](#specd-report) | Render evidence-backed status and PR reports. |
| [`specd decision`](#specd-decision) | Record an explicit human decision. |
| [`specd midreq`](#specd-midreq) | Capture a scoped mid-stream requirement change. |
| [`specd memory`](#specd-memory) | Append or promote steering-memory patterns. |
| [`specd brain`](#specd-brain) | Run the opt-in deterministic orchestration controller. |
| [`specd mcp`](#specd-mcp) | Serve the MCP integration surface over stdio. |
| [`specd handshake`](#specd-handshake) | Emit bootstrap or policy handshake material. |
| [`specd review`](#specd-review) | Scaffold the review report the auditor fills before completion. |
| [`specd submit`](#specd-submit) | Gate-check, then stream the PR summary to the operator submit command. |
| [`specd link`](#specd-link) | Record that one spec depends on another (cross-spec ordering). |
| [`specd unlink`](#specd-unlink) | Remove a cross-spec dependency link. |
| [`specd triage`](#specd-triage) | *(Deferred)* Extended-loop triage tier. |

---

## specd version

```unknown
specd version [--json]
```

Print build metadata. Plain output is human-readable; `--json` emits machine-readable version, commit, build date, Go version, OS, architecture, and dirty-state fields when available.

**Exit codes:** `0` `2`

## specd help

```
specd help [command] [--json]
```

Show usage. With a command name, shows per-command help. With `--json`, emits the
complete command palette as machine-readable JSON (same structure as `core.Commands`).

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable help. |

**Exit codes:** `0` success, `2` unknown command.

---

## specd init

```
specd init [--agent=<name>] [--repair|--refresh] [--dry-run]
```

Initialize `.specd/` scaffold: writes embedded role templates, steering constitution
files, and merges `AGENTS.md` into the repo root. Safe to re-run — idempotent and
atomic; a rerun on a healthy project changes zero bytes.

Every managed asset is written inside stable marker comments
(`<!-- specd:managed:<asset>:v<N> begin/end -->`), so re-init preserves any content
you add **outside** the markers.

| Flag | Description |
|---|---|
| `--agent=<name>` | Select agent harness (informational; configures the `agent` key in state). |
| `--repair` | Restore managed regions that drifted from their template. |
| `--refresh` | Update managed regions to the current binary's template version. |
| `--dry-run` | Print the managed-region changes (diff-style) and write nothing. |

> **Contract — repair overwrites inside the markers.** `--repair`/`--refresh`
> regenerate the content **between** the managed markers from the embedded
> template; anything you hand-edited there is **replaced**. Content outside the
> markers is byte-for-byte preserved. Run with `--dry-run` first to see exactly
> what would change before it is written.

**What gets created:**
- `.specd/roles/` — `scout.md`, `craftsman.md`, `validator.md`, `auditor.md`
- `.specd/steering/` — `reasoning.md`, `workflow.md`, `product.md`, `tech.md`, `structure.md`
- `AGENTS.md` (at repo root, merged idempotently)

**Exit codes:** `0` success, `2` usage error.

---

## specd new

```
specd new <name> [--agent=<name>]
```

Create a new spec workspace under `.specd/specs/<name>/`. Fails if the spec already
exists. The slug must be lowercase alphanumeric with hyphens only.

Creates:
- `requirements.md` — EARS stub
- `design.md` — Design stub (Modules / On-disk contracts / Invariants sections)
- `tasks.md` — Task table stub
- `memory.md` — Steering-memory stub
- `state.json` — Initial state at revision 0 (`status: requirements`)

Creation is atomic under the per-spec advisory lock.

| Flag | Description |
|---|---|
| `--agent=<name>` | Select agent harness (stored in state). |

**Exit codes:** `0` success, `1` spec already exists, `2` usage/slug error.

---

## specd approve

```
specd approve <spec> <gate>
```

Record human approval for a lifecycle gate and advance the spec's status. Refuses
(exit 1) if any core validation gate fails. On success, appends an `approval:<gate>`
record to `state.json` via CAS.

**Valid gates:** `requirements`, `design`, `tasks`, `executing`, `verifying`, `complete`

The `design` gate also arms the design-stub check, which verifies the design
document has been meaningfully filled out before approval.

**Example:**

```bash
specd approve my-feature requirements   # requirements → design
specd approve my-feature design         # design → tasks
specd approve my-feature tasks          # tasks → executing
specd approve my-feature complete       # close the spec
```

**Exit codes:** `0` success, `1` gate failure or invalid gate name, `2` usage error.

---

## specd check

```
specd check <spec> [--security] [--json]
specd check --security [--json]
```

Run the validation gate registry against a spec. Prints findings and exits `1` if
any gate emits an `error`-severity finding. With `--security` and no spec, runs
only the repo-wide security scanners (no spec required).

**Core gates (always run):**

| Gate | What it checks |
|---|---|
| `task-ids` | All task IDs are non-empty and unique |
| `dependencies` | All `depends-on` references point to existing task IDs |
| `dag` | No dependency cycles (acyclic) |
| `roles` | Every task has a non-empty `role` |
| `files` | Every task has a non-empty `files` field |
| `verify` | Every task has a non-empty `verify` command |
| `evidence` | No task is marked complete without a passing verify record |
| `context-budget` | Context token estimates stay within `config.context.max_tokens` |
| `ears` | Requirements follow a recognized EARS pattern |
| `approval` | Approval sequence integrity (records match expected gates) |
| `sync` | `tasks.md` markers agree with `state.json` task statuses |
| `design` | Design document is non-empty and has the required H2 sections (when approving design) |

**Opt-in gate:**

| Flag | Gate | Description |
|---|---|---|
| `--security` | security | Three deterministic scanners over tracked files: secrets (format + entropy), injection (prompt-injection heuristics), slopsquat (typosquat dependencies). |

Per-scanner severity is set in `project.yml` (`security.secrets`,
`security.injection`, `security.slopsquat`, each `off|warn|error`; defaults
secrets=error, injection=warn, slopsquat=warn). Findings judged benign are
waived by exact fingerprint with a required reason in
`.specd/security/allow.json`. Scanners read `git ls-files` only, excluding
checksum manifests, `.specd/`, `testdata/`, `vendor/`, and `reference/`. See
docs/validation-gates.md for the full scanner reference.

| Flag | Description |
|---|---|
| `--security` | Include the opt-in security gate. |
| `--json` | Emit machine-readable findings (array of `{severity, gate, message}`). |

**Exit codes:** `0` all gates pass, `1` one or more gates emit errors, `2` usage error.

---

## specd next

```
specd next <slug> [--json | --waves | --dispatch]
```

Compute and display the currently runnable task frontier (tasks whose dependencies
are all complete and which are not yet complete themselves).

Requires the spec to be in `tasks` or `complete` status (or have both
`approval:requirements` and `approval:design` records).

| Flag | Description |
|---|---|
| `--waves` | Show all wave groups as JSON. |
| `--dispatch` | Emit the bounded context manifest for the first frontier task (JSON). |
| `--json` | Emit machine-readable frontier list. |

**Example:**

```bash
specd next my-feature                # prints: T1\nT2
specd next my-feature --waves        # JSON array of wave arrays
specd next my-feature --dispatch     # JSON context manifest for T1
```

**Exit codes:** `0` success, `1` gate not met (not yet approved), `2` usage error.

---

## specd verify

```
specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>]
specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text-or-path>
```

**Task mode** runs a task's `verify:` shell command (via `sh -c`) and **always** appends
an evidence record regardless of whether it passes or fails. The record contains:

```json
{"task_id":"T1","command":"go test ./...","exit_code":0,"git_head":"abc1234..."}
```

A warning is printed if git HEAD is unresolved — such evidence will not count toward
`task complete`.

**Criterion mode** (`--criterion`) records a per-acceptance-criterion evidence record
instead of running a command. The `<r>.<n>` id addresses acceptance criterion `<n>` of
requirement `R<r>` (sub-bullets under a requirement; a bare requirement is `<r>.1`).
Records are append-only — a later pass never erases a prior fail — and pin the current
git HEAD, same discipline as task verify. An unknown criterion id fails closed (exit 2).
A criterion record is **operator-supplied evidence**: unlike a task verify it runs no
command, and it can never substitute for a task's passing verify record. Coverage is
surfaced per requirement by `specd status` and `specd report`.

```json
{"type":"criterion","criterion":"1.2","status":"pass","evidence":"covered by T3","git_head":"abc1234...","timestamp":"…","actor":"…"}
```

| Flag | Description |
|---|---|
| `--revert-on-fail` | Restore working tree on verify failure (git diff + apply). |
| `--sandbox` | Run inside bwrap/container sandbox (fail-closed if binary absent). |
| `--sandbox-binary=<path>` | Path to sandbox binary (overrides auto-detect). |
| `--criterion <r>.<n>` | Record acceptance-criterion evidence instead of running a task verify. |
| `--status pass\|fail` | Criterion verdict (with `--criterion`). |
| `--evidence <text-or-path>` | Evidence backing the criterion verdict (with `--criterion`). |
| `--tokens <int>` | Optional worker-reported token count, stored verbatim. |
| `--cost <decimal>` | Optional worker-reported cost (decimal string), stored verbatim. |
| `--duration-ms <int>` | Optional worker-reported wall-clock milliseconds, stored verbatim. |

**Cost telemetry (stored, never computed).** The `--tokens/--cost/--duration-ms`
flags attach the host worker's usage to the evidence record verbatim. specd never
counts tokens, estimates, or derives cost — it only records what the worker
reports. Every field is optional; a worker that cannot report cost still produces
valid records. Malformed values (non-integer tokens/duration, non-decimal or
negative cost) fail closed (exit 2) without writing. `report --metrics` aggregates
them with exact decimal math.

**Exit codes:** `0` verify passed / criterion recorded, `1` verify command exited
non-zero, `2` usage error (unknown criterion id, missing evidence, bad status,
malformed telemetry).

---

## specd task

```
specd task <id>
specd task <id> --override --reason <text>
specd task complete <spec> <id>
```

### `specd task <id>`

Searches all specs for the task with the given ID and prints its details:

```
T1 [my-feature] craftsman
  files:      src/foo.go
  depends-on: -
  verify:     go test ./...
  acceptance: R1
```

Add `--json` for machine-readable output.

### `specd task <id> --override --reason <text>`

Clears an **escalated** task (the escalation ratchet). After
`escalation.max_verify_fails` consecutive failing verifies (default 3; set to
`0` to disable the ratchet entirely), a task is blocked: further `verify`
attempts and `task complete` refuse, and neither `next` nor the Brain will pick
it up, until a human clears it here. The override:

- **is not a bypass** — it only resets the consecutive-failure counter. The task
  still needs a genuine passing verify record to complete; the evidence
  requirement is never waived.
- **requires a non-empty `--reason`** — a reason-less override exits `2`.
- **refuses a task that is not escalated** — exits `2`.

Each override is appended to `.specd/specs/<slug>/overrides.jsonl` with the
actor, timestamp, and the count of failures it cleared. After an override, run
`specd verify <spec> <id>` again to re-attempt. `status` surfaces escalated
tasks (blocked when the ratchet is active, advisory when disabled).

### `specd task complete <spec> <id>`

Evidence-gated task completion. Under the per-spec advisory lock:

1. Verifies a passing evidence record exists (exit code 0, real git HEAD).
2. Writes `✅` marker to `tasks.md` (atomic write).
3. Updates `state.json` task status map via CAS.

Both writes are atomic and consistent — the Sync gate enforces agreement between them.

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable task row (for `specd task <id>` only). |
| `--override` | Clear an escalated task (resets the verify-failure ratchet; not a bypass). Requires `--reason`. |
| `--reason <text>` | Human justification for `--override` (required, non-empty). |
| `--tokens <int>` | Optional worker-reported token count, stored verbatim (`complete`). |
| `--cost <decimal>` | Optional worker-reported cost (decimal string), stored verbatim (`complete`). |
| `--duration-ms <int>` | Optional worker-reported wall-clock milliseconds, stored verbatim (`complete`). |

`task complete` accepts the same optional cost-telemetry flags as `verify`,
recorded verbatim on a supplementary evidence record (stored, never computed).

**Exit codes:** `0` success, `1` no passing evidence / deps not complete, `2` usage error / malformed telemetry.

---

## specd status

```
specd status <slug> [--json] | specd status --program
```

Render the current spec status: phase, task table, approval records, and evidence
summary.

With `--json`, emits the full report model plus all `state.json` records as
`RawMessage` (round-trips exactly — no re-synthesis).

With `--program` (no spec argument), emits the cross-spec program view: every
spec with its phase and dependency links, and the **program frontier** — the
specs whose dependencies are all complete and are therefore actionable now.

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable status + records. |
| `--program` | Show the cross-spec program view: specs, links, phases, and frontier. |

**Exit codes:** `0` success, `2` usage error.

---

## specd context

```
specd context <slug> <task-id> [--json | --hud]
```

Build the bounded context manifest for a task — the minimal set of files an agent
needs to read to execute the task, scoped by token budget.

Default output: one file path per line.

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable context manifest. |
| `--hud` | Render operator HUD (files, bytes, estimated tokens, mode). |

**Exit codes:** `0` success, `1` task not found, `2` usage error.

---

## specd report

```
specd report <spec> [--pr | --metrics | --json | --history | --format prometheus]
```

Render an evidence-backed report. Default (no flags) renders the same human-readable
output as `specd status`.

| Flag | Description |
|---|---|
| `--pr` | Emit a PR-oriented summary (task table + evidence summary). |
| `--metrics` | Emit a metrics summary: task counts by status, plus aggregated cost telemetry (tokens, duration, exact-decimal cost) per spec and per task. Tasks with no telemetry are marked present=0 — absence is shown, never imputed. |
| `--json` | Emit machine-readable report model (JSON Lines of events with `--history`). |
| `--history` | Replay the spec's audit trail — approvals, decisions, verify attempts, completions, criteria, submissions, ACP claims — in timestamp order. Derived purely from existing records; writes nothing. Byte-identical across runs. |
| `--format prometheus` | Emit Prometheus textfile-collector metrics (see the metric contract below). |

`--history` output is one line per event: `timestamp | actor | event | reference`.
Empty fields render as `-`. Ties (equal or absent timestamps) break by a fixed
source-type order then record position, so repeated runs are byte-identical.

### Prometheus metric contract

Metric names are an API — renaming one breaks dashboards, so these names are
stable. All carry the `specd_` prefix and a `spec="<slug>"` label.

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `specd_tasks` | gauge | `spec`, `status` | Tasks in each status. |
| `specd_verify_attempts_total` | counter | `spec` | Verify attempts recorded in the evidence ledger. |
| `specd_verify_failures_total` | counter | `spec` | Verify attempts that exited non-zero. |
| `specd_criteria` | gauge | `spec`, `verdict` | Acceptance criteria by verdict (`passing`, `total`). |
| `specd_escalated_tasks` | gauge | `spec` | Tasks blocked awaiting human override (0 until escalation adopted). |
| `specd_worker_tokens_total` | counter | `spec` | Worker-reported tokens, summed. |
| `specd_worker_cost_total` | counter | `spec` | Worker-reported cost, exact-decimal sum. |
| `specd_worker_duration_seconds_total` | counter | `spec` | Worker-reported wall-clock seconds, summed. |

**Exit codes:** `0` success, `2` usage error.

---

## specd decision

```
specd decision <spec> --text <rationale> [--scope <label>]
```

Append an architectural decision record to `state.json`. The record is stamped
with a timestamp, git HEAD, and actor.

| Flag | Description |
|---|---|
| `--text=<text>` | Decision rationale (required). |
| `--scope=<label>` | Optional scope label (e.g. `architecture`, `security`). |

**Exit codes:** `0` success, `2` usage error (missing `--text`).

---

## specd midreq

```
specd midreq <spec> --text <change> [--scope <scope>]
```

Record a mid-stream requirement change. The record is numbered (`midreq:0`,
`midreq:1`, …) and appended to `state.json` under the per-spec lock.

| Flag | Description |
|---|---|
| `--text=<text>` | Change description (required). |
| `--scope=<label>` | Optional scope label. |

**Exit codes:** `0` success, `2` usage error (missing `--text`).

---

## specd memory

```
specd memory <slug> <add|promote> [flags]
```

Append or promote steering-memory patterns in `.specd/specs/<slug>/memory.md`.
This is the **learning flywheel**: patterns extracted from spec work are promoted
to steering files when they reach the promotion threshold.

### `specd memory <slug> add`

| Flag | Description |
|---|---|
| `--key=<key>` | Pattern key (H2 heading). |
| `--pattern=<text>` | One-line pattern statement. |
| `--body=<text>` | Detail of the pattern. |
| `--source=<text>` | Where the pattern came from. |
| `--criticality=<level>` | `minor`, `important`, or `critical`. |
| `--related=<keys>` | Comma-separated related keys → wikilinks. |

### `specd memory <slug> promote`

| Flag | Description |
|---|---|
| `--key=<key>` | Key of the pattern to promote. |
| `--force` | Promote past the threshold (`config.promotion_threshold`, default: 3). |

**Exit codes:** `0` success, `1` pattern not found / promote failed, `2` usage error.

---

## specd brain

```
specd brain <start|step|run|status|cancel|resume> <spec> [--authority]
```

Run the opt-in deterministic orchestration controller. **No LLM sits in this path** —
`Sense` and `Decide` are pure functions of `state.json` and frontier state.

**Requires:** `orchestration.enabled: true` in project config, and the spec must
be in `mode: orchestrated`.

| Subcommand | Description |
|---|---|
| `start` | Initialize a Brain session for the spec. Fails closed if a session already exists. |
| `step` | Run one controller step (observe → decide → dispatch). Refused on a terminal session. |
| `run` | Alias for `step`. |
| `status` | Print the derived session status (`running`/`cancelled`/`complete`/`crashed`), last checkpoint step/time, and live lease holders. |
| `cancel` | Drive the session to the terminal `cancelled` state and release its lease. Task and evidence state are untouched; a second cancel is idempotent. |
| `resume` | Reconstruct the controller from the last checkpoint reconciled against the ledger, re-issuing a dispatch only when its mission id never reached the ledger. Refused (exit 1) on an irreconcilable checkpoint/ledger conflict, or when the session is running with a live lease. |

**Crash safety:** Each dispatch fsyncs a write-ahead checkpoint naming a
deterministic mission id (`session/step/task`) *before* the dispatch becomes
visible in the ledger, so `resume` re-issues a lost dispatch exactly once and never
double-dispatches. `crashed` is derived by `status` from a checkpoint that outran
the ledger — it is never a persisted state. See ADR 0006.

**Fail-closed:** Without `--authority`, the controller observes and reports but
writes nothing. With `--authority`, it can dispatch frontier tasks and record leases
in `session.json` and `acp.jsonl`.

| Flag | Description |
|---|---|
| `--authority` | Grant dispatch authority (fail-closed by default). |

**Exit codes:** `0` success, `1` precondition failure (orchestration not enabled, spec not in orchestrated mode, terminal session, or irreconcilable resume conflict), `2` usage error.

---

## specd mcp

```
specd mcp
specd mcp --config <host> [--root <path>] [--spec <slug>]
```

**Server mode** (no flags): start the MCP JSON-RPC 2.0 server over stdio. Exposes
all non-forbidden commands as MCP tools. Clients send JSON-RPC requests; the
server routes them to command handlers.

All core commands except `handshake` and `mcp` itself are exposed as tools.
Input schema is `{"type":"object","additionalProperties":true}` — tool arguments
are forwarded as flags.

**Config mode** (`--config <host>`): print a paste-ready MCP server config snippet
wiring `specd mcp` for the named host, so you don't hand-write the JSON. Known
hosts: `claude-code` (more can be added). An unknown host exits `2` listing the
known hosts.

| Flag | Description |
|---|---|
| `--config <host>` | Print a config snippet for the host instead of serving. |
| `--root <path>` | Pin the server working directory (`cwd`) in the snippet. |
| `--spec <slug>` | Pin the active spec (`SPECD_SPEC` env) in the snippet. |

**Exit codes:** `0` stream closed cleanly / snippet printed, `1` server error,
`2` unknown host.

---

## specd handshake

```
specd handshake bootstrap [--json] [--expect-palette-digest <d>] [--expect-config-digest <d>]
```

Emit bootstrap handshake material for host integration and diagnostics: the
version, the available tool list, and two **digests** — a SHA-256 of the canonical
`help --json` command palette and a SHA-256 of the effective config. Digests are
stable across runs and change when a verb/flag is added or config changes, so an
agent can detect that its cached palette or config is stale.

| Subcommand | Description |
|---|---|
| `bootstrap` | Emit version, tool list, and palette/config digests. |

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable handshake. |
| `--expect-palette-digest <d>` | Exit `1` if the command-palette digest differs from `<d>`. |
| `--expect-config-digest <d>` | Exit `1` if the effective-config digest differs from `<d>`. |

**Drift detection.** An agent caches the digests from a prior handshake and passes
them back with `--expect-*-digest`; a mismatch exits `1` naming which digest
drifted, so the agent knows to re-fetch the palette before relying on it.

**Exit codes:** `0` success, `1` digest drift, `2` usage error.

---

## specd review

```
specd review <spec> [--force]
```

Scaffold `.specd/specs/<spec>/review_report.md` from an embedded template: the
spec slug, the **git HEAD under review**, a per-task section (id, files,
acceptance), and the fields the reviewer fills — `Verdict`
(`approve | reject | needs-changes`), `Reviewer`, and `Findings`.

| Flag | Description |
|---|---|
| `--force` | Overwrite an existing report already scaffolded for the current git HEAD. |

The report is the deterministic half of review: the **auditor** role fills it,
and the opt-in `review.required` gate (below) reads it. A craftsman reviewing its
own work is a documented anti-pattern — the harness cannot verify reviewer
identity, so this is a discipline the operator enforces, not the binary.

**Phases:** valid in `execute`, `verify`, or `reflect`. **Exit codes:** `0`
success, `1` report already exists for the current HEAD (without `--force`),
`2` usage or out-of-phase.

---

## specd submit

```
specd submit <spec> [--resubmit]
```

Terminal verb. Runs the full gate registry and refuses (exit `1`, listing every
failing gate and incomplete task) unless every gate is green and every task is
complete. When gates pass, it generates the deterministic PR summary — the same
generator as `report --pr`, one implementation — and streams it on **stdin** to
the command configured at `submit.command`, run through the sandboxed exec path
with a timeout (`submit.timeout_seconds`, default 120s).

The binary embeds no git/GitHub logic: the operator command owns transport
(e.g. `gh pr create --fill -F -`, a `curl`, a mail pipe). `submit.command` is a
**shell line** (run via `/bin/sh -c`), not an argv vector.

| Flag | Description |
|---|---|
| `--resubmit` | Allow resubmitting a spec already submitted at the current git HEAD. |

- **Dry-run by default:** with no `submit.command` configured, `submit` prints
  the summary to stdout and exits `0` — nothing is recorded.
- **Ledger:** a run against a configured command appends a submission record
  `{git_head, summary_hash, command, exit, timestamp, actor}` to
  `.specd/specs/<spec>/submissions.jsonl`.
- **Idempotence:** a second submit at the same git HEAD is refused (exit `1`)
  unless `--resubmit` is given — a guard against double-fires from orchestration.

**Phases:** valid only in `execute`, `verify`, or `reflect` (a spec must be
executing or past it). **Exit codes:** `0` success/dry-run, `1` gates or tasks
not ready / command failed / duplicate submission, `2` usage or out-of-phase.

---

## specd link

```
specd link <from-slug> <to-slug>
```

Record a cross-spec dependency: `<from-slug>` depends on `<to-slug>`, so `<to>`
must complete before `<from>` may execute. Links live at the program level in
`.specd/program.json` (versioned, atomic), never inside a spec's `state.json`.

Linking is idempotent and cycle-refused: a link that would create a cycle in the
cross-spec graph is rejected with the offending path printed. Links are
enforcement, not annotation — `approve <from> executing` is refused while any
dependency is incomplete (see `status --program` for the frontier).

**Exit codes:** `0` success, `1` would create a cycle, `2` unknown slug or usage.

---

## specd unlink

```
specd unlink <from-slug> <to-slug>
```

Remove a cross-spec dependency link. Removing a link that does not exist fails
closed.

**Exit codes:** `0` success, `2` no such link or usage error.

---

## specd triage

```
specd triage <spec>
```

> **Deferred.** This command is registered but not yet wired. Running it prints:
> `specd triage: deferred — not yet wired` and exits 0.

---

## Exit Code Semantics

| Code | Meaning |
|---|---|
| `0` | Success / validation passed |
| `1` | Gate failure, verification failure, evidence missing, or config/policy error |
| `2` | Usage error (wrong arguments, unknown flag, out-of-phase verb, out-of-enum flag) |

Every command declares its full exit-code table in metadata; `specd help --json`
emits them per command. Fail-closed rejections — unknown verb, a flag value
outside its declared enum, or an execution verb run in a disallowed phase —
exit `2` before any side effect, never `1`.

---

## Lifecycle Phase Compatibility

Each spec advances through lifecycle phases derived from its status:

| Status | Phase |
|---|---|
| `requirements` | `perceive` |
| `design` | `analyze` |
| `tasks` | `plan` |
| `executing` | `execute` |
| `verifying` | `verify` |
| `complete` | `reflect` |

Most verbs run in any phase. **Execution verbs** — `next`, `verify`, `context`,
and `brain` — are gated: a spec still in the `perceive` (requirements) phase has
no approved design or task DAG to act on, so these verbs fail closed (exit `2`)
there, naming the current and allowed phases. The check is a single dispatch
choke point that runs after spec resolution and before any handler side effect;
a rejected verb leaves `state.json` untouched. Approve the `design` gate to
advance a spec out of `perceive` and unlock execution verbs.

---

## Machine-Readable Help Contract

`specd help --json` emits a versioned payload:

```json
{ "schema_version": 1, "commands": [ { "name": "verify", "usage": "…",
  "flags": [ { "name": "sandbox", "type": "bool", … } ],
  "allowed_phases": ["analyze", "plan", "execute", "verify", "reflect"],
  "exit_codes": [ { "code": 0, "meaning": "success" }, … ], "examples": [ … ] } ] }
```

`schema_version` is the stable `HelpSchemaVersion` contract; consumers (the MCP
server, role prompts, external tooling) pin against it and detect shape changes.
Flag `enum`/`default` map directly into MCP JSON Schema — command metadata is the
single source of truth, so no surface hand-restates command semantics.

---

## Environment Variables

| Variable | Description |
|---|---|
| `SPECD_ACTOR` | Override the actor name stamped on records (default: OS username from `user.Current()`). |

---

## Config Keys

Config lives in `project.yml` at the repository root (optional; defaults apply when
absent). YAML only; two-space indentation; `.yml` extension required.

| Key | Default | Description |
|---|---|---|
| `version` | `"1"` | Config schema version |
| `agent` | `"codex"` | Agent harness name |
| `context.max_tokens` | `12000` | Max token budget for context manifests |
| `gates.verify` | `"error"` | Verify gate severity (`error` or `warn`) |
| `orchestration.enabled` | `false` | Enable Brain orchestration |
| `orchestration.model` | `""` | Model identifier (informational) |
| `criteria.required` | `false` | Opt-in: refuse the completion approval until every acceptance criterion has a current passing record |
| `review.required` | `false` | Opt-in: refuse the completion approval unless `review_report.md` has an `approve` verdict recorded at the current git HEAD |
| `submit.command` | `""` | Operator shell line `specd submit` streams the PR summary to on stdin; empty ⇒ dry-run (print summary, exit 0) |
| `submit.timeout_seconds` | `120` | Timeout bounding the submit command |
| `promotion_threshold` | `3` | Memory pattern promotion threshold |
