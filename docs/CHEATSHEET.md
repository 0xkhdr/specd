# specd — Command Reference

> **Source of truth.** Every verb, flag, exit code, and allowed phase on this page is
> generated to match `internal/core/commands.go` (`var Commands`, `HelpSchemaVersion 1`).
> `docs/CHEATSHEET.md` is a byte-identical copy of this file; `scripts/docs-lint.sh`
> fails CI if they drift. Edit this file, then copy it over the cheatsheet.

`specd <verb> [args] [flags]`. Run `specd help` for the live palette or
`specd help <verb>` for one command. `specd help --json` emits the machine-readable
palette (`schema_version` + `commands[]`) that dispatch, MCP, and role prompts pin against.

## Conventions

**Exit codes** (every verb, unless noted):

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | gate or verify failure |
| `2` | usage error or fail-closed rejection |

Unknown verbs and disallowed flag values **fail closed (exit 2)**. A verb run outside its
allowed lifecycle phase is rejected (exit 2). Deferred verbs print a deferral notice and
exit 0 — they never silently no-op.

**Phase enforcement.** A verb that resolves a spec is checked against that spec's current
phase. `any` = valid in every phase. `post-requirements` = `analyze · plan · execute ·
verify · reflect` (fails closed while a spec is still in the `perceive`/requirements phase).
`post-execution` = `execute · verify · reflect` (terminal verbs need completed work to act on).

---

## Lifecycle

### `init`
```
specd init [--agent=<name>] [--repair|--refresh] [--dry-run]
```
Initialize or re-sync specd project state and managed assets. Scaffolds `.specd/`, writes
`AGENTS.md`, and a commented `project.yml` (with an active `verify.timeout_seconds: 600`
bound) into the project root; an existing `project.yml` is never overwritten. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |
| `--repair` | bool | Restore drifted managed regions from the current templates. |
| `--refresh` | bool | Update managed regions to the current binary's template version. |
| `--dry-run` | bool | Print the managed-region changes and write nothing. |

```bash
specd init
specd init --agent=pinky
specd init --repair --dry-run
specd init --refresh
```

### `agents`
```
specd agents [doctor | guide <slug>] [--json]
```
Inspect installed agent artifacts, run read-only diagnostics with `doctor`, or emit deterministic driver actions with `guide`. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit JSON. |

```bash
specd agents
specd agents doctor --json
specd agents guide payments --json
```

### `new`
```
specd new <name> [--agent=<name>]
```
Create a new spec workspace. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |

```bash
specd new payments
specd new payments --agent=codex
specd new payments --agent=pinky
```

### `approve`
```
specd approve <spec> <gate>
```
Record human approval for a lifecycle gate. Advances a phase only when the gate registry
passes. **Phases:** any.

```bash
specd approve payments requirements
specd approve payments design
```

### `midreq`
```
specd midreq <spec> --text <change> [--scope <scope>]
```
Capture a scoped mid-stream requirement change. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--text` | string | Change description (required). |
| `--scope` | string | Optional scope label. |

```bash
specd midreq payments --text 'add refund path' --scope requirements
```

### `decision`
```
specd decision <spec> --text <rationale> [--scope <scope>]
```
Record an explicit human decision. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--text` | string | Decision rationale (required). |
| `--scope` | string | Optional scope label. |

```bash
specd decision payments --text 'defer webhooks' --scope design
```

---

## Execution

### `next`
```
specd next <slug> [--json | --waves | --dispatch]
```
Select the next eligible task or wave. **Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--waves` | bool | Show all wave groups as JSON. |
| `--dispatch` | bool | Emit the context manifest for the first frontier task. |
| `--json` | bool | Emit machine-readable frontier list. |

```bash
specd next payments
specd next payments --json
```

### `task`
```
specd task <id> [--override --reason <text>] | specd task complete <spec> <id>
```
Show task details, clear an escalated task with a human override, or mark a task complete
(requires passing evidence). **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable task row. |
| `--override` | bool | Clear an escalated task (resets the verify-failure ratchet; does not complete it). Requires `--reason`. |
| `--reason` | string | Human justification for `--override` (required, non-empty). |
| `--tokens` | string | Optional worker-reported token count, stored verbatim (`task complete`). |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim (`task complete`). |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim (`task complete`). |

```bash
specd task T3 --json
specd task T3 --override --reason 'flaky infra, verified manually'
specd task complete payments T3
```

### `check`
```
specd check <spec> [--security] [--json]
```
Run the validation gate registry against a spec. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--security` | bool | Run opt-in security gates. These scan selected tracked files; they do not yet scan `.specd/` or enforce role capability, declared-file diff scope, or mandatory sandboxing. |
| `--schema` | bool | Validate `state.json` schema. |
| `--schema-only` | bool | Validate only `state.json` schema. |
| `--json` | bool | Emit machine-readable findings. |

```bash
specd check payments
specd check payments --security --json
```

### `verify`
```
specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>]
specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>
```
Run and record task verification (task mode), or record a per-acceptance-criterion evidence
record (`--criterion` mode). A task completes **only** against a passing verify record
(exit 0 pinned to a resolvable git HEAD). **Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--revert-on-fail` | bool | Restore working tree on verify failure. |
| `--sandbox` | bool | Run the verify line inside a bwrap sandbox (fail-closed if the binary is absent). |
| `--sandbox-binary` | string | Path to sandbox binary (overrides auto-detect). |
| `--criterion` | string | Record evidence for acceptance criterion `<r>.<n>` instead of running a task verify. |
| `--status` | `pass`\|`fail` | Criterion verdict (with `--criterion`). |
| `--evidence` | string | Evidence text or path backing the criterion verdict (with `--criterion`). |
| `--tokens` | string | Optional worker-reported token count, stored verbatim. |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim. |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim. |

```bash
specd verify payments T3
specd verify payments T3 --revert-on-fail
specd verify payments --criterion 1.2 --status pass --evidence 'covered by T3 integration test'
```

### `context`
```
specd context <slug> <task-id> [--json|--hud]
```
Build the bounded context manifest for a task. **Phases:** post-requirements.

Manifest schema **v1** is the current compatibility renderer; a typed **v2** contract is being introduced additively. Unknown or unsupported manifest versions fail closed rather than being reinterpreted.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable context. |
| `--hud` | bool | Render the operator HUD (files, bytes, tokens, mode). |

```bash
specd context payments T3
specd context payments T3 --hud
```

### `review`
```
specd review <spec> [--force]
```
Scaffold the review report the auditor fills before completion. **Phases:** post-execution.

| Flag | Value | Description |
|---|---|---|
| `--force` | bool | Overwrite an existing report for the current git HEAD. |

```bash
specd review payments
specd review payments --force
```

### `submit`
```
specd submit <spec> [--resubmit]
```
Run every gate, then stream the PR summary to the operator-configured submit command.
**Phases:** post-execution.

| Flag | Value | Description |
|---|---|---|
| `--resubmit` | bool | Allow resubmitting a spec already submitted at the current git HEAD. |

```bash
specd submit payments
specd submit payments --resubmit
```

---

## Inspection

### `help`
```
specd help [command] [--json]
```
Show command help. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable help. |

```bash
specd help
specd help --json
```

### `version`
```
specd version [--json]
```
Print build version metadata. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON. |

```bash
specd version
specd version --json
```

### `status`
```
specd status [spec] [--json] | specd status <spec> --guide [--json] | specd status --program
```
Report current spec and task state, machine driving guidance, or the cross-spec program view. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable status. |
| `--guide` | bool | Emit machine driving guidance: phase, required artifact, legal commands, human-only actions, and blockers. |
| `--program` | bool | Show the cross-spec program view: specs, links, phases, and frontier. |

```bash
specd status payments
specd status payments --json
specd status payments --guide --json
specd status --program
```

### `report`
```
specd report <spec> [--pr|--metrics|--json|--history|--trace|--format prometheus]
```
Render evidence-backed status, PR, history, trace, and metrics reports. Deterministic — generated
from `state.json` + task artifacts, never from an LLM. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--pr` | bool | Emit PR-oriented report. |
| `--metrics` | bool | Emit metrics summary. |
| `--json` | bool | Emit machine-readable report (JSON Lines with `--history`). |
| `--history` | bool | Replay the spec's audit trail from existing records in timestamp order. |
| `--trace` | bool | Export the metadata-only run trace as stable JSON Lines. |
| `--format` | `prometheus` | Alternate output format; prometheus emits textfile-collector metrics. |

```bash
specd report payments --pr
specd report payments --metrics
specd report payments --history
specd report payments --trace
specd report payments --format prometheus
```

---

## Integration

### `memory`
```
specd memory <slug> <add|promote> [flags]
```
Append or promote steering-memory patterns (learning flywheel). **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--key` | string | Pattern key (H2 heading). |
| `--pattern` | string | One-line pattern statement (`add`). |
| `--body` | string | Detail of the pattern (`add`). |
| `--source` | string | Where the pattern came from (`add`). |
| `--criticality` | `minor`\|`important`\|`critical` | Criticality (`add`). |
| `--related` | string | Comma-separated related keys → wikilinks (`add`). |
| `--force` | bool | Promote past the threshold (`promote`). |

```bash
specd memory payments add --key 'atomic writes' --pattern 'use AtomicWrite'
```

### `mcp`
```
specd mcp | specd mcp --config <host> [--root <path>] [--spec <slug>]
```
Serve the MCP integration surface over stdio, or print a host config snippet. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--config` | string | Print a paste-ready MCP config snippet for a host (e.g. `claude-code`). |
| `--root` | string | Pin the server working directory in the snippet. |
| `--spec` | string | Pin the active spec in the snippet. |

```bash
specd mcp
specd mcp --config claude-code --spec demo
```

### `handshake`
```
specd handshake bootstrap [--json] [--expect-palette-digest <d>] [--expect-config-digest <d>]
```
Emit bootstrap handshake material, including palette and config digests. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable handshake. |
| `--expect-palette-digest` | string | Fail (exit 1) if the command-palette digest differs. |
| `--expect-config-digest` | string | Fail (exit 1) if the effective-config digest differs. |

```bash
specd handshake bootstrap
specd handshake bootstrap --json
```

### `link`
```
specd link <from-slug> <to-slug>
```
Record that one spec depends on another (cross-spec ordering). **Phases:** any.

```bash
specd link api auth
```

### `unlink`
```
specd unlink <from-slug> <to-slug>
```
Remove a cross-spec dependency link. **Phases:** any.

```bash
specd unlink api auth
```

---

## Orchestration

### `brain`
```
specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report> <spec> [args] [--authority]
```
Run the opt-in deterministic orchestration controller. No LLM sits in its decision path.
`run` records a pending mission/dispatch for every currently-ready, unleased task (one wave) and
returns. It does **not** launch a worker, agent, model, or adapter. Workers explicitly `claim` a
pending mission, renew its typed lease with `heartbeat`, then `report` passing current evidence.
Report validates mission/lease/worker/role/HEAD, derives the local diff and scope verdict, and calls
normal task completion. Pending dispatch remains no proof of delivery or work.
**Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--authority` | bool | Grant dispatch authority (fail-closed by default). |

```bash
specd brain start payments --authority
specd brain claim payments payments.s1.T1 worker-1 craftsman
specd brain heartbeat payments <lease-id> worker-1
specd brain report payments <lease-id> worker-1
specd brain status payments
specd brain resume payments
specd brain cancel payments
```

---

## Deferred

### `triage`
```
specd triage <spec>
```
Run the opt-in extended-loop triage tier. **Deferred:** registered but not wired — prints a
deferral notice and exits 0. **Phases:** any.

```bash
specd triage payments
```
