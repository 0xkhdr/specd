# Command Reference

This reference lists the optimized command palette only: 16 daily workflow commands and 4 meta-hidden integration commands. It is generated from `specd help --all --json`. specd v0.1.0 has no deprecated commands or aliases.

## Cheat sheet

| Command | One-sentence description |
|---|---|
| `specd init` | Scaffold `.specd/`, managed agent integration, repair, packs, and orchestration defaults. |
| `specd new` | Create a spec and optionally select orchestrated execution with `--orchestrated`. |
| `specd status` | Show one-spec/all-spec progress, recorded mode, and the cross-spec frontier with `--program`. |
| `specd context` | Print the phase-scoped briefing and budgeted LOAD-NOW manifest. |
| `specd check` | Run validation gates or emit/validate the embedded schema with `--schema`/`--schema-only`. |
| `specd review` | Scaffold a structured review report, or extract a deterministic review checklist. |
| `specd approve` | Clear a human approval gate and ratchet the spec to the next phase. |
| `specd next` | Show the next runnable task, all frontier tasks, or dispatch packets with `--dispatch`. |
| `specd verify` | Run a task verification command or record per-criterion proof. |
| `specd task` | Perform the evidence-gated task status transition and telemetry annotation. |
| `specd eval` | Score a spec against its rubric, compile a rubric skeleton with `init`, or report trends. |
| `specd promote` | Promote a prototype spec to a full spec after a passing eval. |
| `specd conductor` | Drive the interactive micro-task conductor session over an append-only ledger. |
| `specd orchestrate` | Inspect and resolve deterministic auto-escalations (status, or resume with an override). |
| `specd submit` | Validate all gates, build the PR summary, and run the configured submit command. |
| `specd deploy` | Run the evidence-gated deploy driver or replay the recorded rollback chain. |
| `specd observe` | Correlate a production error payload into an evidenced mid-requirement. |
| `specd ingest` | Inventory a legacy codebase into an ingestion-flavored spec. |
| `specd report` | Generate snapshots, HTML, metrics, history, diff, live dashboard, or frontier stream views. |
| `specd decision` | Append an architectural decision record to `decisions.md`. |
| `specd midreq` | Log mid-flight requirement feedback with impact and analyzed changes. |
| `specd memory` | Add or promote a durable learning from a spec. |
| `specd waves` | Render the task wave DAG, critical paths, and blockers. |
| `specd brain` | Drive deterministic orchestration sessions and context checkpoints. |
| `specd pinky` | Record worker claims, briefs, heartbeats, progress, queries, reports, blockers, and releases. |
| `specd version` | Print the binary version. |
| `specd help` | Show human help or dump the command registry JSON. |
| `specd mcp` | Run the MCP server or print host configuration snippets. |
| `specd handshake` | Emit hidden host bootstrap and binding-policy diagnostics. |

## Daily workflow commands

| Command | Usage | Flags | Exit codes |
|---|---|---|---|
| `specd init` | `specd init [--agent <auto|all|none|codex|claude-code|cursor|antigravity|vscode>] [--scope project|global] [--yes] [--non-interactive] [--verbose] [--dry-run] [--repair|--refresh|--force] [--orchestration [<policy>]] [--orchestration-workers <n>] [--orchestration-retries <n>] [--orchestration-timeout <minutes>] [--orchestration-cost-limit <usd>] [--orchestration-mode <inline|delegate>] [--orchestration-sandbox <none|bwrap|container>]` | --agent, --scope, --yes, --non-interactive, --verbose, --json, --dry-run, --repair, --refresh, --force, --list-packs, --pack, --sha256, --orchestration, --orchestration-workers, --orchestration-retries, --orchestration-timeout, --orchestration-cost-limit, --orchestration-mode, --orchestration-sandbox | 0 Success, 1 Initialization or pack operation failed, 2 Usage error |
| `specd new` | `specd new <slug> [--title "..."] [--orchestrated] [--prototype]` | --title, --orchestrated, --prototype | 0 Success, 1 Orchestration requested without project capability, 2 Usage error, 3 .specd/ not found or spec already exists |
| `specd status` | `specd status [<slug>] [--all] [--program] [--json]` | --all, --program, --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd context` | `specd context <slug> [--hud] [--json]` | --hud, --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd check` | `specd check <slug> [--schema-only] [--security] [--json] | specd check --schema` | --schema-only, --schema, --security, --json | 0 Success, 1 Validation failed, 2 Usage error, 3 Spec not found |
| `specd review` | `specd review <slug> [checklist] [--force] [--json]` | --force, --json | 0 Success, 1 Gate failure, 2 Usage error, 3 Spec not found |
| `specd approve` | `specd approve <slug> [--json]` | --json | 0 Success, 1 Gate validation failed, 2 Usage error, 3 Spec not found |
| `specd next` | `specd next <slug> [--all] [--dispatch] [--json]` | --all, --dispatch, --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd verify` | `specd verify <slug> <id>  |  specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence "..."` | --criterion, --status, --evidence, --revert-on-fail, --sandbox | 0 Success, 1 Verification failed, 2 Usage error, 3 Spec or task not found |
| `specd task` | `specd task <slug> <id> --status <s> [--evidence "..."] [--reason "..."] [--force]` | --status, --evidence, --reason, --force, --unverified, --tokens, --cost | 0 Success, 1 Gate verification failed, 2 Usage error, 3 Spec or task not found |
| `specd eval` | `specd eval <slug> [init|trend] [--suite <name>] [--force] [--json]` | --suite, --force, --json | 0 Success, 1 Score below minScore, 2 Usage error, 3 Spec or rubric not found |
| `specd promote` | `specd promote <slug> --evidence "..." [--suite <name>] [--json]` | --evidence, --suite, --json | 0 Success, 1 Not a prototype, eval failed, or missing evidence, 2 Usage error, 3 Spec not found |
| `specd conductor` | `specd conductor <slug> <start|step|accept|reject|stop|replay|switch|status> [micro] [--reason "..."] [--json]` | --reason, --json | 0 Success, 1 Gate failure, 2 Usage error, 3 Spec not found |
| `specd orchestrate` | `specd orchestrate <slug> <status\|resume> [--override] [--json]` | --override, --json | 0 Success, 1 Gate failure, 2 Usage error, 3 Spec not found |
| `specd submit` | `specd submit <slug> [--waves w1,w2] [--dry-run] [--json]` | --waves, --dry-run, --json | 0 Success, 1 Gate violation or submit failure, 2 Usage error, 3 Spec not found |
| `specd deploy` | `specd deploy <slug> --env <env> [--dry-run] [--json]  \|  specd deploy rollback <slug> --env <env> [--json]` | --env, --dry-run, --json | 0 Success, 1 Precondition/gate/step failure, 2 Usage error, 3 Spec/config not found or rollback halted |
| `specd observe` | `specd observe correlate <payload.json> [--spec <slug>] [--json]  \|  specd observe --listen [--spec <slug>]` | --listen, --spec, --json | 0 Success, 1 Invalid payload or no correlation, 2 Usage error, 3 Payload/root not found |
| `specd ingest` | `specd ingest new <slug> --path <dir> [--include-ignored] [--json]` | --path, --include-ignored, --title, --json | 0 Success, 1 Invalid path or existing spec, 2 Usage error, 3 Path/root not found |
| `specd report` | `specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary] [--conductor] [--serve|--watch|--history|--diff]` | --format, --out, --pr-summary, --conductor, --serve, --watch, --history, --diff | 0 Success, 2 Usage error, 3 Spec not found |
| `specd decision` | `specd decision <slug> "<text>" [--supersedes <id>]` | --supersedes | 0 Success, 2 Usage error, 3 Spec not found |
| `specd midreq` | `specd midreq <slug> "<input>" --impact <low|medium|high|critical>` | --impact, --interpretation, --changes | 0 Success, 2 Usage error, 3 Spec not found |
| `specd memory` | `specd memory <slug> add|promote [flags]` | --key, --pattern, --body, --source, --criticality, --related, --force | 0 Success, 2 Usage error, 3 Spec not found |
| `specd waves` | `specd waves <slug> [--json]` | --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd brain` | `specd brain <start|status|step|pause|resume|cancel|checkpoint> ... [--program] [--auto-step|--verbose|--ledger|--directive|--compact]` | --program, --auto-step, --verbose, --ledger, --compact, --directive, --session, --approval-policy, --max-workers, --max-retries, --timeout-seconds, --cost-limit, --worker-cmd, --bootstrap, --max-steps, --title, --worker, --spec, --task, --attempt, --action, --reason, --in-reply-to, --json | 0 Success, 1 Gate or validation failure, 2 Usage error, 3 Workspace or session not found |
| `specd pinky` | `specd pinky <claim|status|update|report|block|release> ...` | --mission, --session, --worker, --spec, --task, --attempt, --artifact, --percent, --message, --reason, --text, --verification-ref, --summary, --changed-files, --git-head, --duration-ms, --host-tokens, --host-cost, --json | 0 Success, 1 Gate or validation failure, 2 Usage error, 3 Workspace or session not found |

## Meta-hidden commands

Meta-hidden commands exist for hosts, integrations, and diagnostics; they are excluded from the default daily palette and from default MCP tool discovery unless explicitly requested.

| Command | Usage | Flags | Exit codes |
|---|---|---|---|
| `specd version` | `specd version [--json]` | --json | 0 Success |
| `specd help` | `specd help [command]` | --all, --json | 0 Success, 2 Usage error (unknown command) |
| `specd mcp` | `specd mcp [--root <path>] [--spec <slug>] [--config <host>]` | --root, --spec, --config | 0 Success (stream closed or config printed), 1 Server error, 2 Usage error |
| `specd handshake` | `specd handshake bootstrap [--include-schema] [--json] | specd handshake policy [<slug>] [--expect-config-digest <sha256>] [--json]` | --include-schema, --expect-config-digest, --json | 0 Success, 1 Policy violation or digest mismatch, 2 Usage error, 3 .specd/ or spec not found |
| `specd program` | `specd program [--json] | specd program <link\|unlink> <spec> --on <dep> | specd program schedule [<name> --interval <seconds> --command <cmd> [--sandbox <backend>] | <name> --remove] | specd program tick [--now <unix>] [--json]` | --on, --interval, --command, --sandbox, --remove, --now, --json | 0 Success, 1 Gate/link failure or scheduled command failure, 2 Usage error, 3 Spec not found |

## Merged behavior homes

- Diagnostics and safe repair live under `specd init --repair`.
- Execution mode is selected by `specd new --orchestrated` and observed through `specd status`.
- Dispatch packets live under `specd next --dispatch`.
- Schema emission and validation live under `specd check --schema` and `specd check --schema-only`.
- HTML serving, frontier streaming, history replay, and spec diffs live under `specd report` flags.
- Cross-spec program inspection lives under `specd status --program`; dependency authoring belongs in spec creation/planning.
- Binary install/reinstall uses `scripts/install.sh`; removal is manual (delete the installed binary — there is no uninstall script).

## Exit code semantics

| Code | Meaning |
|---|---|
| `0` | Success / validation passed |
| `1` | Gate, validation, verify, config, or policy failure |
| `2` | Usage error |
| `3` | `.specd/` root, spec, task, workspace, or session not found |

## Environment variables and config

Machine-readable command, flag, and exit metadata is available from `specd help --all --json`. Config precedence remains embedded defaults → global config → project config → `SPECD_*` overrides. Human-authored config is YAML v2 by default; legacy JSON is still read but has no built-in conversion path (convert manually).

Observability env vars:

| Variable | Effect |
|---|---|
| `SPECD_LOG=info|debug` | Emits structured duration metrics to stderr; stdout/`SPECD_JSON=1` stay unchanged. |
| `SPECD_METRICS_ENDPOINT=<addr>` | Opt-in Prometheus text endpoint for duration samples, e.g. `127.0.0.1:9099`; unset starts no listener. |
| `SPECD_TRACE_FILE=<path>` | With `go build -tags specd_trace`, writes Chrome trace JSON spans to this path. |

specd v0.1.0 has no deprecated commands or aliases — the command surface above is complete.
