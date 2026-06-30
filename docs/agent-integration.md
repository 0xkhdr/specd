# Agent Integration

Wiring a coding agent (Claude Code, Cursor, Aider, or any command-running wrapper)
to drive the `specd` workflow.

## Contents

1. [The two AGENTS.md files](#the-two-agentsmd-files)
2. [Steering constitution](#steering-constitution)
3. [Role personas](#role-personas)
4. [Subagent coordination modes](#subagent-coordination-modes)
5. [Context engineering](#context-engineering)
6. [Host adapter pattern](#host-adapter-pattern)
7. [Brain/Pinky orchestration](#brainpinky-orchestration)
8. [Cross-spec programs](#cross-spec-programs)

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

Set in `.specd/config.yml` (or legacy `.specd/config.json`) via `roles.subagent_mode` (or legacy `roles.subagentMode`):

### `inline` mode (default)
- The host agent performs the work, swapping persona context inline.
- **Pros:** Simple; works with any agent.
- **Cons:** Context bloat from full chat history.

### `delegate` mode
- The host spawns specialized subagents per role for implementation work when native subagents are available. This is binding policy from `specd fusion policy` / `.specd/config.yml` (or legacy `.specd/config.json`).
- If the host lacks subagent capability, it must say so inline before work (for example: "Delegate mode requested, but this host has no subagents; running role inline under same constraints.").
- Base mode uses `specd next <slug> --dispatch --json` packets; spawn one role-bound subagent per packet and pass its `contextManifest`, files, contract, acceptance, verify, and completion command.
- Orchestrated mode prefers Brain/Pinky missions; the host maps each `dispatch` decision to a Pinky worker and the claim → heartbeat/progress → verify → report/block → release lifecycle.
- **Pros:** Isolated context, reduced token consumption.
- **Cons:** Requires agent-spawning capabilities (Claude Code, etc.).

### Frontier dispatch

`specd next <slug> --dispatch --json` emits one ready-to-run packet per task in the
current frontier — contract, files, acceptance, verify command, the completion
command, and a budgeted `contextManifest`. Pattern for parallel execution:

```bash
# 1. Get dispatch packets
specd next my-feature --dispatch --json

# 2. For each packet, spawn a subagent with its contextManifest + contract.
#    Resolve the role by name via the shared top-level `assets` map
#    (`role/<name>` -> path); read it once and reuse across same-role packets.
# 3. Each subagent:
#      ... implement task ...
#      specd verify my-feature T1
#      specd task my-feature T1 --status complete
# 4. The orchestrator monitors the frontier and dispatches the next wave.
```

Role prompt bytes are emitted **once** per response via `assets`, not inlined per
packet — a 5-task wave on one role no longer repeats the role prompt 5×. Hosts
that cannot resolve asset paths pass `--inline-roles` to restore full-text
`rolePrompt` in every packet (back-compat).

## Fusion bootstrap, policy sentinel, and command discovery

At session start, run `specd fusion bootstrap --json` when available. Cache its
`commands.digest` and `config.digest`. Before acting on one spec, run
`specd fusion policy <slug> --expect-config-digest <cached> --json`; digest
mismatch means rerun bootstrap. The policy output is binding for
`roles.subagentMode`, orchestration capability, verify sandbox, gate severities,
MCP exposure, and Base vs Orchestrated loop choice.

Command syntax comes from schema, not memory: shell hosts call `specd help <command> --json` (or `specd help --json` for registry overview); MCP hosts read `tools/list` input schemas, annotations, and enum fields. If `specd_fusion` is hidden, use `specd_status`, `specd_context`, and schema discovery before acting.

## Context engineering

`specd context <slug>` controls what enters the agent's context window:

```bash
specd context my-feature
```

Output sections:
1. **Phase briefing** — active phase rules (e.g. "You are in PLAN phase. Do not edit code.").
2. **LOAD NOW** — the budgeted context manifest: each item with its `mode`,
   measured `~tokens` (`tokenHint`), and rationale, plus a budget line
   (`est X / budget Y`).
3. **Signals**
   - Blockers: currently blocked tasks + reasons
   - Awaiting approval: mid-req change locks
   - Uncovered requirements: requirements with no task mappings

`specd context <slug> --json` adds an additive `contextManifest` block:

```jsonc
{
  "contextManifest": {
    "version": 1,
    "estimatedTokens": 5400,   // sum of required-item hints
    "budget": 12000,           // effective ceiling (phase/role/file-count derived, host-capped)
    "items": [
      { "order": 1, "kind": "role", "mode": "read-full", "required": true, "tokenHint": 1200, "rationale": "..." }
    ]
  }
}
```

All three surfaces — `specd context` (A), `specd next --dispatch` (B), and the Pinky
mission brief (C) — are produced by one shared engine (`BuildContextManifest`),
so they agree on what to load and never drift.

**Item modes.** `read-full` loads the whole artifact; `run-command` runs the
named command; `reference-if-needed` stays collapsed until needed; `read-targeted`
resolves to a **slice**, not the whole file — the task's row in `tasks.md`, only
the covered requirement lines, the named design section, or a bounded window of
recent `memory.md` entries. `tokenHint` measures the slice in targeted mode, so
the budget reflects real bytes.

**Budget enforcement.** The opt-in `context-budget` gate (off by default) fails
`specd check` when required-item `estimatedTokens` exceeds `budget`, naming the
heaviest items. JSON manifests may include `overBudget`, `budgetActions`, and
per-item `selector` metadata so hosts can load exact slices. Enable the gate in
`.specd/config.yml` (or legacy `.specd/config.json`) under `gates.context_budget` (or legacy `gates.contextBudget`) (`"warn"` or `"error"`).

---

## Execution mode — host decision protocol

Each spec runs in `base` (default) or `orchestrated` mode, recorded in its
`state.json`. The host agent follows a fixed protocol so behavior is identical
across runs:

1. **Default Base.** "create/build/spec X" → author in Base; do **not** start
   Brain/Pinky. In Base the host owns every step.
2. **Explicit opt-in → Orchestrated.** "use Pinky and the Brain", "orchestrate
   this", "run autonomously" → create the spec with `specd new <slug> --orchestrated`,
   then drive with `specd brain run` (or MCP `brain_orchestrate`). Brain/Pinky
   refuse Base specs.
3. **Recommend, don't impose.** After `tasks.md` is approved, consult
   `specd status <slug> --json`; if the status signals orchestration would help,
   surface a one-line suggestion and **wait for the user**. Never switch without
   a yes.
4. **Respect the recorded mode.** Later actions read `spec.executionMode` and
   follow it.

Capability (`orchestration.enabled`) only *permits* orchestration; the spec's
`executionMode` *selects* it — never conflate them.

## Host adapter pattern

Managed host adapters live in `internal/integration`. Add a bespoke adapter only
when specd can safely detect, plan, install, inspect, and verify a host without
owning unrelated user configuration. Production adapter checklist:

1. Implement `HostAdapter` in a new `internal/integration/<host>.go` file.
2. Keep all writes project-scoped unless the user explicitly requested global
   scope; preserve unrelated JSON/TOML keys and record ownership in
   `.specd/integrations.json`.
3. Register the adapter in `DefaultRegistry()` and add unit tests for detect,
   plan, idempotent install, inspect, and verify.
4. Add or verify the matching `specd mcp --config <host>` snippet. This snippet
   fallback is the universal path: every host can still integrate by pasting a
   generated config even when no managed adapter exists.
5. Update `docs/agent-harness-compat.md` and `docs/mcp-guide.md` so the support
   matrix, setup docs, and tests agree.

Never remove the `--config` fallback to force adapter use. Bespoke adapters are
convenience and safety wrappers; snippets remain the compatibility floor for new
or experimental agents.

## Brain/Pinky Orchestration

The **Brain & Pinky model** is `specd`'s native multi-agent orchestration architecture. It transforms the harness from a passive command-line validator into an active, autonomous coordinator.

Unlike traditional orchestration stacks, `specd` separates concerns into two layers:
1. **The Brain (Deterministic Controller):** A state machine that analyzes the current spec state (requirements, design, task DAG progress) and makes decisions (e.g., "dispatch builder for T1", "await human approval for planning"). **The Brain never calls an LLM and never executes unsafe code directly.**
2. **Pinky (Ephemeral Execution Workers):** Independent AI agents spawned by your application or host environment. They receive a structured **Mission** from the Brain, claim a temporary filesystem lease, perform the work (using specialized role personas), run the verification tests, and write back evidence.

---

### Architectural Design & Sequence Flow

The interaction between the Host Orchestrator, the `specd` Brain, and Pinky workers is fully file-backed via the **ACP (Agent Communication Protocol)**.

```mermaid
sequenceDiagram
    participant Host as Host Orchestrator
    participant Brain as specd brain
    participant Pinky as Pinky Worker Agent
    
    Host->>Brain: start / step (reconcile & decide)
    Note over Brain: Check spec gates, DAG wave, active leases
    
    alt Decision: Dispatch Mission
        Brain-->>Host: JSON decision: {"action": "dispatch", "mission": {...}}
        Host->>Pinky: Spawn worker with Mission Brief
        Pinky->>Host: specd pinky claim (acquire lock & lease)
        loop Lease Keepalive
            Pinky->>Host: specd pinky heartbeat (renew lease)
        end
        Pinky->>Pinky: Implement code changes & run tests
        Pinky->>Host: specd pinky progress (optional telemetry)
        Pinky->>Host: specd pinky report (evidence + git head + verify ref)
        Pinky->>Host: Release lease
    else Decision: Awaiting Approval
        Brain-->>Host: JSON decision: {"action": "awaiting-approval"}
        Note over Host: Wait for human input/approval
    else Decision: Wait
        Brain-->>Host: JSON decision: {"action": "wait"}
        Note over Host: Sleep and retry later (tasks busy or blocked)
    end
    
    Host->>Brain: step (reconcile report & advance state)
```

---

### Step-by-Step Developer Walkthrough

To build your own custom orchestration harness or integrate `specd` into your product, follow this integration workflow.

#### Step 1: Enable Orchestration in configuration
Enable orchestration during project initialization:
```bash
specd init --orchestration planning --orchestration-workers 4
```
This writes the configuration blocks into `.specd/config.yml`:
```yaml
orchestration:
  enabled: true
  approval_policy: "planning"
  worker_mode: "host"
  max_workers: 4
  max_retries: 2
  session_timeout_minutes: 120
  host_reported_cost_limit_usd: 10.00
  transport:
    kind: "file"
    poll_interval_millis: 500
    message_ttl_seconds: 3600
    lease_seconds: 120
    heartbeat_seconds: 30
```

#### Step 2: Start a spec session
Initiate an orchestration session for a given spec. This generates a unique `session` UUID:
```bash
specd brain start my-feature --approval-policy planning --max-workers 4 --max-retries 2 --timeout-seconds 7200 --json
```

#### Step 3: Run the Polling & Step Loop
Re-invoke the Brain's stepping handler to reconcile the filesystem database, check worker lease timeouts, and request the next decision.

Brain decision playbook:
- `dispatch`: spawn/assign the role-bound worker, pass the mission, then let the worker claim it.
- `wait`: no runnable action; back off briefly, then call status/step again.
- `awaiting-approval`: stop and ask the human; resume only after authorized approval/directive.
- `escalate`: surface the blocker and stop autonomous work.
- `policy-violation`: stop immediately; report the violated policy/cost/time/approval bound.
- `complete-session`: stop polling; produce final report/summary.

Run the step command:
```bash
specd brain step my-feature --session <session-id> --approval-policy planning --max-workers 4 --max-retries 2 --timeout-seconds 7200 --json
```
The Brain will output a structured JSON response detailing the latest decision:
```json
{
  "action": "dispatch",
  "reason": "task T1 is runnable and has no active lease",
  "mission": {
    "sessionID": "uuid-session-123",
    "workerID": "w-T1-attempt-1",
    "spec": "my-feature",
    "taskID": "T1",
    "role": "builder",
    "deadline": "2026-06-20T17:30:00Z",
    "files": ["src/login.go", "src/login_test.go"],
    "verifyCommand": "go test ./src -run TestLogin"
  }
}
```

#### Step 4: Ephemeral Worker Execution (The Pinky Lifecycle)
When the host sees a `dispatch` decision, it spins up an AI worker (e.g. Claude Code or an internal script) and instructs it to drive the Pinky protocol:

1. **Claim the Mission:**
   The worker starts by claiming the mission to lock in its lease and prevent other workers from taking it:
   ```bash
   specd pinky claim --mission mission.json
   ```
2. **Keep the Lease Alive:**
   While doing creative work, the host must periodically send heartbeats before the `leaseSeconds` deadline expires, otherwise the Brain will reclaim and reassign the task:
   ```bash
   specd pinky heartbeat --session <session-id> --worker <worker-id> --attempt 1
   ```
3. **Emit Progress (Optional):**
   Send telemetry updates back to the orchestrator:
   ```bash
   specd pinky progress --session <session-id> --worker <worker-id> --spec my-feature --task T1 --attempt 1 --percent 50 --message "Implementing OAuth login handler"
   ```
4. **Mid-task Queries & Directives:**
   If the worker encounters a design ambiguity, it can ask a question:
   ```bash
   specd pinky query --session <session-id> --worker <worker-id> --spec my-feature --task T1 --attempt 1 --text "Should we support Google OAuth?"
   ```
   The Brain halts progress at the next safe boundary and asks the host/user for input, which is written back as a directive:
   ```bash
   specd brain directive --session <session-id> --worker <worker-id> --spec my-feature --task T1 --attempt 1 --action continue --reason "Yes, google oauth only" --in-reply-to <message-id>
   ```
5. **Run Verification & Report Completion:**
   Before marking a task complete, the worker **must** run the verification command to get a passing record in the local ledger:
   ```bash
   specd verify my-feature T1
   ```
   It then writes the final terminal report, citing the verification reference:
   ```bash
   specd pinky report --session <session-id> --worker <worker-id> --spec my-feature --task T1 --attempt 1 --verification-ref "verify-rec-456" --summary "OAuth integration finished and tests pass" --git-head "abc123sha" --changed-files "src/login.go,src/login_test.go" --duration-ms 45000 --host-tokens 8500 --host-cost 0.12
   ```

---

### Reference Implementation: Custom Python Orchestrator

Below is a clean, dependency-free reference implementation of a host loop driving the `specd` Brain and spawning Pinky worker agents.

```python
import subprocess
import json
import time
import os

def run_specd(cmd_args):
    """Utility to call specd and return parsed JSON."""
    result = subprocess.run(
        ["specd"] + cmd_args + ["--json"],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        raise RuntimeError(f"specd failed: {result.stderr}")
    return json.loads(result.stdout)

def orchestrate_spec(spec_slug):
    # 1. Start session
    print(f"Starting orchestration session for {spec_slug}...")
    session_data = run_specd([
        "brain", "start", spec_slug,
        "--approval-policy", "planning",
        "--max-workers", "2",
        "--max-retries", "1",
        "--timeout-seconds", "3600"
    ])
    session_id = session_data["sessionID"]
    
    # 2. Main Step-Sense Loop
    while True:
        # Ask Brain what to do next
        step_data = run_specd([
            "brain", "step", spec_slug,
            "--session", session_id,
            "--approval-policy", "planning",
            "--max-workers", "2",
            "--max-retries", "1",
            "--timeout-seconds", "3600"
        ])
        
        action = step_data.get("action")
        print(f"Brain Decision: {action} - {step_data.get('reason')}")
        
        if action == "complete-session":
            print("Orchestration complete!")
            break
            
        elif action == "escalate" or action == "policy-violation":
            print(f"Orchestration blocked! Reason: {step_data.get('reason')}")
            break
            
        elif action == "dispatch":
            # Extract mission details
            mission = step_data["mission"]
            execute_pinky_worker(mission)
            
        elif action == "wait":
            # Sleep briefly and poll again
            time.sleep(2)

def execute_pinky_worker(mission):
    print(f"Spawning Pinky for task {mission['taskID']} ({mission['role']})")
    
    # Save mission to temp file for claim
    with open("temp_mission.json", "w") as f:
        json.dump(mission, f)
        
    try:
        # Worker claims the mission
        run_specd(["pinky", "claim", "--mission", "temp_mission.json"])
        
        # Simulate creative agent work...
        print("Worker is editing code files...")
        time.sleep(5) 
        
        # Run verification tests
        subprocess.run(["specd", "verify", mission["spec"], mission["taskID"]], check=True)
        
        # Report completion back to Pinky ledger
        run_specd([
            "pinky", "report",
            "--session", mission["sessionID"],
            "--worker", mission["workerID"],
            "--spec", mission["spec"],
            "--task", mission["taskID"],
            "--attempt", "1",
            "--verification-ref", f"verify-{mission['taskID']}",
            "--summary", f"Successfully completed {mission['taskID']}",
            "--changed-files", ",".join(mission.get("files", []))
        ])
        print("Worker successfully completed mission and reported evidence.")
    finally:
        if os.path.exists("temp_mission.json"):
            os.remove("temp_mission.json")

if __name__ == "__main__":
    orchestrate_spec("my-feature")
```

---

### The Built-in Driver Loop (`brain run`)

If you do not want to implement a custom orchestration loop, `specd` includes a built-in driver command `specd brain run` that automates step-sense polling and worker spawns:

```bash
specd brain run my-feature --worker-cmd 'python3 my_agent.py'
```

When using `specd brain run`:
- The harness manages step polling, timeouts, lease reclaims, and DAG wave progression.
- `--worker-cmd` is invoked per dispatch. It receives the temporary mission JSON path via the `SPECD_MISSION` environment variable, along with context environment variables (`SPECD_SESSION`, `SPECD_WORKER`, `SPECD_SPEC`, `SPECD_TASK`, `SPECD_ROLE`).
- A hung or runaway worker is automatically terminated when the mission deadline is reached, thanks to process group isolation.

---

### Trust Boundary & Safety Invariants

Orchestrator clients must respect the following security invariants enforced by the harness:

- **Advisory Cost & Time Brakes:** While host-reported cost is untrusted telemetry, the Brain sums `host-cost` across all session reports. If the sum exceeds `hostReportedCostLimitUSD` (when `> 0`), the Brain halts and escalates with a `policy-violation` to prevent runaway LLM costs.
- **Evidence Integrity:** A Pinky report is only accepted by the Brain if it matches a passing `specd verify` run recorded in the spec database. A worker cannot bypass validation by reporting success without running the task's tests.
- **Lease Expiry:** If a worker stops heartbeating or crashes, its lease is automatically reclaimed by the Brain after `leaseSeconds` and rescheduled (up to `maxRetries`), ensuring temporary network failures do not stall development.
- **Cooperative Cancellation:** `specd brain cancel` does not forcibly terminate processes on the host. Instead, it records cancellation intent in the database. Workers must poll `specd pinky inbox` or check command exits to stop themselves cleanly.

---

## Driving Brain/Pinky from MCP Hosts

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
| `brain_pause` / `brain_cancel` | `brain pause`/`cancel` | `session` (required), `program` |
| `brain_resume` | `brain resume` (`--list` or `--session`) | `session` (omit to **list** resumable sessions), `max_age_minutes`, `program` |

`brain_orchestrate` bootstraps a missing spec (using `goal` as its title), then
runs the Brain loop to completion under the planning policy. Supply `worker_cmd`
to execute Pinky dispatches; without one the loop stops at the first dispatch so
the host can run the worker itself. Start-and-monitor is one tool call carrying a
goal + spec — no `--approval-policy`/`--max-workers`/… plumbing.

**Parity-tested tools.** Default MCP discovery exposes survivor-backed tools only:
`specd_init`, `specd_new`, `specd_status`, `specd_context`, `specd_check`,
`specd_approve`, `specd_next`, `specd_verify`, `specd_task`, `specd_report`,
`specd_decision`, `specd_midreq`, `specd_memory`, `specd_waves`, `specd_brain`,
and `specd_pinky`, plus the intent tools below. Meta-hidden commands (`help`,
`version`, `mcp`, `fusion`) are excluded from the default list and appear only
when a host asks for hidden/meta discovery.

**Intent-to-flag mappings.** Composite tools route high-level intents to survivor
flags: `specd_query view=dispatch` maps to `next --dispatch`; `specd_inspect
view=schema` maps to `check --schema`; `specd_inspect view=validate` maps to
`check --schema-only`; `specd_read view=serve` maps to `report --serve`;
`specd_read view=watch` maps to `report --watch`; `specd_read view=history` maps
to `report --history`; `specd_inspect view=diff` maps to `report --diff`; and
`specd_inspect view=program` maps to `status --program`.

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

#### Zero-intervention auto-resume on host start (R5)

A crash or IDE/extension restart should not strand an in-flight orchestration.
The host does not need to remember which spec it was driving — it discovers
resumable sessions and continues the most recent one. The startup recipe:

1. **List.** Run `specd brain resume --list --json` (or MCP `brain_resume` with
   no `session`). It returns, most-recently-updated first, every session whose
   status is `running` or `paused`:
   `[{sessionID, spec, status, updatedAt, pausedSince, lastDecision}]`. Complete,
   failed, and cancelling sessions are excluded; an empty array (`[]`, exit 0)
   means there is nothing to resume.
2. **Bound staleness.** Pass `--max-age-minutes <n>` (config:
   `orchestration.resilience.autoResume.maxAgeMinutes`) to drop sessions whose
   `updatedAt` is older than `n` minutes, so a long-dead session is not woken.
3. **Resume.** For the chosen session run `specd brain run --session <id>` to
   continue the driver loop. Resume is idempotent — the CAS on `state.json`
   guards against double-dispatch, so calling it twice on a `running` session is
   safe and re-dispatches nothing.
4. **Tie-break.** When more than one session is resumable, resume the head of the
   list (most-recently-updated). A host with a UI may instead present the list
   and let the user choose.

Declare the policy in config so hosts behave consistently:

```jsonc
{
  "orchestration": {
    "resilience": {
      "autoResume": { "enabled": true, "onHostStart": true, "maxAgeMinutes": 120 }
    }
  }
}
```

Defaults are `enabled=false`; when the `resilience` block is absent the on-disk
config stays byte-identical to today, so auto-resume is strictly opt-in.

#### Proactive checkpointing before context shedding (R1/R4)

Before a host sheds context (a `/clear`) or on a token-limit warning, a worker
should checkpoint rather than abandon mid-task work. `specd pinky checkpoint
--session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --percent
<0-100> [--notes <text>] [--changed-files <csv>] [--git-head <sha>] [--reason
<text>]` records the progress and hands the task back (the lease is cleared, not
just released, so the same attempt is re-claimable). A host can force a
checkpoint of **every** active worker at once with `specd brain checkpoint <slug>
--session <id> --reason <text>`. On the next step Brain prefers
`resume-from-checkpoint` over a fresh dispatch for any task with a checkpoint
matching its current attempt, and the resuming worker's brief is prepended with a
"resuming — do not restart" header carrying the prior progress, notes, and
touched files. A stale-attempt checkpoint (left by a superseded attempt) is
ignored, and a verified-complete task's checkpoint is deleted so finished work is
never resurrected. All of this is gated on
`orchestration.resilience.checkpointEnabled` (default off, byte-stable config).

## Cross-spec programs

For multi-spec efforts, declare dependencies between whole specs:

```bash
specd new api --depends-on auth     # 'api' waits for 'auth'
edit the spec dependency declaration to remove auth   # remove the dependency
specd status --program                        # view the program-level DAG
specd status --program --json                 # JSON output for orchestrators
```

Edges are stored in `.specd/program.json`. Self-edges and cycles are rejected.

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│  auth   │────►│  api    │────►│  web    │
│ (Wave 1)│     │ (Wave 2)│     │ (Wave 3)│
└─────────┘     └─────────┘     └─────────┘
```

`specd status --program` resolves which whole specs are runnable — the cross-spec
analog of `specd next`.
