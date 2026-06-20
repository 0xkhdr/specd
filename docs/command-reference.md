# Command Reference

Every `specd` command, its flags, and exit codes — plus environment variables and
the `config.json` schema. This mirrors the embedded registry; run
`specd help --json` for the machine-readable form.

## Contents

- [Lifecycle commands](#lifecycle-commands)
- [Execution commands](#execution-commands)
- [Inspection commands](#inspection-commands)
- [Format & history commands](#format--history-commands)
- [Record commands](#record-commands)
- [Program commands](#program-commands)
- [Orchestration commands](#orchestration-commands)
- [Meta commands](#meta-commands)
- [Environment variables](#environment-variables)
- [Argument grammar](#argument-grammar)
- [Output streams](#output-streams)
- [Exit code semantics](#exit-code-semantics)
- [Config file](#config-file-specdconfigjson)

---

## Lifecycle commands

| Command | Description | Exit codes |
|---|---|---|
| `specd init [--agent <auto\|all\|none\|codex\|claude-code\|gemini\|cursor\|vscode>] [--scope <project\|global>] [--yes] [--non-interactive] [--dry-run] [--repair] [--refresh] [--json] [--verbose] [--force] [--list-packs] [--pack <name\|url> [--sha256 <hex>]] [--orchestration [<policy>]] [--orchestration-workers <n>] [--orchestration-retries <n>] [--orchestration-timeout <minutes>] [--orchestration-cost-limit <usd>] [--orchestration-mode <inline\|delegate>] [--orchestration-sandbox <none\|bwrap\|container>]` | Scaffold `.specd/` (steering, roles, skills) + `AGENTS.md`, then detect coding agents and install **project-scoped** MCP registration and verify it. `--agent auto` (default) configures the unambiguous host interactively, else scaffolds only; `--agent none` skips host config; `--scope global` needs `--yes`/consent; `--dry-run` previews mutations; `--repair`/`--refresh` restore/update managed assets; `--pack` applies a [spec pack](./spec-packs.md) (remote URL requires a pinned `--sha256`, fail-closed); `--orchestration` (defaults to `planning`) enables Brain/Pinky orchestration and sets the approval policy. | `0` ok, `1` write/config/handshake/partial failure, `2` usage / unavailable host, `3` operation needs an initialized root and none exists |
| `specd doctor [--agent <name\|all>] [--json] [--fix]` | Diagnose scaffold integrity, MCP server handshake/`tools/list`, and host-registration health with remediation commands. `--fix` applies only safe, project-scoped, specd-owned repairs | `0` healthy, `1` a check failed, `2` usage |
| `specd new <slug> [--title "..."]` | Create a spec with six artifact stubs | `0` ok, `2` usage, `3` no `.specd/` or spec exists |
| `specd approve <slug> [--json]` | Clear approval gate / advance to the next phase (human gate) | `0` ok, `1` gates failed, `2` usage, `3` not found |

## Execution commands

| Command | Description | Exit codes |
|---|---|---|
| `specd next <slug> [--all] [--json]` | Get the next runnable task, or the entire frontier with `--all` | `0` ok, `2` usage, `3` not found |
| `specd dispatch <slug> [--json]` | Emit ready-to-run subagent packets for the frontier | `0` ok, `2` usage, `3` not found |
| `specd verify <slug> <id> [--sandbox none\|bwrap\|container] [--revert-on-fail]` | Run the task's `verify:` command and record proof (exit code, output tail, duration, git HEAD, changed files, optional coverage). `--sandbox` overrides `verify.sandbox` for this run (fail-closed if the isolator is absent); `--revert-on-fail` stashes the working tree (recoverable) on a non-zero exit instead of leaving it dirty | `0` pass, `1` fail, `2` usage, `3` not found |
| `specd verify <slug> --criterion <r>.<n> --status pass\|fail --evidence "..."` | Record a per-acceptance-criterion proof (feeds the [acceptance gate](./validation-gates.md)) | `0` ok, `1` fail, `2` usage, `3` not found |
| `specd task <slug> <id> --status <s> [--evidence "..."] [--reason "..."] [--unverified] [--force] [--tokens <n>] [--cost <n>]` | Evidence-gated status flip; dual-writes `tasks.md` + `state.json`. `--tokens`/`--cost` annotate telemetry (stored, never computed) | `0` ok, `1` rejected, `2` usage, `3` not found |

> On verify timeout, the **task's recorded** exit code is `124` and the record is
> marked `verified: false`. The `specd verify` process itself still exits `1`.

## Inspection commands

| Command | Description | Exit codes |
|---|---|---|
| `specd status [<slug>] [--json]` | Progress board for a spec, or list all specs | `0` ok, `2` usage, `3` not found |
| `specd check <slug> [--json]` | Run the validation gates on a spec (7 core, plus opt-in acceptance/scope/custom gates) | `0` pass, `1` fail, `2` usage, `3` not found |
| `specd waves <slug> [--json]` | Wave graph, critical paths, blockers | `0` ok, `2` usage, `3` not found |
| `specd context <slug> [--json]` | Phase briefing + load list + signals | `0` ok, `2` usage, `3` not found |
| `specd report <slug> [--format md\|html] [--out <path>] [--pr-summary]` | Generate a snapshot report. `--pr-summary` emits a deterministic, network-free PR summary (Markdown, or JSON under `SPECD_JSON`) — wave/task progress, gate status, and the commit↔task link map | `0` ok, `2` usage, `3` not found |
| `specd serve <slug> [--addr 127.0.0.1:8765]` | Read-only HTTP dashboard: same HTML as `report --format html` at `GET /`, JSON `ReportData` at `GET /api/report`. No mutating routes | `0` ok, `2` usage, `3` not found |
| `specd watch [--once] [--spec <slug>] [--sse <addr>] [--webhook <url>]` | Stream a `FrontierEvent` whenever a spec's runnable task set changes. Read-only. Default NDJSON on stdout; `--sse` serves Server-Sent Events at `GET /events`; `--webhook` POSTs each event with bounded retry/backoff; `--once` does a single pass | `0` ok, `2` usage, `3` not found |

See [Validation Gates](./validation-gates.md) for what each `check` gate enforces.

## Format & history commands

| Command | Description | Exit codes |
|---|---|---|
| `specd schema [--version <v>]` | Emit the embedded, versioned [open spec format](./open-spec-format.md) JSON Schema to stdout. Needs no `.specd/` root | `0` ok, `1` unknown version, `2` usage |
| `specd validate <slug> --schema [--version <v>] [--json]` | Validate a spec's `state.json` against the embedded JSON Schema (structural/format conformance, independent of the semantic gates). Read-only | `0` conformant, `1` violations, `2` usage, `3` not found |
| `specd replay <slug> [--acp-session <id>]` | Reconstruct a deterministic, read-only event timeline (task start/finish/verify/block + acceptance records) from on-disk audit data, or replay events for a specific orchestration session. Text, or typed JSON array under `SPECD_JSON` | `0` ok, `2` usage, `3` not found |
| `specd diff <slug> --from <ref> [--to <ref>] [--json]` | Show how a spec's artifacts changed between two git refs — a read-only `git diff --name-status` scoped to the spec dir. `--to` defaults to the working tree. Under `--json`, returns structured file-diff lists. | `0` ok, `1` git diff failed, `2` usage, `3` not found |

## Record commands

| Command | Description | Exit codes |
|---|---|---|
| `specd decision <slug> "..." [--supersedes <id>]` | Append an ADR to `decisions.md`. `--supersedes` deprecates and references an existing ADR ID. | `0` ok, `2` usage, `3` not found |
| `specd midreq <slug> "..." --impact <low\|medium\|high\|critical> [--interpretation "..."] [--changes "..."]` | Log a mid-flight requirement update. `--interpretation` specifies the analyzed feedback; `--changes` lists implementation updates. | `0` ok, `2` usage, `3` not found |
| `specd memory <slug> add --key "..." --pattern "..." --body "..." --source "T1" --criticality important [--related "..."] [--force]` | Record a learning. `--criticality` specifies severity (`important`, `critical`); `--source` cites task/log source; `--pattern` matches recurrence. | `0` ok, `2` usage, `3` not found |
| `specd memory <slug> promote --key "..."` | Promote a learning to global steering. | `0` ok, `2` usage, `3` not found |

## Program commands

| Command | Description | Exit codes |
|---|---|---|
| `specd program [status] [--json]` | Render the spec-level DAG + runnable frontier (JSON for orchestrators) | `0` ok, `2` usage |
| `specd program link <spec> --on <dep>` | Declare an inter-spec dependency | `0` ok, `2` usage |
| `specd program unlink <spec> --on <dep>` | Remove an inter-spec dependency | `0` ok, `2` usage |

Edges are stored in `.specd/program.json`. Self-edges and cycles are rejected.

## Orchestration commands

`specd brain` and `specd pinky` are deterministic controllers over the same
state, locks, gates, verification records, and ACP file transport as the normal
CLI. They do **not** call an LLM and do **not** spawn provider-specific agents;
the host remains responsible for executing Pinky missions.

MCP exposes these commands as generated tools, not MCP-only handlers:
`specd_brain` receives `brain` subcommands in `args`, and `specd_pinky` receives
`pinky` subcommands in `args`. Every call is bounded. Use repeated `status` and
`step` calls for polling/reconciliation; do not expect one MCP request to wait
for a worker lifecycle to finish.

| Command | Description | Exit codes |
|---|---|---|
| `specd brain start <slug> --approval-policy <manual\|planning\|session> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--session <id>] [--cost-limit <usd>] [--json]` | Start one spec orchestration session and advance it by one step. Emits one deterministic decision: dispatch, wait, request approval, retry, cancel, escalate, or complete-session | `0` ok, `1` policy/gate/session failure, `2` usage, `3` not found |
| `specd brain run <slug> [--approval-policy <policy>] [--worker-cmd <cmd>] [--bootstrap] [--max-steps <n>] [--session <id>] [--json]` | Run the reference single-spec driver loop. Automatically loops step, dispatches missions to a host command, and blocks/reconciles to completion. | `0` ok, `1` policy/gate/session failure, `2` usage, `3` not found |
| `specd brain step <slug> --session <id> --approval-policy <manual\|planning\|session> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--cost-limit <usd>] [--json]` | Advance an existing spec session by one bounded decision | same |
| `specd brain status --session <id> [--json]` | Read persisted session state | `0` ok, `1` invalid/missing session, `2` usage |
| `specd brain why --session <id> [--json]` | Explain the latest replayable Brain decision for a session | `0` ok, `1` invalid/missing session, `2` usage |
| `specd brain directive --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --action <continue\|retry\|cancel\|reassign\|escalate> --reason <text> [--in-reply-to <message-id>] [--json]` | Send a bounded Brain directive to one active Pinky lease, usually in reply to a Pinky query | same |
| `specd brain pause\|resume\|cancel --session <id> [--json]` | Persist cooperative session control. `cancel` records intent; later steps emit cancellation directives but never kill host processes | `0` ok, `1` invalid/terminal session, `2` usage |
| `specd brain start --program [--session <id>] ... [--json]` / `specd brain step --program --session <id> ... [--json]` | Schedule child specs from the program DAG under the same explicit policy limits | same |
| `specd brain run --program [--approval-policy <policy>] [--worker-cmd <cmd>] [--max-steps <n>] [--session <id>] [--json]` | Run the program reference driver loop across multiple dependent specs. | `0` ok, `1` policy/gate/session failure, `2` usage, `3` not found |
| `specd brain status\|pause\|resume\|cancel --program --session <id> [--json]` | Read/control a parent program session | same |
| `specd pinky claim --mission <path\|-> [--json]` | Host worker claims a Brain-issued mission under an ACP lease | `0` ok, `1` invalid/stale/duplicate claim, `2` usage, `3` not found |
| `specd pinky brief --session <id> --worker <id> --spec <slug> (--task <id> [--attempt <n>] \| --artifact <name>) [--json]` | Render a paste-ready worker brief/prompt (or mission JSON under `--json`) for a dispatched task or authoring mission | `0` ok, `2` usage, `3` not found |
| `specd pinky heartbeat --session <id> --worker <id> --attempt <n> [--json]` | Renew the worker lease before expiry | same |
| `specd pinky progress --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --percent <0-100> --message <text> [--json]` | Record host-reported progress as ACP evidence; it is telemetry, not proof of completion | same |
| `specd pinky query --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --text <question> [--json]` | Ask one bounded mid-task question without releasing the lease; Brain or the host answers with `brain directive` | same |
| `specd pinky inbox --session <id> --worker <id> [--json]` | Read Brain directives addressed to a worker | same |
| `specd pinky report --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --verification-ref <ref> --summary <text> [--changed-files <csv>] [--git-head <sha>] [--duration-ms <n>] [--host-tokens <n>] [--host-cost <usd>] [--json]` | Record terminal worker evidence. Completion is accepted only when it binds to a matching passing `specd verify` record and satisfies the task scope/evidence gates. Host-reported tokens/cost/duration are stored verbatim as telemetry, not proof | same |
| `specd pinky block --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --reason <text> [--json]` | Record a worker blocker for Brain reconciliation | same |
| `specd pinky release --session <id> --worker <id> --attempt <n>` | Release a claim idempotently | same |

## Meta commands

| Command | Description | Exit codes |
|---|---|---|
| `specd update [--force]` | Self-update to the latest release | `0` ok, `1` failed, `2` usage |
| `specd uninstall [--force] [--dry-run] [--json]` | Remove the install dir, the `~/.local/bin/specd` link, and the `# specd` PATH lines from shell rc files (backed up). A bare invocation only previews; `--force` actually removes. Project-local `.specd/` dirs are always preserved | `0` ok, `1` failed, `2` usage |
| `specd mcp [--root <path>] [--http [<addr>]] [--config <host>]` | Run the [MCP](https://modelcontextprotocol.io) server (JSON-RPC 2.0), exposing every command as an MCP tool. Default: stdio transport (no network). `--http [<addr>]` starts HTTP/SSE on loopback (default `127.0.0.1:8765`); bare `--http` uses the default address. `--config <host>` prints a ready-to-paste config snippet and exits without starting a server. Stdlib-only, no LLM calls | `0` clean exit, `1` server error or bind failure, `2` usage |
| `specd version` | Show version | `0` ok |
| `specd help [command] [--json]` | Show help / dump the JSON command registry | `0` ok, `2` unknown command |

> **`specd update` verifies before replacing.** It downloads the release
> `SHA256SUMS`, checks the archive digest, and **fails closed** on mismatch — the
> same filename `install.sh` and `.goreleaser.yml` use.
>
> **Windows limitation:** `specd update` cannot replace the running `specd.exe`
> in place (Windows locks in-use executables). On Windows, reinstall from a fresh
> download instead. All other commands work normally. See TESTING.md.

---

## Environment variables

| Variable | Default | Effect |
|---|---|---|
| `SPECD_JSON` | `0` | Emit structured JSON for all commands (same as the `--json` flag) |
| `SPECD_LOCK_TIMEOUT_MS` | `5000` | Max wait for a spec advisory lock |
| `SPECD_LOCK_STALE_MS` | `30000` | Age at which an orphaned `.lock` file is auto-reclaimed |
| `SPECD_VERIFY_TIMEOUT_MS` | `600000` | Per-run timeout for `specd verify` |
| `SPECD_VERIFY_SHELL` | `sh` | Shell executable used to run each `verify:` line and custom-gate command (always invoked as `<shell> -c "<command>"`). On Windows, `sh` or an equivalent bash-like shell must be in the `PATH` (e.g. from Git for Windows); native `cmd.exe` is not supported due to the hardcoded `-c` argument. |
| `SPECD_WATCH_INTERVAL_MS` | `1000` | Poll interval for `specd watch` |
| `SPECD_CUSTOM_GATE_TIMEOUT_MS` | — | Per-gate wall-clock budget for [custom gates](./custom-gates.md) |
| `SPECD_SANDBOX_IMAGE` | — | Container image for `--sandbox container` ([verify sandboxing](../SECURITY.md)) |
| `SPECD_REDIS_ADDR` / `SPECD_REDIS_PREFIX` | — | Redis state backend (only in the `specd_redis` tagged build) |
| `SPECD_PG_DSN` / `SPECD_PG_DRIVER` | — | Postgres state backend (only in the `specd_postgres` tagged build) |
| `NO_COLOR` | — | Disable ANSI colors |

> The default binary links **no** DB/Redis driver; the Redis/Postgres backends
> compile in only under the `specd_redis` / `specd_postgres` build tags.

> The `--json` flag is equivalent to `SPECD_JSON=1` and is the recommended way to
> request machine-readable output.

## Argument grammar

`specd` uses a small custom parser (`internal/cli/args.go`) — no Cobra/urfave
(see [contributor guide](./contributor-guide.md) for the rationale). The grammar:

- **Positionals** — any token not starting with `--` (e.g. the `<slug>` and
  `<id>`). Order-significant.
- **Value flags** — `--key value` *or* `--key=value`. The `=` form splits on the
  first `=`, so `--evidence=a=b` yields `evidence` → `a=b`. Use the `=` form
  whenever the value could be mistaken for a flag.
- **Boolean flags** — registered in `booleanFlags` (`--force`, `--json`, `--all`,
  `--unverified`, `--dry-run`, `--list-packs`, `--once`, `--pr-summary`,
  `--revert-on-fail`, `--schema`). They take no value;
  `--json status` keeps `status` as a positional. A test
  (`TestBooleanFlagsRegistered`) asserts every `args.Bool(...)` flag used in
  `internal/cmd` is registered, so a forgotten registration can never silently
  consume the next token.

A bare value flag with no following value (or followed by another `--flag`)
resolves to `"true"`. A value that legitimately begins with `--` must use the
`--key=--value` form.

## Output streams

One rule, so a consumer never has to guess where to read:

- **Machine/result output → stdout.** Human-readable result lines and every
  `--json` / `SPECD_JSON` response (emitted via `core.PrintJSON`) go to stdout.
- **Diagnostics → stderr.** Gate-failure dumps (`fail  <loc>: <msg>`), the
  trailing `✗ N violation(s)` summaries, and unknown-command errors go to stderr
  via `core.Error` / the `errLine` helper. Inline status glyphs inside a result
  table (e.g. `✗` marking a blocked spec in `specd program`) stay on stdout —
  they are part of the result, not a diagnostic.

Every JSON list field is a **non-nil** slice: arrays the agent parses always
serialize as `[]`, never `null`. Genuinely-optional scalar/object fields may use
`omitempty`.

## Exit code semantics

All commands return one of the `core.Exit*` constants (`internal/core/exit.go`)
— never a bare literal. A single convention applies across every command, so
scripted callers can branch on the code reliably:

| Code | Constant | Meaning |
|---|---|---|
| `0` | `ExitOK` | Success / validation passed |
| `1` | `ExitGate` | Enforcement failure — a validation gate, `check`, or `verify` failed |
| `2` | `ExitUsage` | Usage error / CLI argument error |
| `3` | `ExitNotFound` | Root `.specd/` or spec slug not found |

`ExitGate` (`1`) deliberately covers **both** "check found violations" and
"verify failed": both are enforcement failures, and collapsing them keeps the
contract simple. Distinguish the two by the command you invoked, not the code.

## Config file (`.specd/config.json`)

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3,
  "gates": {
    "traceability": "warn",
    "acceptance": "off",
    "scope": "off",
    "custom": []
  },
  "verify": { "sandbox": "none" },
  "orchestration": {
    "enabled": false,
    "approvalPolicy": "manual",
    "workerMode": "host",
    "maxWorkers": 4,
    "maxRetries": 2,
    "sessionTimeoutMinutes": 120,
    "hostReportedCostLimitUSD": 0,
    "transport": {
      "kind": "file",
      "pollIntervalMillis": 500,
      "messageTTLSeconds": 3600,
      "leaseSeconds": 120,
      "heartbeatSeconds": 30
    },
    "program": { "maxConcurrentSpecs": 2 }
  }
}
```

| Key | Default | Bounds / values | Effect |
|---|---|---|---|
| `defaultVerify` | `npm test` | string | Fallback `verify:` command; set it to your detected test command when bootstrapping steering (see the `specd-steering` skill) |
| `report.format` | `md` | `md`, `html` | Default `specd report` format |
| `report.autoRefreshSeconds` | `0` | integer | HTML report auto-refresh interval (`0` = off) |
| `roles.subagentMode` | `inline` | `inline`, `delegate` | Subagent coordination. Delegation requires host support; specd never spawns provider agents itself |
| `promotionThreshold` | `3` | integer | Recurrences before a learning is suggested for `memory promote` |
| `gates.traceability` | `warn` | `warn`, `error` | Severity of the traceability gate |
| `gates.acceptance` | `off` | `off`, `warn`, `error` | Per-criterion acceptance gate |
| `gates.scope` | `off` | `off`/`*`, `warn`, `error` | Flags verify-time changed files outside a task's `files:` contract |
| `gates.custom` | `[]` | list | External [custom gates](./custom-gates.md) (`{name, command, severity}`) run after the core pipeline |
| `verify.sandbox` | `none` | `none`, `bwrap`, `container` | Isolation backend for `specd verify` (fail-closed if the isolator is absent). See [SECURITY.md](../SECURITY.md) |
| `orchestration.enabled` | `false` | boolean | Opt-in switch for Brain/Pinky workflows. Fresh and legacy projects are disabled by default |
| `orchestration.approvalPolicy` | `manual` | `manual`, `planning`, `session` | Approval authority. `manual` requires human approval at every approval gate. `planning` may advance requirements/design/tasks gates only. `session` may act inside the current orchestration session. No policy can clear high/critical mid-requirement gates; those remain human-only |
| `orchestration.workerMode` | `host` | `host` only | Pinky missions are executed by the external host; specd performs no LLM/provider calls |
| `orchestration.maxWorkers` | `4` | `1..64` | Concurrent worker lease budget |
| `orchestration.maxRetries` | `2` | `0..10` | Retry budget for failed/reclaimed work |
| `orchestration.sessionTimeoutMinutes` | `120` | `1..1440` | Session expiry used by Brain decisions |
| `orchestration.hostReportedCostLimitUSD` | `0` | finite `>=0` | Optional budget over host-reported telemetry. `0` means no cost cap. Values are recorded verbatim and never priced by specd |
| `orchestration.transport.kind` | `file` | `file` only | ACP transport backend |
| `orchestration.transport.pollIntervalMillis` | `500` | `50..60000` | Recommended host polling interval |
| `orchestration.transport.messageTTLSeconds` | `3600` | `60..86400` | ACP message freshness window |
| `orchestration.transport.leaseSeconds` | `120` | `10..3600`, must be `<= messageTTLSeconds` | Worker lease duration |
| `orchestration.transport.heartbeatSeconds` | `30` | `1..1200`, must be `< leaseSeconds` | Host heartbeat cadence |
| `orchestration.program.maxConcurrentSpecs` | `2` | `1..64` | Parent program child-session concurrency |

Backward compatibility: missing `orchestration` fields decode to these defaults.
Malformed, unsupported, secret-shaped, `NaN`/`Inf`, or out-of-range authority
config fails closed to disabled/manual defaults; bounded integers clamp through
one warning path where safe.
