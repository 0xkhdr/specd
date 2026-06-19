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
  leases, progress, blockers, reports, and cancellation directives over ACP
  files; it never spawns provider agents or kills host processes.
- **Host telemetry** (`host-tokens`, `host-cost`, `duration-ms`) is evidence
  supplied by the host and stored verbatim. specd does not compute token usage,
  price model calls, or trust telemetry as completion proof.
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

MCP hosts use the generated `specd_brain` and `specd_pinky` tools. The `args`
array is the normal CLI subcommand list; flags are ordinary tool arguments.
There are no MCP-only orchestration commands.

Typical host loop:

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
