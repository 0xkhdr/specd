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
| `specd init [--force] [--list-packs] [--pack <name\|url> [--sha256 <hex>]]` | Scaffold `.specd/` (steering, roles, the skill pack) + `AGENTS.md`. `--list-packs` lists embedded packs; `--pack` applies a [spec pack](./spec-packs.md) transactionally (remote URL requires a pinned `--sha256`, fail-closed) | `0` ok, `1` pack resolve/apply failed, `2` usage, `3` `.specd/` exists (no `--force`) |
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
| `specd schema [--version <v>]` | Emit the embedded, versioned [open spec format](./spec-format.md) JSON Schema to stdout. Needs no `.specd/` root | `0` ok, `1` unknown version, `2` usage |
| `specd validate <slug> --schema [--version <v>] [--json]` | Validate a spec's `state.json` against the embedded JSON Schema (structural/format conformance, independent of the semantic gates). Read-only | `0` conformant, `1` violations, `2` usage, `3` not found |
| `specd replay <slug>` | Reconstruct a deterministic, read-only event timeline (task start/finish/verify/block + acceptance records) from on-disk audit data. Text, or typed JSON array under `SPECD_JSON` | `0` ok, `2` usage, `3` not found |
| `specd diff <slug> --from <ref> [--to <ref>]` | Show how a spec's artifacts changed between two git refs — a read-only `git diff --name-status` scoped to the spec dir. `--to` defaults to the working tree | `0` ok, `1` git diff failed, `2` usage, `3` not found |

## Record commands

| Command | Description |
|---|---|
| `specd decision <slug> "..." [--supersedes <id>]` | Append an ADR to `decisions.md` |
| `specd midreq <slug> "..." --impact <low\|medium\|high\|critical> [--interpretation "..."] [--changes "..."]` | Log a mid-flight requirement update |
| `specd memory <slug> add --key "..." --pattern "..." --body "..." --source "T1" --criticality important [--related "..."] [--force]` | Record a learning |
| `specd memory <slug> promote --key "..."` | Promote a learning to global steering |

## Program commands

| Command | Description |
|---|---|
| `specd program [status] [--json]` | Render the spec-level DAG + runnable frontier (JSON for orchestrators) |
| `specd program link <spec> --on <dep>` | Declare an inter-spec dependency |
| `specd program unlink <spec> --on <dep>` | Remove an inter-spec dependency |

Edges are stored in `.specd/program.json`. Self-edges and cycles are rejected.

## Meta commands

| Command | Description | Exit codes |
|---|---|---|
| `specd update [--force]` | Self-update to the latest release | `0` ok, `1` failed, `2` usage |
| `specd uninstall [--force] [--dry-run] [--json]` | Remove the install dir, the `~/.local/bin/specd` link, and the `# specd` PATH lines from shell rc files (backed up). A bare invocation only previews; `--force` actually removes. Project-local `.specd/` dirs are always preserved | `0` ok, `1` failed, `2` usage |
| `specd mcp [--root <path>]` | Run the [MCP](https://modelcontextprotocol.io) stdio server (JSON-RPC 2.0), exposing every read-safe and state-mutating command as an MCP tool. Stdlib-only, no network, no LLM calls | `0` stream closed, `1` server error, `2` usage |
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
| `SPECD_VERIFY_SHELL` | `sh -c` | Shell used to run each `verify:` line and custom-gate command |
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
  "verify": { "sandbox": "none" }
}
```

| Key | Default | Effect |
|---|---|---|
| `defaultVerify` | `npm test` | Fallback `verify:` command; set it to your detected test command when bootstrapping steering (see the `specd-steering` skill) |
| `report.format` | `md` | Default `specd report` format (`md` or `html`) |
| `report.autoRefreshSeconds` | `0` | HTML report auto-refresh interval (`0` = off) |
| `roles.subagentMode` | `inline` | `inline` or `delegate` subagent coordination |
| `promotionThreshold` | `3` | Recurrences before a learning is suggested for `memory promote` |
| `gates.traceability` | `warn` | `warn` or `error` — severity of the traceability gate |
| `gates.acceptance` | `off` | `off`, `warn`, or `error` — per-criterion acceptance gate |
| `gates.scope` | `off` | `off`/`*` = no-op, else `warn`/`error` — flags verify-time changed files outside a task's `files:` contract |
| `gates.custom` | `[]` | List of external [custom gates](./custom-gates.md) (`{name, command, severity}`) run after the core pipeline |
| `verify.sandbox` | `none` | Isolation backend for `specd verify`: `none`, `bwrap`, or `container` (fail-closed if the isolator is absent). See [SECURITY.md](../SECURITY.md) |
