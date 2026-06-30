# Command Reference

This reference lists the optimized command palette only: 16 daily workflow commands and 4 meta-hidden integration commands. It is generated from `specd help --all --json`; deprecated commands appear only in the migration appendix.

## Cheat sheet

| Command | One-sentence description |
|---|---|
| `specd init` | Scaffold `.specd/`, managed agent integration, repair, migration, packs, and orchestration defaults. |
| `specd new` | Create a spec and optionally select orchestrated execution with `--orchestrated`. |
| `specd status` | Show one-spec/all-spec progress, recorded mode, and the cross-spec frontier with `--program`. |
| `specd context` | Print the phase-scoped briefing and budgeted LOAD-NOW manifest. |
| `specd check` | Run validation gates or emit/validate the embedded schema with `--schema`/`--schema-only`. |
| `specd approve` | Clear a human approval gate and ratchet the spec to the next phase. |
| `specd next` | Show the next runnable task, all frontier tasks, or dispatch packets with `--dispatch`. |
| `specd verify` | Run a task verification command or record per-criterion proof. |
| `specd task` | Perform the evidence-gated task status transition and telemetry annotation. |
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
| `specd fusion` | Emit hidden host bootstrap and binding-policy diagnostics. |

## Daily workflow commands

| Command | Usage | Flags | Exit codes |
|---|---|---|---|
| `specd init` | `specd init [--agent <auto|all|none|codex|claude-code|cursor|antigravity|vscode>] [--scope project|global] [--yes] [--non-interactive] [--verbose] [--dry-run] [--repair|--refresh|--force] [--migrate] [--orchestration [<policy>]] [--orchestration-workers <n>] [--orchestration-retries <n>] [--orchestration-timeout <minutes>] [--orchestration-cost-limit <usd>] [--orchestration-mode <inline|delegate>] [--orchestration-sandbox <none|bwrap|container>]` | --agent, --scope, --yes, --non-interactive, --verbose, --json, --dry-run, --repair, --migrate, --refresh, --force, --list-packs, --pack, --sha256, --orchestration, --orchestration-workers, --orchestration-retries, --orchestration-timeout, --orchestration-cost-limit, --orchestration-mode, --orchestration-sandbox | 0 Success, 1 Initialization or pack operation failed, 2 Usage error |
| `specd new` | `specd new <slug> [--title "..."] [--orchestrated]` | --title, --orchestrated | 0 Success, 1 Orchestration requested without project capability, 2 Usage error, 3 .specd/ not found or spec already exists |
| `specd status` | `specd status [<slug>] [--all] [--program] [--json]` | --all, --program, --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd context` | `specd context <slug> [--json]` | --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd check` | `specd check <slug> [--schema-only] [--json] | specd check --schema` | --schema-only, --schema, --json | 0 Success, 1 Validation failed, 2 Usage error, 3 Spec not found |
| `specd approve` | `specd approve <slug> [--json]` | --json | 0 Success, 1 Gate validation failed, 2 Usage error, 3 Spec not found |
| `specd next` | `specd next <slug> [--all] [--dispatch] [--json]` | --all, --dispatch, --json | 0 Success, 2 Usage error, 3 Spec not found |
| `specd verify` | `specd verify <slug> <id>  |  specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence "..."` | --criterion, --status, --evidence, --revert-on-fail, --sandbox | 0 Success, 1 Verification failed, 2 Usage error, 3 Spec or task not found |
| `specd task` | `specd task <slug> <id> --status <s> [--evidence "..."] [--reason "..."] [--force]` | --status, --evidence, --reason, --force, --unverified, --tokens, --cost | 0 Success, 1 Gate verification failed, 2 Usage error, 3 Spec or task not found |
| `specd report` | `specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary] [--serve|--watch|--history|--diff]` | --format, --out, --pr-summary, --serve, --watch, --history, --diff | 0 Success, 2 Usage error, 3 Spec not found |
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
| `specd fusion` | `specd fusion bootstrap [--include-schema] [--json] | specd fusion policy [<slug>] [--expect-config-digest <sha256>] [--json]` | --include-schema, --expect-config-digest, --json | 0 Success, 1 Policy violation or digest mismatch, 2 Usage error, 3 .specd/ or spec not found |

## Merged behavior homes

- Diagnostics and safe repair live under `specd init --repair`.
- Legacy config conversion lives under `specd init --migrate`.
- Execution mode is selected by `specd new --orchestrated` and observed through `specd status`.
- Dispatch packets live under `specd next --dispatch`.
- Schema emission and validation live under `specd check --schema` and `specd check --schema-only`.
- HTML serving, frontier streaming, history replay, and spec diffs live under `specd report` flags.
- Cross-spec program inspection lives under `specd status --program`; dependency authoring belongs in spec creation/planning.
- Binary lifecycle operations use `scripts/install.sh` and `scripts/uninstall.sh`.

## Exit code semantics

| Code | Meaning |
|---|---|
| `0` | Success / validation passed |
| `1` | Gate, validation, verify, config, or policy failure |
| `2` | Usage error |
| `3` | `.specd/` root, spec, task, workspace, or session not found |

## Environment variables and config

Machine-readable command, flag, and exit metadata is available from `specd help --all --json`. Config precedence remains embedded defaults → global config → project config → `SPECD_*` overrides. Human-authored config is YAML v2 by default; legacy JSON is still read and can be upgraded with the optimized init migration flag.

<!-- docs-lint: migration-appendix begin -->
## Migration appendix

| Old command | New home | Removed in | Notes |
|---|---|---|---|
| `doctor` | `init --repair` | `palette-merge` | `Health checks fold into init repair/refresh diagnostics.` |
| `migrate` | `init --migrate` | `palette-merge` | `Legacy config conversion is an init-time maintenance operation.` |
| `mode` | `new --orchestrated / status` | `palette-merge` | `Mode choice is made at creation; status reports recorded mode.` |
| `dispatch` | `next --dispatch` | `palette-merge` | `Frontier packet generation belongs to next.` |
| `validate` | `check --schema-only` | `palette-merge` | `Structural validation is a check gate variant.` |
| `schema` | `check --schema` | `palette-merge` | `Schema emission is a check metadata variant.` |
| `serve` | `report --serve` | `palette-merge` | `Live dashboard is a report view.` |
| `watch` | `report --watch` | `palette-merge` | `Frontier stream is a report view.` |
| `replay` | `report --history` | `palette-merge` | `Timeline replay is report history.` |
| `diff` | `report --diff` | `palette-merge` | `Spec artifact diffs are report snapshots.` |
| `program` | `status --program` | `palette-merge` | `Cross-spec frontier is a status view.` |
| `update` | `scripts/install.sh --force` | `palette-merge` | `Self-update moved to the install script.` |
| `uninstall` | `scripts/uninstall.sh` | `palette-merge` | `Removal is script-only.` |

<!-- docs-lint: migration-appendix end -->
