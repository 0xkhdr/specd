# Agent Integration

Wiring a coding agent (Claude Code, Cursor, Aider, or any command-running wrapper)
to drive the `specd` workflow.

## Contents

1. [The two AGENTS.md files](#the-two-agentsmd-files)
2. [Steering constitution](#steering-constitution)
3. [Role personas](#role-personas)
4. [Subagent coordination modes](#subagent-coordination-modes)
5. [Context engineering](#context-engineering)
6. [Brain/Pinky orchestration](#brainpinky-orchestration)
7. [Cross-spec programs](#cross-spec-programs)

---

## The two AGENTS.md files

| File | Location | Purpose |
|---|---|---|
| **Root AGENTS.md** | `0xkhdr/specd` repo root | Guide for agents **developing specd itself** |
| **Template AGENTS.md** | `internal/core/embed_templates/AGENTS.md` | Guide written to **user repos** by `specd init` |

## Steering constitution

Durable rules under `.specd/steering/` that outlive individual chat sessions:

| File | Purpose |
|---|---|
| `reasoning.md` | Six-phase thinking discipline + backpropagation protocol |
| `workflow.md` | Spec lifecycle transitions + validation gates |
| `product.md` | Domain rules, target audience, business constraints |
| `tech.md` | Approved stack, languages, dependencies, testing frameworks |
| `structure.md` | File organization, directory structures, module boundaries |
| `memory.md` | Promoted learnings across specs |

`product.md`, `structure.md`, and `tech.md` are authored by the agent itself,
guided by the `specd-steering` skill (see the [User Guide](./user-guide.md#bootstrap-project-context-recommended)).

## Role personas

Prompts under `.specd/roles/`. Each task's `role:` key binds it to one persona.

| Role | Permissions | Responsibilities |
|---|---|---|
| 🔍 `investigator` | Read-only | Explore code, trace paths, find integration points. Reports exact file/line refs. |
| 🛠️ `builder` | Write | Implement the task contract. Modifies designated files + tests. Runs verify. |
| 🧪 `verifier` | Read-only | Runs tests independently. Captures output as evidence. |
| 🛡️ `reviewer` | Read-only | Audits git diffs. Logs issues with severity tags + exact locations. |

## Subagent coordination modes

Set in `.specd/config.json` via `roles.subagentMode`:

### `inline` mode (default)
- The host agent performs the work, swapping persona context inline.
- **Pros:** Simple; works with any agent.
- **Cons:** Context bloat from full chat history.

### `delegate` mode
- The host spawns specialized subagents per role.
- **Pros:** Isolated context, reduced token consumption.
- **Cons:** Requires agent-spawning capabilities (Claude Code, etc.).

### Frontier dispatch

`specd dispatch <slug> --json` emits one ready-to-run packet per task in the
current frontier — role prompt, contract, files, acceptance, verify command, and
the completion command. Pattern for parallel execution:

```bash
# 1. Get dispatch packets
specd dispatch my-feature --json

# 2. For each packet, spawn a subagent with its rolePrompt + contract.
# 3. Each subagent:
#      ... implement task ...
#      specd verify my-feature T1
#      specd task my-feature T1 --status complete
# 4. The orchestrator monitors the frontier and dispatches the next wave.
```

## Context engineering

`specd context <slug>` controls what enters the agent's context window:

```bash
specd context my-feature
```

Output sections:
1. **Phase briefing** — active phase rules (e.g. "You are in PLAN phase. Do not edit code.").
2. **Load list** — the minimal file list for the context window.
3. **Signals**
   - Blockers: currently blocked tasks + reasons
   - Awaiting approval: mid-req change locks
   - Uncovered requirements: requirements with no task mappings

---

## Brain/Pinky orchestration

Brain/Pinky is an optional deterministic orchestration layer. It is disabled by
default in `.specd/config.json`:

```json
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
```

Trust boundary:
- **Brain** senses specd state and records one bounded decision per step. It is a
  deterministic controller, not an LLM.
- **Pinky** is a worker contract executed by the host. specd writes missions,
  leases, progress, bounded queries, directives, blockers, reports, and
  cancellation directives over ACP files; it never spawns provider agents or
  kills host processes.
- **Host telemetry** (`host-tokens`, `host-cost`, `duration-ms`) is evidence
  supplied by the host and stored verbatim. specd does not compute token usage,
  price model calls, or trust telemetry as completion proof.
- **Advisory cost / time brakes.** Although host-reported cost stays untrusted,
  the brain still acts on it: it sums `host-cost` across a session's reports and,
  when the total reaches `hostReportedCostLimitUSD` (`> 0`), the next `step`
  escalates with `policy-violation` instead of dispatching more work. A session
  that outlives its `sessionTimeoutMinutes` wall-clock deadline escalates the
  same way. Both are advisory halts (the input is untrusted) but they force a
  terminal decision rather than relying on lease expiry. `0` disables the cost
  brake.
- **Completion proof** still requires a passing `specd verify` record (or the
  existing manual proof path for read-only roles) and `--evidence`. Pinky reports
  are accepted only when they bind to the matching verification record and task
  scope.

Approval policy:
- `manual` (default): all approvals remain human-driven.
- `planning`: Brain may advance normal requirements/design/tasks planning gates
  when gates pass, but cannot clear mid-requirement gates.
- `session`: Brain may act inside the active orchestration session, subject to
  the same gates, locks, retries, and evidence rules.
- High/critical mid-requirement gates are always human-only; automation cannot
  clear them under any policy.

Backward compatibility: older `.specd/config.json` files with no
`orchestration` block load as disabled/manual/host/file defaults. Unsupported
modes, secret-shaped fields, invalid costs, or unsafe timing relationships fail
closed to the disabled defaults.

### Driving Brain/Pinky from MCP hosts

MCP hosts have two layers. Prefer the **intent-level tools** for orchestration;
drop to the raw passthrough only when you need a flag the intent tools do not
surface.

**Intent-level tools (recommended).** Six semantic tools wrap the deterministic
primitives with sane policy defaults — one clear affordance per intent, no flag
plumbing. They add no new core authority; each translates to a `specd_brain`/
`approve` invocation the passthrough could already produce.

| Tool | Wraps | Key arguments |
|---|---|---|
| `brain_orchestrate` | `brain run` | `spec` (required), `goal`, `worker_cmd`, `approval_policy` (default `planning`), `max_steps`, `session`, `no_bootstrap` |
| `brain_status` | `brain status` | `session` (required), `program` |
| `brain_approve` | `approve` | `spec` (required) |
| `brain_pause` / `brain_resume` / `brain_cancel` | `brain pause`/`resume`/`cancel` | `session` (required), `program` |

`brain_orchestrate` bootstraps a missing spec (using `goal` as its title), then
runs the Brain loop to completion under the planning policy. Supply `worker_cmd`
to execute Pinky dispatches; without one the loop stops at the first dispatch so
the host can run the worker itself. Start-and-monitor is one tool call carrying a
goal + spec — no `--approval-policy`/`--max-workers`/… plumbing.

**Raw passthrough (power users).** The generated `specd_brain` and `specd_pinky`
tools remain. The `args` array is the normal CLI subcommand list; flags are
ordinary tool arguments.

Typical raw host loop:

1. Call `specd_brain` with `args: ["start", "<slug>"]` and explicit
   `approval-policy`, `max-workers`, `max-retries`, and `timeout-seconds`.
2. Read the bounded JSON result. If Brain dispatches a mission, the host starts
   or assigns its own worker; specd does not spawn one.
3. The worker calls `specd_pinky` with `args: ["claim"]`, then heartbeat,
   progress/block, and terminal report calls while holding the lease.
4. The worker runs the task's `specd verify` command through the host shell.
5. The worker reports completion with `specd_pinky args: ["report"]` and the
   matching `verification-ref`; Brain later steps and reconciles the event log.
6. The host repeats bounded `specd_brain status` / `step` calls until the
   session completes, pauses, escalates, or waits for human approval.

### Beginning-to-delivery: authoring frontier, `brain run`, and worker briefs

Brain drives both **planning** and **execution**, not just execution:

- **Authoring frontier.** When a spec is in `requirements`, `design`, or `tasks`
  and the phase artifact is missing or fails its gate, Brain emits a
  `dispatch-authoring` decision (a mission to author that artifact, verified by
  `specd check`). Under `planning`/`session` approval policy it dispatches and,
  once the gate passes, emits `advance-phase` to ratchet to the next status —
  the same gate `specd approve` enforces. Under `manual` it requests human
  approval instead. Execution tasks never run before the `tasks → executing`
  gate clears.

- **Reference driver loop.** `specd brain run <slug>` ties steps to worker
  spawns: it steps, hands each dispatched mission to a host worker, blocks until
  the worker reports (the dispatch→spawn contract), and stops on a terminal
  outcome (`complete | escalated | awaiting-approval | worker-stop | max-steps |
  stalled`). It defaults to the `planning` policy and is re-runnable (it resumes
  an active session). `--worker-cmd '<shell>'` receives each mission via
  `SPECD_MISSION` (a temp JSON path) plus `SPECD_SESSION/WORKER/SPEC/TASK/ROLE`
  env; with no `--worker-cmd` the loop stops at the first dispatch so an operator
  can wire a worker by hand.

- **Pre-spec preflight.** `specd brain run --bootstrap` creates a missing spec
  (`specd new`) before driving. A missing `.specd` workspace or steering fails
  closed with the remedy command (`specd init` / `specd init --repair`).

- **Worker briefs and agent templates.** `specd pinky brief --session <id>
  --worker <id> --spec <slug> (--task <id> | --artifact <name>) [--json]` renders
  a paste-ready, context-engineered worker brief (or, with `--json`, the
  claimable mission). Each mission carries a deterministic `contextManifest`:
  required role + Pinky skill + one phase-scoped skill + `specd context` + scoped
  files, optional source artifacts, and a soft token ceiling so different hosts
  package the same minimal sufficient context. `specd init` installs Claude Code
  sub-agent definitions at `.claude/agents/pinky-{builder,investigator,reviewer,verifier}.md`, each a thin
  shell that follows the manifest and runs claim → execute → verify → report.

The core stays deterministic: the driver loop and briefs are orchestration glue;
all authoring/execution happens inside the host worker. The final
`verifying → complete` transition still requires the acceptance-evidence gate and
is never auto-cleared.

Mid-task clarification is explicit: a leased worker may send `specd pinky query --text "..."`
and keep working only up to the next safe waiting point. Brain or the host replies with
`specd brain directive ... --in-reply-to <query-message-id>`, and workers poll
`specd pinky inbox` for directives. This avoids full blocker escalation for bounded questions
while preserving the file-backed ACP audit trail.

Cancellation is cooperative: `specd_brain cancel` records intent, and a later
step emits cancellation directives for active leases. Hosts must deliver that
signal to their workers and stop them safely; specd never kills provider or
editor processes.

Recovery is file-backed: after a host or MCP restart, call `specd_brain status`
for the persisted session and continue with `step`. Expired leases are reclaimed
by Brain within policy, and duplicate Pinky terminal reports are idempotent.

## Cross-spec programs

For multi-spec efforts, declare dependencies between whole specs:

```bash
specd program link api --on auth     # 'api' waits for 'auth'
specd program unlink api --on auth   # remove the dependency
specd program                        # view the program-level DAG
specd program --json                 # JSON output for orchestrators
```

Edges are stored in `.specd/program.json`. Self-edges and cycles are rejected.

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│  auth   │────►│  api    │────►│  web    │
│ (Wave 1)│     │ (Wave 2)│     │ (Wave 3)│
└─────────┘     └─────────┘     └─────────┘
```

`specd program status` resolves which whole specs are runnable — the cross-spec
analog of `specd next`.
