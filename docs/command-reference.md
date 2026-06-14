# Command Reference

Every `specd` command, its flags, and exit codes — plus environment variables and
the `config.json` schema. This mirrors the embedded registry; run
`specd help --json` for the machine-readable form.

## Contents

- [Lifecycle commands](#lifecycle-commands)
- [Execution commands](#execution-commands)
- [Inspection commands](#inspection-commands)
- [Record commands](#record-commands)
- [Program commands](#program-commands)
- [Meta commands](#meta-commands)
- [Environment variables](#environment-variables)
- [Exit code semantics](#exit-code-semantics)
- [Config file](#config-file-specdconfigjson)

---

## Lifecycle commands

| Command | Description | Exit codes |
|---|---|---|
| `specd init [--force]` | Scaffold `.specd/` + `AGENTS.md` in the project root | `0` ok, `2` usage, `3` `.specd/` exists (no `--force`) |
| `specd boot [--force] [--dry-run] [--json] [--output-dir <dir>]` | Auto-detect tech stack (AI-free detectors) → `boot.json`, managed block in `steering/tech.md`, `config.defaultVerify` | `0` ok, `1` write error, `2` usage, `3` no `.specd/` |
| `specd enrich [plan] [--json]` | Brief the agent: which steering sections to author + what evidence to read | `0` ok, `2` usage, `3` no `boot.json` |
| `specd enrich apply --target <product\|structure\|tech> [--content-file <path>]` | Accept agent-authored markdown into the managed `SPECD ENRICH` block (stdin if no `--content-file`) | `0` ok, `1` gate/write fail, `2` usage, `3` not found |
| `specd enrich status [--json]` | Report enrichment freshness (also: `specd check --enrich`) | `0` ok, `1` drift, `3` not found |
| `specd new <slug> [--title "..."]` | Create a spec with six artifact stubs | `0` ok, `2` usage, `3` no `.specd/` or spec exists |
| `specd approve <slug> [--json]` | Clear approval gate / advance to the next phase (human gate) | `0` ok, `1` gates failed, `2` usage, `3` not found |

## Execution commands

| Command | Description | Exit codes |
|---|---|---|
| `specd next <slug> [--all] [--json]` | Get the next runnable task, or the entire frontier with `--all` | `0` ok, `2` usage, `3` not found |
| `specd dispatch <slug> [--json]` | Emit ready-to-run subagent packets for the frontier | `0` ok, `2` usage, `3` not found |
| `specd verify <slug> <id>` | Run the task's `verify:` command and record proof (exit code, output tail, duration, git HEAD) | `0` pass, `1` fail, `2` usage, `3` not found |
| `specd verify <slug> --criterion <r>.<n> --status pass\|fail --evidence "..."` | Record a per-acceptance-criterion proof | `0` ok, `1` fail, `2` usage, `3` not found |
| `specd task <slug> <id> --status <s> [--evidence "..."] [--reason "..."] [--unverified] [--force]` | Evidence-gated status flip; dual-writes `tasks.md` + `state.json` | `0` ok, `1` rejected, `2` usage, `3` not found |

> On verify timeout, the **task's recorded** exit code is `124` and the record is
> marked `verified: false`. The `specd verify` process itself still exits `1`.

## Inspection commands

| Command | Description | Exit codes |
|---|---|---|
| `specd status [<slug>] [--json]` | Progress board for a spec, or list all specs | `0` ok, `2` usage, `3` not found |
| `specd check <slug> [--json]` | Run all 7 validation gates on a spec | `0` pass, `1` fail, `2` usage, `3` not found |
| `specd check --boot` | Run the repo-global boot-freshness gate | `0` pass, `1` drift, `2` usage, `3` no `boot.json` |
| `specd check --enrich` | Run the repo-global enrich-freshness gate | `0` pass, `1` missing/drift, `2` usage, `3` not found |
| `specd waves <slug> [--json]` | Wave graph, critical paths, blockers | `0` ok, `2` usage, `3` not found |
| `specd context <slug> [--json]` | Phase briefing + load list + signals | `0` ok, `2` usage, `3` not found |
| `specd report <slug> [--format md\|html] [--out <path>]` | Generate a snapshot report | `0` ok, `2` usage, `3` not found |

See [Validation Gates](./validation-gates.md) for what each `check` gate enforces.

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
| `specd version` | Show version | `0` ok |
| `specd help [command] [--json]` | Show help / dump the JSON command registry | `0` ok, `2` unknown command |

---

## Environment variables

| Variable | Default | Effect |
|---|---|---|
| `SPECD_JSON` | `0` | Emit structured JSON for all commands (same as the `--json` flag) |
| `SPECD_LOCK_TIMEOUT_MS` | `5000` | Max wait for a spec advisory lock |
| `SPECD_LOCK_STALE_MS` | `30000` | Age at which an orphaned `.lock` file is auto-reclaimed |
| `SPECD_VERIFY_TIMEOUT_MS` | `600000` | Per-run timeout for `specd verify` |
| `NO_COLOR` | — | Disable ANSI colors |

> The `--json` flag is equivalent to `SPECD_JSON=1` and is the recommended way to
> request machine-readable output.

## Exit code semantics

| Code | Meaning |
|---|---|
| `0` | Success / validation passed |
| `1` | Validation gate failure / check failed |
| `2` | Usage error / CLI argument error |
| `3` | Root `.specd/` or spec slug not found |

## Config file (`.specd/config.json`)

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3,
  "gates": { "traceability": "warn", "acceptance": "off" }
}
```

| Key | Default | Effect |
|---|---|---|
| `defaultVerify` | `npm test` | Fallback `verify:` command; `specd boot` overwrites it with the detected stack's test command |
| `report.format` | `md` | Default `specd report` format (`md` or `html`) |
| `report.autoRefreshSeconds` | `0` | HTML report auto-refresh interval (`0` = off) |
| `roles.subagentMode` | `inline` | `inline` or `delegate` subagent coordination |
| `promotionThreshold` | `3` | Recurrences before a learning is suggested for `memory promote` |
| `gates.traceability` | `warn` | `warn` or `error` — severity of the traceability gate |
| `gates.acceptance` | `off` | `off`, `warn`, or `error` — per-criterion acceptance gate |
