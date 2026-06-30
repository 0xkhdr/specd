# specd Repository Analysis — Coding Agent Command Architecture & Optimization Plan

> **Prepared for:** Coding agent integration planning  
> **Source:** https://github.com/0xkhdr/specd (spec-driven coding harness CLI)  
> **Date:** 2026-06-30

---

## 1. Executive Summary

`specd` is an **agent-agnostic, spec-driven coding harness CLI** written in Go. It enforces a structured spec workflow (requirements → design → tasks → evidence-gated execution) through deterministic validation gates, DAG-based task execution, and a rigid planning ratchet. The project is designed to be driven entirely via shell commands or MCP (Model Context Protocol) tools — no API, no plugin, no LLM calls in the core.

This document analyzes the repo's command surface, MCP tool exposure, and shell command patterns to produce a **minimal, non-redundant, well-structured slash command architecture** for a coding agent that will plan and implement optimizations.

---

## 2. Domain & Use Case Analysis

### 2.1 What specd Does

| Aspect | Description |
|--------|-------------|
| **Core Mission** | Shift process enforcement from the LLM's non-deterministic context to a strict, local, tool-gated pipeline |
| **Philosophy** | *"The agent reasons. The harness enforces."* |
| **Output** | Versioned Markdown specs on disk + machine `state.json` |
| **Zero Dependencies** | Go stdlib only; single binary; zero LLM calls |
| **Agent Interface** | Shell commands (`specd <cmd>`) or MCP tools (`specd_brain`, `specd_pinky`, etc.) |

### 2.2 The Five-Phase Lifecycle

```
requirements.md ──► design.md ──► tasks.md ──► Code/Tests ──► verify ──► complete
     │                  │              │            │            │           │
  analyze            plan           plan         execute      verify      reflect
     │                  │              │            │            │           │
  specd check      specd check    specd check   specd next   specd verify  specd approve
  specd approve    specd approve  specd approve specd task   specd task   specd report
```

### 2.3 Key Artifacts (All under `.specd/`)

| Artifact | Purpose | Agent Action |
|----------|---------|--------------|
| `steering/{reasoning,workflow,product,tech,structure}.md` | Durable constitution | Read at session start |
| `steering/memory.md` | Promoted learnings | Read phase-scoped |
| `roles/{investigator,builder,reviewer,verifier,brain,pinky}.md` | Role personas | Adopt per task |
| `skills/<name>/SKILL.md` | Progressive skill disclosure | Read before entering a phase |
| `specs/<slug>/{requirements,design,tasks,decisions,memory,mid-requirements}.md` | Spec artifacts | Author & mutate via CLI only |
| `specs/<slug>/state.json` | Machine truth for status | **Never hand-edit** |
| `config.yml` | Project policy | Human-authored, agent-read |
| `program.json` | Cross-spec dependency DAG | Link/unlink via CLI |

### 2.4 Execution Modes

| Mode | Description | Trigger |
|------|-------------|---------|
| **Base** (default) | Agent drives every step manually | `specd new <slug>` |
| **Orchestrated** | Brain/Pinky multi-agent layer drives | `specd new <slug> --orchestrated` or `specd mode --set orchestrated` |

---

## 3. Command Surface Deep Dive

### 3.1 Command Taxonomy (Exit Codes: 0=ok, 1=gate/validation, 2=usage, 3=not found)

#### Lifecycle Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd init [--agent <name>]` | Scaffold `.specd/`, install MCP, verify integration | Once per project |
| `specd doctor [--fix]` | Diagnose & repair scaffold/MCP health | When something is broken |
| `specd new <slug> [--title "..."] [--orchestrated]` | Create a new spec with stubs | Per feature/bugfix |
| `specd mode <slug> [--set base|orchestrated] [--recommend]` | Get/set execution mode | When user asks for orchestration |
| `specd approve <slug>` | Advance phase gate (human approval) | After `specd check` passes |

#### Execution Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd next <slug> [--all] [--json]` | Get next runnable task (or full frontier) | During execute phase |
| `specd dispatch <slug> [--inline-roles] [--json]` | Emit subagent packets for frontier | For parallel subagent execution |
| `specd verify <slug> <task> [--sandbox none|bwrap|container] [--revert-on-fail]` | Run task's verify command & record proof | After implementing a task |
| `specd task <slug> <task> --status <status> [--evidence "..."] [--unverified]` | Evidence-gated status flip | After verify passes |

#### Inspection Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd status [<slug>] [--json]` | Progress board for one spec or all specs | Orient / check where we are |
| `specd check <slug> [--json]` | Run validation gates (7 core + opt-in) | Before claiming phase complete |
| `specd waves <slug> [--json]` | Wave graph, critical paths, blockers | Debug DAG issues |
| `specd context <slug> [--json]` | Phase briefing + budgeted LOAD NOW manifest | At start of every session/phase |
| `specd report <slug> [--format md|html] [--pr-summary]` | Generate snapshot report | At completion |
| `specd serve <slug> [--addr]` | Read-only HTTP dashboard | For live monitoring |
| `specd watch [--once] [--spec <slug>] [--sse] [--webhook]` | Stream frontier events | For automation hooks |

#### Record Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd decision <slug> "<text>" [--supersedes <id>]` | Append ADR to `decisions.md` | When making architectural decisions |
| `specd midreq <slug> "<text>" --impact <level>` | Log mid-flight requirement update | When requirements change mid-execution |
| `specd memory <slug> add --key "..." --pattern "..." --body "..." --source <task> --criticality <level>` | Record a learning | When discovering patterns |
| `specd memory <slug> promote --key "..."` | Promote learning to global steering | When a learning recurs enough |

#### Program Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd program [status] [--json]` | Cross-spec DAG + runnable frontier | Multi-spec efforts |
| `specd program link <slug> --on <dep>` | Declare inter-spec dependency | When specs depend on each other |
| `specd program unlink <slug> --on <dep>` | Remove inter-spec dependency | When dependencies change |

#### Orchestration Commands (Brain/Pinky)
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd brain start <slug> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--session <id>]` | Start orchestration session | Begin autonomous execution |
| `specd brain run <slug> [--worker-cmd <cmd>] [--max-steps <n>]` | Built-in driver loop (polls & dispatches) | Full autonomous run |
| `specd brain step <slug> --session <id> --approval-policy <policy> ...` | Advance session by one bounded decision | Custom orchestration loops |
| `specd brain status --session <id> [--json]` | Read persisted session state | Check orchestration health |
| `specd brain why --session <id> [--json]` | Explain latest Brain decision | Debug orchestration behavior |
| `specd brain directive --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --action <action> --reason "..."` | Send directive to Pinky worker | Reply to worker queries |
| `specd brain pause|resume|cancel --session <id>` | Cooperative session control | Pause/resume/cancel orchestration |
| `specd brain compact <slug> --session <id> [--reason "..."]` | Context compaction checkpoint | Before `/clear` or token limit |
| `specd brain clear <slug> --session <id>` | Alias for compact with manual-clear reason | Quick context shed |
| `specd brain ledger --session <id> [--json]` | Read context ledger | Audit token budget history |
| `specd brain resume --list [--max-age-minutes <n>] [--json]` | Discover resumable sessions | After host restart |
| `specd pinky claim --mission <file> [--json]` | Worker claims a mission | Worker start |
| `specd pinky brief --session <id> --worker <id> --spec <slug> (--task <task> | --artifact <art>) [--json]` | Render worker brief | Worker context setup |
| `specd pinky heartbeat --session <id> --worker <id> --attempt <n>` | Renew worker lease | During long tasks |
| `specd pinky progress --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --percent <n> --message "..."` | Report progress telemetry | Optional mid-task updates |
| `specd pinky query --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --text "..."` | Ask bounded mid-task question | When blocked mid-task |
| `specd pinky inbox --session <id> --worker <id> [--json]` | Read Brain directives | Poll for instructions |
| `specd pinky report --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --verification-ref <ref> --summary "..."` | Terminal worker evidence | Task completion |
| `specd pinky block --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --reason "..."` | Record worker blocker | When task is blocked |
| `specd pinky release --session <id> --worker <id> --attempt <n>` | Release claim idempotently | Clean worker exit |
| `specd pinky checkpoint --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --percent <n> [--reason "..."]` | Checkpoint before context shed | Before `/clear` |

#### Meta Commands
| Command | Purpose | When to Use |
|---------|---------|-------------|
| `specd update [--force]` | Self-update to latest release | Maintenance |
| `specd uninstall [--force] [--dry-run]` | Remove binary & PATH entries | Cleanup |
| `specd mcp [--root <path>] [--http [<addr>]] [--config <host>]` | Run MCP server (stdio or HTTP/SSE) | MCP client integration |
| `specd version` | Show version | Debugging |
| `specd help [command] [--json]` | Show help / dump JSON command registry | Discovery |

### 3.2 MCP Tool Exposure

`specd mcp` exposes **every CLI command as an MCP tool**. The naming convention is:
- Raw passthrough: `specd_brain`, `specd_pinky` (receive subcommands in `args`)
- Intent-level (recommended): `brain_orchestrate`, `brain_status`, `brain_approve`, `brain_pause`, `brain_cancel`, `brain_resume`

**Key insight:** The MCP layer is a thin wrapper over the CLI. There is **no separate API surface**. Every MCP tool maps 1:1 to a CLI subcommand. This means:
- Shell commands and MCP tools are **functionally identical**
- The agent only needs to know the CLI commands; MCP auto-exposes them
- No need to maintain separate "MCP tool" documentation — the CLI is the canonical interface

---

## 4. Critical Path Analysis — What the Coding Agent Actually Needs

### 4.1 The Agent's Job

The coding agent's role is to:
1. **Plan** specs (requirements → design → tasks)
2. **Implement** tasks (write code, run tests)
3. **Verify** completion (evidence-gated)
4. **Report** status (to user and harness)

The agent does **NOT** need to:
- Install/uninstall specd (user does this)
- Run `specd update` (user maintenance)
- Manage cross-spec programs (advanced, optional)
- Run `specd serve`/`watch` (monitoring, not agent work)
- Run `specd replay`/`diff` (audit, not planning)
- Run `specd schema`/`validate` (CI/CI gates, not agent work)

### 4.2 The Minimal Required Command Set

Based on the workflow, the agent needs **only these commands** for 95% of work:

| Phase | Command | Purpose | Frequency |
|-------|---------|---------|-----------|
| **Session Start** | `specd context <slug>` | Load phase-scoped context | Every session |
| | `specd status <slug>` | Orient — where are we? | Every session |
| **Planning** | `specd new <slug> --title "..."` | Create spec | Per feature |
| | `specd check <slug>` | Validate current phase | After authoring each artifact |
| | `specd approve <slug>` | Advance phase gate | After check passes |
| **Execution** | `specd next <slug>` | Get runnable task | Per task |
| | `specd verify <slug> <task>` | Run verification | After implementing |
| | `specd task <slug> <task> --status complete --evidence "..."` | Mark done | After verify passes |
| **Orchestration** (opt-in) | `specd brain start <slug> ...` | Start autonomous session | Once per spec |
| | `specd brain step <slug> --session <id> ...` | Advance one decision | Per step in custom loop |
| | `specd brain status --session <id>` | Check session health | Periodic |
| **Debugging** | `specd waves <slug>` | Debug DAG/blockers | When stuck |
| | `specd check <slug>` | Re-validate | When something breaks |

### 4.3 Path Redundancy Analysis

**All paths are relative to the `.specd/` root**, which is found by walking up from CWD. The agent never needs absolute paths. Key paths:

| Path | Usage | Redundancy? |
|------|-------|-------------|
| `.specd/` | Root directory | Found automatically — never pass explicitly |
| `.specd/config.yml` | Config | Read-only; mutated via `specd init`/`doctor` only |
| `.specd/steering/*.md` | Constitution | Loaded by `specd context` — never read directly |
| `.specd/roles/*.md` | Personas | Loaded by `specd dispatch`/`pinky brief` — never read directly |
| `.specd/skills/*/SKILL.md` | Skills | Read before entering phase — loaded by agent, not CLI |
| `.specd/specs/<slug>/` | Spec directory | Referenced only by `<slug>` in CLI commands |
| `.specd/specs/<slug>/requirements.md` | Requirements | Author via editor; validate via `specd check` |
| `.specd/specs/<slug>/design.md` | Design | Author via editor; validate via `specd check` |
| `.specd/specs/<slug>/tasks.md` | Tasks | Author via editor; validate via `specd check` |
| `.specd/specs/<slug>/state.json` | Machine truth | **Never touch** — only CLI mutates |
| `.specd/specs/<slug>/decisions.md` | ADRs | Append via `specd decision` |
| `.specd/specs/<slug>/memory.md` | Learnings | Append via `specd memory` |
| `.specd/specs/<slug>/mid-requirements.md` | Feedback | Append via `specd midreq` |

**Conclusion:** There is **zero path redundancy**. Every path has exactly one purpose and one access method. The agent should:
- **Never** construct paths manually — use CLI commands with `<slug>` only
- **Never** read/write `state.json` directly
- **Never** touch `.specd/` root files directly (except skills, which are read via editor)

---

## 5. Recommended Slash Command Architecture

### 5.1 Design Principles

1. **Minimal surface:** Only commands the agent needs for planning + execution
2. **No redundancy:** Each command has one clear purpose
3. **No path duplication:** All paths derived from `<slug>`; no absolute paths
4. **Phase-aligned:** Commands grouped by lifecycle phase
5. **Evidence-gated:** Every completion requires `--evidence`
6. **JSON-first:** All commands support `--json` for structured parsing

### 5.2 Slash Command Specification

```markdown
# /specd-init
Initialize specd scaffolding in the current project.
Usage: /specd-init [--agent <name>]
Note: Typically run once by the user, not the coding agent.

# /specd-new
Create a new spec for a feature or bugfix.
Usage: /specd-new <slug> --title "<title>" [--orchestrated]
Example: /specd-new auth-jwt --title "Implement JWT Authentication"

# /specd-status
Check the current status of a spec or all specs.
Usage: /specd-status [<slug>] [--json]
Example: /specd-status auth-jwt --json

# /specd-context
Load phase-scoped context for the current spec.
Usage: /specd-context <slug> [--json]
Example: /specd-context auth-jwt --json

# /specd-check
Run validation gates on the current spec phase.
Usage: /specd-check <slug> [--json]
Example: /specd-check auth-jwt --json

# /specd-approve
Advance the spec to the next phase (human gate).
Usage: /specd-approve <slug> [--json]
Example: /specd-approve auth-jwt

# /specd-next
Get the next runnable task from the frontier.
Usage: /specd-next <slug> [--all] [--json]
Example: /specd-next auth-jwt --json

# /specd-verify
Run the task's verification command and record proof.
Usage: /specd-verify <slug> <task> [--sandbox none|bwrap|container] [--revert-on-fail]
Example: /specd-verify auth-jwt T1

# /specd-task
Evidence-gated status flip for a task.
Usage: /specd-task <slug> <task> --status <pending|running|complete|blocked> [--evidence "..."] [--unverified] [--reason "..."]
Example: /specd-task auth-jwt T1 --status complete --evidence "commit abc123; go test PASS"

# /specd-waves
Display the wave graph, critical paths, and blockers.
Usage: /specd-waves <slug> [--json]
Example: /specd-waves auth-jwt --json

# /specd-report
Generate a snapshot report for the spec.
Usage: /specd-report <slug> [--format md|html] [--pr-summary]
Example: /specd-report auth-jwt --format md

# /specd-decision
Record an architectural decision.
Usage: /specd-decision <slug> "<text>" [--supersedes <id>]
Example: /specd-decision auth-jwt "Use RS256 for JWT signing" --supersedes ADR-001

# /specd-midreq
Log a mid-flight requirement update.
Usage: /specd-midreq <slug> "<text>" --impact <low|medium|high|critical> [--interpretation "..."] [--changes "..."]
Example: /specd-midreq auth-jwt "Add refresh token support" --impact high

# /specd-memory
Record or promote a learning.
Usage: /specd-memory <slug> add --key "..." --pattern "..." --body "..." --source <task> --criticality <important|critical>
       /specd-memory <slug> promote --key "..."
Example: /specd-memory auth-jwt add --key "jwt-expiry" --pattern "token expiration" --body "Always set exp claim" --source T1 --criticality important

# /specd-brain-start
Start an orchestration session for autonomous execution.
Usage: /specd-brain-start <slug> --approval-policy <manual|planning|session> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--session <id>]
Example: /specd-brain-start auth-jwt --approval-policy planning --max-workers 4 --max-retries 2 --timeout-seconds 7200

# /specd-brain-step
Advance an orchestration session by one bounded decision.
Usage: /specd-brain-step <slug> --session <id> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n>
Example: /specd-brain-step auth-jwt --session abc-123 --approval-policy planning --max-workers 4 --max-retries 2 --timeout-seconds 7200

# /specd-brain-status
Check the status of an orchestration session.
Usage: /specd-brain-status --session <id> [--json]
Example: /specd-brain-status --session abc-123 --json

# /specd-brain-pause|resume|cancel
Control an orchestration session cooperatively.
Usage: /specd-brain-pause --session <id>
       /specd-brain-resume --session <id>
       /specd-brain-cancel --session <id>

# /specd-pinky-claim
Claim a Brain-issued mission (worker start).
Usage: /specd-pinky-claim --mission <file> [--json]

# /specd-pinky-report
Report terminal worker evidence.
Usage: /specd-pinky-report --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --verification-ref <ref> --summary "..." [--changed-files "..."] [--git-head "..."] [--duration-ms <n>] [--host-tokens <n>] [--host-cost <n>]

# /specd-pinky-block
Record a worker blocker.
Usage: /specd-pinky-block --session <id> --worker <id> --spec <slug> --task <task> --attempt <n> --reason "..."

# /specd-pinky-release
Release a worker claim idempotently.
Usage: /specd-pinky-release --session <id> --worker <id> --attempt <n>
```

### 5.3 Commands NOT Included (Intentionally Omitted)

| Omitted Command | Reason |
|-----------------|--------|
| `specd init` | User runs once; agent never needs it |
| `specd doctor` | User maintenance; agent never needs it |
| `specd update`/`uninstall` | User maintenance |
| `specd mode` | User sets mode; agent reads from context |
| `specd dispatch` | Covered by `specd-brain-step` in orchestrated mode; base mode uses `specd-next` |
| `specd serve`/`watch` | Monitoring, not agent work |
| `specd replay`/`diff` | Audit, not planning/execution |
| `specd schema`/`validate` | CI gates, not agent work |
| `specd program link/unlink` | Advanced multi-spec; agent handles one spec at a time |
| `specd brain run` | Covered by `specd-brain-start` + `specd-brain-step` loop |
| `specd brain why`/`ledger`/`compact`/`clear` | Debugging/advanced; not core workflow |
| `specd brain directive` | Advanced; host handles replies |
| `specd pinky brief`/`heartbeat`/`progress`/`query`/`inbox`/`checkpoint` | Worker lifecycle; host wrapper handles these |
| `specd version`/`help` | Meta; agent discovers via docs |

---

## 6. Action Plan for the Coding Agent

### 6.1 Phase 1: Bootstrap (One-time)

1. **User runs:** `specd init --agent auto` (or equivalent for their agent)
2. **Agent reads:** `.specd/skills/specd-steering/SKILL.md`
3. **Agent authors:** `.specd/steering/{product,structure,tech}.md` based on repo inspection
4. **Agent sets:** `defaults.verify_command` in `.specd/config.yml`

### 6.2 Phase 2: Spec Planning (Per Feature)

```
/specd-new <slug> --title "..."
→ Author requirements.md (EARS format)
→ /specd-check <slug>
→ /specd-approve <slug>   # requirements → design

→ Author design.md (7 mandatory H2 sections)
→ /specd-check <slug>
→ /specd-approve <slug>   # design → tasks

→ Author tasks.md (wave DAG with 7 keys per task)
→ /specd-check <slug>
→ /specd-approve <slug>   # tasks → executing
```

### 6.3 Phase 3: Execution (Per Task)

**Base Mode (default):**
```
/specd-next <slug> --json
→ Implement task
→ /specd-verify <slug> <task>
→ /specd-task <slug> <task> --status complete --evidence "..."
→ Repeat until frontier empty
→ /specd-approve <slug>   # executing → complete
→ /specd-report <slug>
```

**Orchestrated Mode (opt-in):**
```
/specd-brain-start <slug> --approval-policy planning --max-workers 4 --max-retries 2 --timeout-seconds 7200 --json
→ Parse decision JSON
→ If dispatch: spawn worker with mission
→ If wait: sleep & retry
→ If awaiting-approval: prompt user
→ If complete-session: done
→ /specd-brain-status --session <id> --json
```

### 6.4 Phase 4: Reflection (Post-completion)

```
/specd-report <slug> --pr-summary
/specd-memory <slug> add --key "..." --pattern "..." --body "..." --source <task> --criticality important
# Promote if recurrence threshold met
```

---

## 7. Recommendations for the Coding Agent

### 7.1 Context Management

- **Always run `specd context <slug> --json` at session start.** This gives you the exact files to load and their token budgets. Do not guess.
- **Read skills progressively:** `specd-foundations` → `specd-steering` → `specd-requirements` → `specd-design` → `specd-tasks` → `specd-execute`. Never read ahead.
- **Use `--json` on every command** for deterministic parsing. Never parse human-readable output.

### 7.2 Evidence Discipline

- **Never mark a task complete without `--evidence`.** The harness enforces this; trying to bypass it wastes tokens.
- **Evidence format:** `"commit <sha>; <test command> <result>; <key change summary>"`
- **For read-only roles** (investigator, reviewer): use `--unverified --evidence "<proof>"`

### 7.3 Path Discipline

- **Never construct `.specd/specs/<slug>/...` paths manually.** Use `<slug>` in CLI commands only.
- **Never hand-edit `state.json`.** Use `specd task`, `specd verify`, `specd approve` only.
- **Never touch `.specd/` root files directly** (except skills, which are read-only).

### 7.4 Error Handling

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| `0` | Success | Continue |
| `1` | Gate/validation failure | Run `specd check <slug> --json` to diagnose; fix and retry |
| `2` | Usage error | Check command syntax with `specd help <command> --json` |
| `3` | Not found | Run `specd status` to verify slug; run `specd init` if `.specd/` missing |

### 7.5 Orchestration Safety

- **Base mode is the default.** Only switch to orchestrated if the user explicitly asks.
- **Always set `--approval-policy manual` for sensitive work.** Never auto-approve high/critical mid-requirements.
- **Use `specd brain resume --list --json` after host restarts** to discover stranded sessions.
- **Checkpoint before `/clear`:** `specd brain compact <slug> --session <id> --reason "pre-clear"`

### 7.6 Token Optimization

- **Prefix all shell commands with `rtk`** (Rust Token Killer) when available: `rtk git status`, `rtk ls`, `rtk read`, etc. This reduces context usage by 60-90%.
- **Use `specd dispatch <slug> --json`** to get frontier packets with shared role assets (not inlined per packet) when spawning subagents.

### 7.7 Concurrency & Locking

- **All state mutations are atomic and versioned.** If you hit a CAS/revision conflict, re-read state and retry.
- **Advisory locks per spec:** Only one agent should mutate a spec at a time. The lock timeout is 5s default (`SPECD_LOCK_TIMEOUT_MS`).

---

## 8. Summary: The Minimal Agent Interface

For a coding agent planning and implementing optimizations, the **entire required surface** is:

```
# Planning
/specd-new <slug> --title "..."
/specd-check <slug> [--json]
/specd-approve <slug>

# Execution
/specd-next <slug> [--json]
/specd-verify <slug> <task>
/specd-task <slug> <task> --status complete --evidence "..."

# Orientation
/specd-status <slug> [--json]
/specd-context <slug> [--json]
/specd-waves <slug> [--json]

# Orchestration (opt-in)
/specd-brain-start <slug> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n>
/specd-brain-step <slug> --session <id> ...
/specd-brain-status --session <id> [--json]

# Records
/specd-decision <slug> "..."
/specd-midreq <slug> "..." --impact <level>
/specd-memory <slug> add --key "..." --pattern "..." --body "..." --source <task> --criticality <level>

# Reporting
/specd-report <slug> [--format md|html]
```

**Total: 15 slash commands** covering the full lifecycle from spec creation to completion, with zero redundancy and zero path duplication.

---

*End of Analysis*
