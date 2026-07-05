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
specd init [--agent=<name>]
```

Initialize `.specd/` scaffold: writes embedded role templates, steering constitution
files, and merges `AGENTS.md` into the repo root. Safe to re-run — idempotent and
atomic; a rerun on a healthy project changes zero bytes.

| Flag | Description |
|---|---|
| `--agent=<name>` | Select agent harness (informational; configures the `agent` key in state). |

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
```

Run the validation gate registry against a spec. Prints findings and exits `1` if
any gate emits an `error`-severity finding.

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
| `--security` | security | Opt-in security checks (policy-level, not content). |

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

**Exit codes:** `0` verify passed / criterion recorded, `1` verify command exited
non-zero, `2` usage error (unknown criterion id, missing evidence, bad status).

---

## specd task

```
specd task <id>
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

### `specd task complete <spec> <id>`

Evidence-gated task completion. Under the per-spec advisory lock:

1. Verifies a passing evidence record exists (exit code 0, real git HEAD).
2. Writes `✅` marker to `tasks.md` (atomic write).
3. Updates `state.json` task status map via CAS.

Both writes are atomic and consistent — the Sync gate enforces agreement between them.

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable task row (for `specd task <id>` only). |

**Exit codes:** `0` success, `1` no passing evidence / deps not complete, `2` usage error.

---

## specd status

```
specd status <slug> [--json]
```

Render the current spec status: phase, task table, approval records, and evidence
summary.

With `--json`, emits the full report model plus all `state.json` records as
`RawMessage` (round-trips exactly — no re-synthesis).

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable status + records. |

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
specd report <spec> [--pr | --metrics | --json]
```

Render an evidence-backed report. Default (no flags) renders the same human-readable
output as `specd status`.

| Flag | Description |
|---|---|
| `--pr` | Emit a PR-oriented summary (task table + evidence summary). |
| `--metrics` | Emit a metrics summary (task counts by status). |
| `--json` | Emit machine-readable report model. |

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
specd brain <start|step|run|status> <spec> [--authority]
```

Run the opt-in deterministic orchestration controller. **No LLM sits in this path** —
`Sense` and `Decide` are pure functions of `state.json` and frontier state.

**Requires:** `orchestration.enabled: true` in project config, and the spec must
be in `mode: orchestrated`.

| Subcommand | Description |
|---|---|
| `start` | Initialize a Brain session for the spec. |
| `step` | Run one controller step (observe → decide → dispatch). |
| `run` | Alias for `step`. |
| `status` | Print the current session JSON. |

**Fail-closed:** Without `--authority`, the controller observes and reports but
writes nothing. With `--authority`, it can dispatch frontier tasks and record leases
in `session.json` and `acp.jsonl`.

| Flag | Description |
|---|---|
| `--authority` | Grant dispatch authority (fail-closed by default). |

**Exit codes:** `0` success, `1` precondition failure (orchestration not enabled, or spec not in orchestrated mode), `2` usage error.

---

## specd mcp

```
specd mcp
```

Start the MCP JSON-RPC 2.0 server over stdio. Exposes all non-forbidden commands
as MCP tools. Clients send JSON-RPC requests; the server routes them to command
handlers.

All core commands except `handshake` and `mcp` itself are exposed as tools.
Input schema is `{"type":"object","additionalProperties":true}` — tool arguments
are forwarded as flags.

**Exit codes:** `0` stream closed cleanly, `1` server error.

---

## specd handshake

```
specd handshake [bootstrap|policy] [--json]
```

Emit bootstrap or policy handshake material for host integration and diagnostics.

| Subcommand | Description |
|---|---|
| `bootstrap` | Emit version and available tool list. |
| `policy` | *(reserved)* |

| Flag | Description |
|---|---|
| `--json` | Emit machine-readable handshake. |

**Exit codes:** `0` success, `2` usage error.

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
| `promotion_threshold` | `3` | Memory pattern promotion threshold |
