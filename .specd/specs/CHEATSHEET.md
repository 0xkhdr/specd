# specd Command Cheat Sheet — Optimized Palette

> 20 surviving commands (16 daily + 4 meta). Memorize the 16; the 4 meta are host/integration only.
> Exit codes: `0` ok · `1` gate/validation · `2` usage · `3` not found. All commands accept `--json`.

## Daily Workflow (16)

| Command | One-liner |
|---------|-----------|
| `init` | Scaffold `.specd/` and integration; `--repair` self-heals, `--migrate` upgrades legacy config. |
| `new` | Create a spec; `--orchestrated` selects Brain/Pinky mode. |
| `status` | Orient — progress board for one spec or all; `--program` shows cross-spec DAG. |
| `context` | Load phase-scoped briefing + budgeted LOAD-NOW manifest. |
| `check` | Run validation gates; `--schema`/`--schema-only` for schema checks. |
| `approve` | Advance the phase gate after `check` passes. |
| `next` | Get the next runnable task; `--all` shows frontier, `--dispatch` emits subagent packets. |
| `verify` | Run a task's verify command and record proof; sole home of `--sandbox`/`--revert-on-fail`. |
| `task` | Evidence-gated status flip (`--status`, `--evidence`). |
| `report` | Snapshot report; `--format`, `--serve`, `--watch`, `--history`, `--diff`. |
| `decision` | Append an ADR to `decisions.md`. |
| `midreq` | Log a mid-flight requirement change with `--impact`. |
| `memory` | Record or `promote` a learning. |
| `waves` | Show wave graph, critical paths, and blockers. |
| `brain` | Orchestration session: `start`/`step`/`status`/`checkpoint`/`pause`/`resume`/`cancel`. |
| `pinky` | Worker lifecycle: `claim`/`status`/`update`/`report`/`block`/`release`. |

## Meta — host/integration only (4)

| Command | One-liner |
|---------|-----------|
| `version` | Print the binary version. |
| `help` | List commands (`--all` includes meta); dump registry as JSON. |
| `mcp` | Run the MCP server (stdio or HTTP/SSE). |
| `fusion` | Emit host bootstrap / binding-policy oracle for agent integration. |

## Killed — see migration appendix (13)
`doctor`→`init --repair` · `migrate`→`init --migrate` · `mode`→`new --orchestrated` / `status` · `dispatch`→`next --dispatch` · `validate`→`check --schema-only` · `schema`→`check --schema` · `serve`→`report --serve` · `watch`→`report --watch` · `replay`→`report --history` · `diff`→`report --diff` · `program`→`status --program` · `update`/`uninstall`→install script.
