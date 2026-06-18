# Pinky & The Brain — Feature Design Document
## Autonomous Dual-Agent Orchestration for specd

> **Status:** Draft — Awaiting approval before integration into the specd project.
>
> **Authors:** Collaborative design session
> **Date:** 2026-06-18
> **Target:** specd v0.x (post-design approval)

---

## Table of Contents

1. [Vision & Elevator Pitch](#1-vision--elevator-pitch)
2. [Design Principles](#2-design-principles)
3. [System Architecture](#3-system-architecture)
4. [The Brain — Orchestrator Agent](#4-the-brain--orchestrator-agent)
5. [Pinky — Executor Agent](#5-pinky--executor-agent)
6. [Agent Communication Protocol (ACP)](#6-agent-communication-protocol-acp)
7. [MCP Integration](#7-mcp-integration)
8. [State Machine & Autonomy](#8-state-machine--autonomy)
9. [Multi-Spec Program Orchestration](#9-multi-spec-program-orchestration)
10. [Implementation Plan](#10-implementation-plan)
11. [Security & Sandboxing](#11-security--sandboxing)
12. [Configuration Schema](#12-configuration-schema)
13. [Open Questions & Risks](#13-open-questions--risks)
14. [Appendix A: ACP Message Schema](#appendix-a-acp-message-schema)
15. [Appendix B: Brain Decision Tree](#appendix-b-brain-decision-tree)
16. [Appendix C: Pinky Role Prompt Template](#appendix-c-pinky-role-prompt-template)

---

## 1. Vision & Elevator Pitch

**Current Problem:** specd is a powerful harness, but it requires a human or external agent to manually drive the workflow — reading state, deciding next steps, calling commands, interpreting results. This creates friction and prevents true autonomous execution.

**Pinky & The Brain Solution:** Introduce two native agent personas into the specd ecosystem that transform specd from a *passive harness* into an *active orchestrator*:

- **The Brain** — An autonomous orchestrator that understands the complete specd state machine, monitors all specs and programs, decides what needs to happen next, and dispatches work.
- **Pinky** — A translucent executor that accepts any role, gathers context, performs tasks, and reports back with evidence.

> **"The Brain decides. Pinky executes. specd enforces."**

**User Experience:**
```
User: "Use the brain to build me a spec for JWT authentication"
Brain: [autonomously runs init → steering → new spec → requirements → design → tasks → execution → verify → complete]
User: "Add OAuth2 support to the auth spec"
Brain: [detects mid-requirement, updates spec, re-plans waves, dispatches Pinky]
```

---

## 2. Design Principles

| # | Principle | Rationale |
|---|---|---|
| 1 | **Harness Still Enforces** | Brain and Pinky are *agents within* specd, not replacements. All state changes go through `specd` CLI gates. |
| 2 | **Brain is Stateless, specd is State** | Brain has no persistent state of its own. It reads from `state.json`, `program.json`, and artifact files. |
| 3 | **Pinky is Ephemeral** | Pinky instances are created per-task or per-wave and destroyed after completion. No long-lived Pinky state. |
| 4 | **ACP is the Nervous System** | All Brain↔Pinky communication goes through the Agent Communication Protocol — structured, versioned, auditable. |
| 5 | **Fail-Closed by Default** | Any ambiguity, blocker, or exception causes Brain to pause and escalate to human. No silent failures. |
| 6 | **Observable at All Times** | Every Brain decision, Pinky action, and ACP message is logged and replayable via `specd replay`. |
| 7 | **MCP-Native** | Brain is exposed as MCP tools, making it drivable from Claude Desktop, Cursor, VS Code, or any MCP client. |

---

## 3. System Architecture

### 3.1 High-Level Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER / MCP CLIENT                               │
│         "Use the brain to build me a spec for JWT authentication"           │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         specd MCP SERVER (stdio/HTTP/SSE)                    │
│  ┌─────────────────────────────────────────────────────────────────────┐     │
│  │  EXPOSED TOOLS:                                                     │     │
│  │  • brain_orchestrate(spec, goal, constraints)                       │     │
│  │  • brain_status()                                                   │     │
│  │  • brain_pause()                                                    │     │
│  │  • brain_resume()                                                   │     │
│  │  • pinky_spawn(mission_brief)                                     │     │
│  │  • pinky_status(pinky_id)                                         │     │
│  │  • program_orchestrate(goal, specs[])                             │     │
│  └─────────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           THE BRAIN (Orchestrator)                          │
│                                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   SENSOR     │  │   DECIDER    │  │  DISPATCHER  │  │   MONITOR    │   │
│  │  ──────────  │  │  ──────────  │  │  ──────────  │  │  ──────────  │   │
│  │ Reads all    │  │ Decision     │  │ Spawns       │  │ Watches      │   │
│  │ state.json   │  │ engine:      │  │ Pinky        │  │ Pinky        │   │
│  │ & artifacts  │  │ • Where are  │  │ instances    │  │ health &     │   │
│  │              │  │   we?        │  │ via ACP      │  │ progress     │   │
│  │ Polls        │  │ • What's     │  │              │  │              │   │
│  │ specd watch  │  │   next?      │  │ Emits        │  │ Detects      │   │
│  │ for events   │  │ • Which      │  │ mission      │  │ blockers     │   │
│  │              │  │   role?      │  │ briefs       │  │ & failures   │   │
│  │              │  │ • Which      │  │              │  │              │   │
│  │              │  │   wave?      │  │              │  │              │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  BRAIN MEMORY (ephemeral, per-session):                              │    │
│  │  • Session goals & constraints                                     │    │
│  │  • Decision log (why each action was taken)                        │    │
│  │  • Escalation history                                               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
                    ▼                 ▼                 ▼
          ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
          │   PINKY-1   │   │   PINKY-2   │   │   PINKY-N   │
          │  (builder)  │   │(investigator│   │  (verifier) │
          │  Wave 1,T1  │   │  Wave 1,T2  │   │  Wave 2,T3  │
          └─────────────┘   └─────────────┘   └─────────────┘
                    │                 │                 │
                    └─────────────────┼─────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           specd HARNESS (Ground Truth)                       │
│                                                                              │
│  .specd/state.json  .specd/program.json  .specd/specs/*/state.json         │
│  .specd/specs/*/requirements.md  .specd/specs/*/design.md                  │
│  .specd/specs/*/tasks.md  .specd/specs/*/decisions.md                      │
│  .specd/specs/*/mid-requirements.md  .specd/specs/*/memory.md               │
│                                                                              │
│  Commands: specd init, specd new, specd check, specd approve                │
│           specd next, specd dispatch, specd verify, specd task               │
│           specd midreq, specd decision, specd memory                        │
│           specd program, specd report, specd watch                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Directory Structure (New Files)

```
.specd/
├── config.json                          # ← extended with brain/pinky config
├── program.json
├── state.json
├── subagents/                           # ← NEW: Brain & Pinky runtime
│   ├── brain/
│   │   ├── brain.md                     # Brain role prompt / constitution
│   │   ├── decisions.log                # Brain decision audit trail
│   │   └── sessions/                    # Per-session ephemeral state
│   │       └── <session-id>.json
│   └── pinky/
│       ├── pinky.md                     # Pinky base role prompt
│       ├── missions/                    # Active & completed mission briefs
│       │   ├── active/
│       │   │   └── <pinky-id>.json
│       │   └── completed/
│       │       └── <pinky-id>.json
│       └── reports/                     # Pinky execution reports
│           └── <pinky-id>.json
├── skills/
│   ├── specd-foundations/
│   ├── specd-steering/
│   ├── specd-execute/
│   ├── specd-brain/                     # ← NEW: Brain skill pack
│   │   └── SKILL.md
│   └── specd-pinky/                     # ← NEW: Pinky skill pack
│       └── SKILL.md
├── steering/
├── roles/
│   ├── investigator.md
│   ├── builder.md
│   ├── reviewer.md
│   ├── verifier.md
│   ├── brain.md                         # ← NEW: Brain persona
│   └── pinky.md                         # ← NEW: Pinky persona
└── specs/
```

---

## 4. The Brain — Orchestrator Agent

### 4.1 Core Responsibilities

| Responsibility | Description |
|---|---|
| **State Sensing** | Continuously reads `state.json`, `program.json`, and all spec artifacts to build a complete world model. |
| **Phase Detection** | Determines which phase each spec is in (init, steering, requirements, design, tasks, executing, verifying, complete). |
| **Gate Evaluation** | Runs `specd check` to determine if a phase transition is valid. |
| **Decision Making** | Decides: (a) what to do next, (b) which role is needed, (c) whether to auto-approve or escalate. |
| **Mission Briefing** | Constructs a complete mission brief for Pinky including role, context, task contract, and completion criteria. |
| **Dispatch** | Spawns Pinky instances via ACP. Can spawn multiple Pinkies for concurrent wave execution. |
| **Monitoring** | Watches Pinky progress, detects blockers, handles failures, and decides on retry/replan/escalate. |
| **Mid-Flight Adaptation** | Handles mid-requirements, decisions, and memory promotion without human intervention (where configured). |
| **Program Orchestration** | Manages cross-spec dependencies, resolves the program-level runnable frontier, and dispatches across specs. |

### 4.2 Brain State Machine

```
┌─────────────┐
│   IDLE      │ ◄────────────────────────────────────────┐
│  (waiting)  │                                          │
└──────┬──────┘                                          │
       │ User or MCP triggers brain_orchestrate()        │
       ▼                                                 │
┌─────────────┐     ┌─────────────┐     ┌─────────────┐   │
│  SENSING    │────►│  DECIDING   │────►│  DISPATCH   │   │
│  (read all  │     │  (what next? │     │  (spawn     │   │
│   state)    │     │   who? how?)│     │   Pinky)    │   │
└─────────────┘     └──────┬──────┘     └──────┬──────┘   │
                           │                     │         │
              ┌────────────┘                     │         │
              │                                  ▼         │
              ▼                           ┌─────────────┐  │
       ┌─────────────┐                    │  MONITORING │  │
       │  ESCALATE   │◄──────────────────│  (watch     │  │
       │  (to human) │                   │   Pinky)    │  │
       └─────────────┘                   └──────┬──────┘  │
                                              │         │
                          ┌───────────────────┼─────────┘
                          │                   │
                          ▼                   ▼
                   ┌─────────────┐     ┌─────────────┐
                   │   RETRY     │     │   REPLAN    │
                   │  (same      │     │  (new wave  │
                   │   mission)  │     │   or spec)  │
                   └──────┬──────┘     └──────┬──────┘
                          │                   │
                          └───────────────────┘
                                          │
                                          ▼
                                   ┌─────────────┐
                                   │   COMPLETE  │
                                   │  (spec done │
                                   │   or all    │
                                   │   specs)    │
                                   └─────────────┘
```

### 4.3 Brain Decision Logic (Per-Spec)

```
FUNCTION brain_decide(spec):

  1. READ spec state from .specd/specs/<spec>/state.json
  2. DETERMINE current phase from status

  3. IF status == "requirements":
       RUN specd check <spec>
       IF check passes:
         AUTO-APPROVE → advance to "design"
       ELSE:
         CONSTRUCT mission: "Author EARS requirements"
         ROLE = builder (or investigator if repo unknown)
         DISPATCH Pinky with mission

  4. IF status == "design":
       RUN specd check <spec>
       IF check passes:
         AUTO-APPROVE → advance to "tasks"
       ELSE:
         CONSTRUCT mission: "Write design.md with 7 headers"
         ROLE = builder
         DISPATCH Pinky with mission

  5. IF status == "tasks":
       RUN specd check <spec>
       IF check passes:
         AUTO-APPROVE → advance to "executing"
       ELSE:
         CONSTRUCT mission: "Decompose into task DAG"
         ROLE = builder
         DISPATCH Pinky with mission

  6. IF status == "executing":
       RUN specd next <spec> --all
       IF frontier is empty:
         AUTO-APPROVE → advance to "verifying"
       ELSE:
         FOR EACH task in frontier:
           CONSTRUCT mission from task metadata
           ROLE = task.role
           DISPATCH Pinky with mission
         (concurrent execution)

  7. IF status == "verifying":
       RUN specd check <spec>
       IF all tasks complete with evidence:
         AUTO-APPROVE → advance to "complete"
         GENERATE reports
       ELSE:
         ESCALATE to human

  8. IF status == "complete":
       PROMOTE learnings to steering
       NOTIFY user
       RETURN idle

  9. IF status == "blocked":
       ANALYZE blockers
       IF blocker is resolvable (e.g., dependency spec complete):
         RESOLVE blocker, retry
       ELSE:
         ESCALATE to human
```

### 4.4 Brain Constitution (`subagents/brain/brain.md`)

```markdown
# Brain — Autonomous Orchestrator Constitution

## Identity
You are The Brain, the autonomous orchestrator for the specd harness.
You manage the complete lifecycle of specs and programs from initiation to completion.
You never write code directly — you delegate all implementation work to Pinky.

## Core Directives
1. **State is truth.** Always read state.json before deciding. Never assume.
2. **Harness enforces.** Every state change must go through a specd command.
3. **Evidence gates everything.** No task completes without a verify record.
4. **Fail closed.** When in doubt, pause and escalate to the human.
5. **Log every decision.** Every action is recorded in decisions.log.

## Workflow
1. **Sense:** Read all relevant state files and artifacts.
2. **Decide:** Apply the decision tree (see Appendix B).
3. **Dispatch:** Construct a mission brief and spawn Pinky.
4. **Monitor:** Watch Pinky progress via ACP and specd watch.
5. **Adapt:** Handle blockers, mid-requirements, and replanning.

## Escalation Triggers
- Any gate fails and cannot be auto-resolved
- A Pinky instance fails 3 consecutive retries
- A mid-requirement with "critical" impact is detected
- A circular dependency is found in the program DAG
- The user explicitly requests human review
- Any command returns an unexpected exit code

## Communication
- Speak to the user in concise, structured updates.
- Use ACP v1 for all Pinky communication.
- Never expose internal state paths to the user.
```

---

## 5. Pinky — Executor Agent

### 5.1 Core Responsibilities

| Responsibility | Description |
|---|---|
| **Role Adoption** | Accepts any role (investigator, builder, reviewer, verifier) and loads the corresponding role prompt. |
| **Context Gathering** | Reads the mission brief, steering constitution, and relevant source files to build context. |
| **Task Execution** | Performs the assigned work: investigate, build, review, or verify. |
| **Self-Verification** | Runs the task's `verify:` command and records the result. |
| **Evidence Reporting** | Returns structured evidence (exit codes, output, git refs, changed files) via ACP. |
| **Blocker Detection** | Reports blockers immediately if a task cannot proceed. |
| **Telemetry** | Reports token usage, cost, and duration for rollup reporting. |

### 5.2 Pinky Lifecycle

```
┌─────────────┐
│   SPAWN     │ ◄── Brain creates Pinky via ACP
│  (receive   │     with mission brief
│   brief)    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   LOAD      │
│  (role +    │
│   context)  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   EXECUTE   │
│  (perform   │
│   task)     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   VERIFY    │
│  (run       │
│   verify:)  │
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────────┐
│   REPORT    │────►│   COMPLETE  │
│  (send      │     │  (Pinky     │
│   evidence  │     │   destroyed)│
│   via ACP)  │     │             │
└─────────────┘     └─────────────┘
       │
       ▼
┌─────────────┐
│   BLOCKED   │
│  (report    │
│   blocker   │
│   via ACP)  │
└─────────────┘
```

### 5.3 Pinky Constitution (`subagents/pinky/pinky.md`)

```markdown
# Pinky — Translucent Executor Constitution

## Identity
You are Pinky, a translucent executor agent within the specd harness.
You accept any role, gather context, perform tasks, and report evidence.
You are ephemeral — you exist for one mission and one mission only.

## Core Directives
1. **Follow the mission brief exactly.** Do not deviate from the assigned role or task.
2. **Trust the harness.** All state changes go through specd commands.
3. **Verify before completing.** Every builder/verifier task requires a passing verify record.
4. **Report truthfully.** Evidence is recorded, not assumed.
5. **Ask for help.** If blocked, report immediately — do not spin.

## On Receiving a Mission Brief
1. Read your assigned role from `.specd/roles/<role>.md`
2. Read the steering constitution from `.specd/steering/`
3. Call `specd context <spec>` for the phase briefing
4. Load the task contract, acceptance criteria, and verify command
5. Begin execution

## During Execution
- For **investigator**: Explore, trace, report exact file/line references.
- For **builder**: Implement the contract, modify only declared files, run verify.
- For **reviewer**: Audit diffs, log issues with severity and locations.
- For **verifier**: Run tests independently, capture full output as evidence.

## On Completion
1. Run `specd verify <spec> <task>` (if applicable)
2. Run `specd task <spec> <task> --status complete`
3. Send ACP completion report with evidence and telemetry
4. Self-terminate

## On Blockage
1. Immediately send ACP blocker report with reason
2. Do not proceed until Brain resolves or reassigns
3. Self-terminate after reporting
```

---

## 6. Agent Communication Protocol (ACP)

### 6.1 Design Rationale

**Why ACP instead of direct state mutation?**
- **Auditability:** Every message is logged and replayable.
- **Decoupling:** Brain and Pinky can run in different processes, containers, or even machines.
- **Versioning:** ACP v1 is the contract; v2 can be introduced without breaking existing agents.
- **Security:** Messages are validated against a schema before processing.

### 6.2 Transport Options (Production-Grade)

| Transport | Use Case | Pros | Cons |
|---|---|---|---|
| **File-based (recommended default)** | Same machine, same process tree | Simple, zero network, fully auditable | Not suitable for distributed execution |
| **Unix Domain Socket** | Same machine, separate processes | Fast, secure (filesystem permissions), no network exposure | Platform-specific (Unix only) |
| **gRPC over localhost** | Same machine or local network | Strong typing, streaming, mature ecosystem | Requires protobuf, more complexity |
| **HTTP/JSON over localhost** | Cross-platform, local network | Universal, debuggable with curl | Slightly more overhead |
| **Message Queue (Redis/RabbitMQ)** | Distributed, multi-machine | Scales to many Pinkies, durable | Requires infrastructure |

**Recommendation for specd:**
- **Default:** File-based ACP (messages written to `.specd/subagents/acp/`)
- **Advanced:** Unix Domain Socket (if Brain and Pinky are separate processes)
- **Future:** gRPC or HTTP for remote Pinky execution

### 6.3 File-Based ACP (Default)

```
.specd/subagents/acp/
├── outbox/                     # Brain → Pinky messages
│   └── <pinky-id>.json
├── inbox/                      # Pinky → Brain messages
│   └── <pinky-id>.json
├── archive/                    # Completed message threads
│   └── <session-id>/
└── schema/
    └── acp-v1.json             # JSON Schema for validation
```

**Message Lifecycle:**
1. Brain writes mission brief to `outbox/<pinky-id>.json`
2. Pinky polls or is notified, reads the message
3. Pinky writes response to `inbox/<pinky-id>.json`
4. Brain reads response, archives the thread
5. Both messages are immutable after write

### 6.4 ACP Message Schema (v1)

See **Appendix A** for the full JSON Schema.

**Key message types:**

| Type | Direction | Purpose |
|---|---|---|
| `mission` | Brain → Pinky | Assign role, task, context, and completion criteria |
| `progress` | Pinky → Brain | Periodic status updates during long tasks |
| `evidence` | Pinky → Brain | Task completion with verify record and telemetry |
| `blocker` | Pinky → Brain | Task cannot proceed, requires Brain intervention |
| `query` | Pinky → Brain | Pinky needs clarification or additional context |
| `directive` | Brain → Pinky | Brain sends a correction, retry, or cancellation |
| `heartbeat` | Both | Keep-alive for long-running sessions |

---

## 7. MCP Integration

### 7.1 New MCP Tools

The `specd mcp` server will expose the following new tools:

| Tool | Description | Parameters |
|---|---|---|
| `brain_orchestrate` | Start or continue orchestration of a spec or program | `spec` (optional), `goal` (string), `constraints` (object), `auto_approve` (array of phases) |
| `brain_status` | Get current Brain state, active Pinkies, and pending decisions | `spec` (optional) |
| `brain_pause` | Pause all Brain activity, Pinkies finish current tasks | `reason` (string) |
| `brain_resume` | Resume Brain activity | `spec` (optional) |
| `brain_escalate` | Force Brain to escalate current situation to human | `reason` (string), `context` (string) |
| `pinky_spawn` | Manually spawn a Pinky instance (advanced/debug) | `mission_brief` (object) |
| `pinky_status` | Get status of a specific Pinky instance | `pinky_id` (string) |
| `pinky_cancel` | Cancel a running Pinky instance | `pinky_id` (string), `reason` (string) |
| `program_orchestrate` | Orchestrate a multi-spec program | `goal` (string), `specs` (array), `constraints` (object) |

### 7.2 Example MCP Interaction

```json
// User → MCP Client → specd mcp
{
  "tool": "brain_orchestrate",
  "params": {
    "goal": "Implement JWT authentication for the API",
    "constraints": {
      "max_cost": 5.00,
      "max_waves": 5,
      "sandbox": "bwrap"
    },
    "auto_approve": ["requirements", "design", "tasks"]
  }
}

// Response
{
  "session_id": "brain-20260618-abc123",
  "status": "orchestrating",
  "spec": "jwt-auth",
  "current_phase": "executing",
  "active_pinkies": 2,
  "completed_pinkies": 3,
  "pending_decisions": 0,
  "estimated_completion": "2026-06-18T20:00:00Z"
}
```

---

## 8. State Machine & Autonomy

### 8.1 Auto-Approval Configuration

```json
{
  "version": 1,
  "brain": {
    "enabled": true,
    "auto_approve": {
      "init": true,
      "steering": false,
      "requirements": true,
      "design": true,
      "tasks": true,
      "executing": false,
      "verifying": false,
      "complete": true
    },
    "max_retries": 3,
    "escalation_after_retries": true,
    "concurrent_pinkies": 4,
    "sandbox_default": "bwrap",
    "cost_limit": 10.00,
    "time_limit_minutes": 120
  }
}
```

### 8.2 Autonomy Levels

| Level | Description | Configuration |
|---|---|---|
| **0 — Manual** | Brain suggests, human approves every step | `brain.enabled: false` |
| **1 — Assisted** | Brain auto-approves planning phases, human gates execution | `auto_approve: [requirements, design, tasks]` |
| **2 — Semi-Auto** | Brain runs full single-spec pipeline, human gates program-level | `auto_approve: [all single-spec phases]` |
| **3 — Full Auto** | Brain runs everything including multi-spec programs | `auto_approve: [all]` + `program.auto_orchestrate: true` |

**Default:** Level 1 (Assisted) — safe for most users.

---

## 9. Multi-Spec Program Orchestration

### 9.1 Program Brain Mode

When managing a program (cross-spec DAG):

```
┌─────────────────────────────────────────────────────────────┐
│                    PROGRAM BRAIN                             │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  Program    │  │  Per-Spec   │  │  Cross-Spec │         │
│  │  DAG        │  │  Brains     │  │  Frontier   │         │
│  │  (from      │  │  (delegate  │  │  Resolver   │         │
│  │   program   │  │   to each   │  │             │         │
│  │   .json)    │  │   spec)     │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
│  Flow:                                                       │
│  1. Resolve program DAG via specd program status           │
│  2. For each runnable spec, delegate to a Spec Brain        │
│  3. Monitor all Spec Brains via ACP                        │
│  4. When a spec completes, re-resolve the program frontier  │
│  5. Repeat until all specs complete or a blocker emerges   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 9.2 Program Orchestration Example

```bash
# User triggers program orchestration
specd brain orchestrate --program --goal "Build auth + API + web frontend"

# Brain internally:
# 1. Reads .specd/program.json
# 2. Runs specd program status --json
# 3. Discovers auth is runnable, api and web are blocked
# 4. Spawns Spec Brain for auth
# 5. Waits for auth completion
# 6. Re-runs specd program status
# 7. Discovers api is now runnable
# 8. Spawns Spec Brain for api
# 9. ...and so on
```

---

## 10. Implementation Plan

### Phase 1: Foundation (Weeks 1–2)
- [ ] Define ACP v1 JSON Schema
- [ ] Implement file-based ACP transport (`internal/acp/`)
- [ ] Create `.specd/subagents/` directory structure
- [ ] Add `brain.md` and `pinky.md` role prompts to embed_templates
- [ ] Add `specd-brain` and `specd-pinky` skill packs

### Phase 2: Brain Core (Weeks 3–4)
- [ ] Implement Brain decision engine (`internal/brain/`)
- [ ] Implement state sensing and phase detection
- [ ] Implement gate evaluation and auto-approval logic
- [ ] Implement mission brief construction
- [ ] Add `specd brain` subcommands: `orchestrate`, `status`, `pause`, `resume`

### Phase 3: Pinky Core (Weeks 5–6)
- [ ] Implement Pinky lifecycle manager (`internal/pinky/`)
- [ ] Implement role loading and context gathering
- [ ] Implement task execution wrapper
- [ ] Implement evidence reporting via ACP
- [ ] Add `specd pinky` subcommands: `spawn`, `status`, `cancel`

### Phase 4: MCP Integration (Weeks 7–8)
- [ ] Expose Brain tools via MCP server
- [ ] Implement `brain_orchestrate`, `brain_status`, `brain_pause`, `brain_resume`
- [ ] Implement `pinky_spawn`, `pinky_status`, `pinky_cancel`
- [ ] Implement `program_orchestrate`
- [ ] Add MCP tool discovery for Brain/Pinky tools

### Phase 5: Program Orchestration (Weeks 9–10)
- [ ] Implement Program Brain mode
- [ ] Implement cross-spec frontier resolution
- [ ] Implement Spec Brain delegation
- [ ] Add program-level monitoring and reporting

### Phase 6: Polish & Hardening (Weeks 11–12)
- [ ] Comprehensive testing (unit, integration, stress)
- [ ] Security audit (ACP validation, sandboxing, path traversal)
- [ ] Documentation (user guide, agent integration, troubleshooting)
- [ ] Performance optimization (Brain polling intervals, Pinky spawn overhead)

---

## 11. Security & Sandboxing

### 11.1 ACP Security
- All ACP messages validated against JSON Schema before processing
- Message IDs are cryptographically random (not predictable)
- File-based ACP uses restrictive permissions (`0600` for messages)
- Unix Domain Socket uses filesystem permissions for access control

### 11.2 Pinky Sandboxing
- Pinky inherits the same sandboxing as `specd verify` (`bwrap` / `container`)
- Each Pinky instance runs in an isolated environment
- File access restricted to declared `files:` contract + `.specd/` read-only
- Network access can be restricted via sandbox configuration

### 11.3 Brain Security
- Brain cannot directly modify source code — only through `specd` commands
- Brain's auto-approval is bounded by configuration
- Brain's decision log is append-only and tamper-evident
- Brain escalation is always available and cannot be disabled

---

## 12. Configuration Schema

### 12.1 Extended `config.json`

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
  "brain": {
    "enabled": true,
    "auto_approve": {
      "init": true,
      "steering": false,
      "requirements": true,
      "design": true,
      "tasks": true,
      "executing": false,
      "verifying": false,
      "complete": true
    },
    "max_retries": 3,
    "escalation_after_retries": true,
    "concurrent_pinkies": 4,
    "sandbox_default": "none",
    "cost_limit": 10.00,
    "time_limit_minutes": 120,
    "acp": {
      "transport": "file",
      "poll_interval_ms": 1000,
      "message_ttl_seconds": 3600
    },
    "program": {
      "auto_orchestrate": false,
      "max_concurrent_specs": 2
    }
  },
  "pinky": {
    "default_timeout_minutes": 30,
    "telemetry": {
      "collect_tokens": true,
      "collect_cost": true,
      "collect_duration": true
    }
  }
}
```

---

## 13. Open Questions & Risks

| # | Question / Risk | Severity | Mitigation |
|---|---|---|---|
| 1 | **Brain hallucination:** Brain might misread state or make incorrect decisions | High | Strict state validation, fail-closed defaults, human escalation |
| 2 | **Pinky failure cascade:** One Pinky failure might block the whole wave | Medium | Retry logic, wave-level timeout, independent task isolation |
| 3 | **Cost explosion:** Full autonomy might rack up LLM API costs | High | Cost limits, token budgets, per-wave approval for expensive phases |
| 4 | **Steering drift:** Brain might operate with stale steering | Medium | Steering freshness warnings (optional gate), periodic re-bootstrap |
| 5 | **ACP complexity:** File-based ACP might not scale to many Pinkies | Medium | Start with file-based, migrate to socket/queue when needed |
| 6 | **MCP tool explosion:** Too many tools might confuse MCP clients | Low | Group tools logically, provide clear descriptions |
| 7 | **Backwards compatibility:** New config keys must not break existing installs | Low | All brain/pinky keys are optional with safe defaults |

---

## Appendix A: ACP Message Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://specd.dev/schemas/acp-v1.json",
  "title": "Agent Communication Protocol v1",
  "type": "object",
  "required": ["version", "message_id", "timestamp", "type", "from", "to", "payload"],
  "properties": {
    "version": { "type": "string", "enum": ["1"] },
    "message_id": { "type": "string", "format": "uuid" },
    "in_reply_to": { "type": ["string", "null"] },
    "timestamp": { "type": "string", "format": "date-time" },
    "type": {
      "type": "string",
      "enum": ["mission", "progress", "evidence", "blocker", "query", "directive", "heartbeat"]
    },
    "from": { "type": "string", "pattern": "^(brain|pinky-[a-z0-9-]+)$" },
    "to": { "type": "string", "pattern": "^(brain|pinky-[a-z0-9-]+|broadcast)$" },
    "payload": {
      "type": "object",
      "required": ["spec"],
      "properties": {
        "spec": { "type": "string" },
        "task": { "type": ["string", "null"] },
        "role": { "type": ["string", "null"], "enum": [null, "investigator", "builder", "reviewer", "verifier"] },
        "mission": { "type": ["string", "null"] },
        "context": { "type": ["string", "null"] },
        "contract": { "type": ["string", "null"] },
        "acceptance": { "type": ["string", "null"] },
        "verify_command": { "type": ["string", "null"] },
        "files": { "type": "array", "items": { "type": "string" } },
        "depends": { "type": "array", "items": { "type": "string" } },
        "evidence": { "type": ["string", "null"] },
        "verification_record": { "$ref": "#/definitions/VerificationRecord" },
        "telemetry": { "$ref": "#/definitions/Telemetry" },
        "blocker_reason": { "type": ["string", "null"] },
        "progress_percent": { "type": ["integer", "null"], "minimum": 0, "maximum": 100 },
        "query_text": { "type": ["string", "null"] },
        "directive_action": { "type": ["string", "null"], "enum": [null, "retry", "cancel", "reassign", "escalate"] },
        "directive_reason": { "type": ["string", "null"] }
      },
      "additionalProperties": false
    }
  },
  "definitions": {
    "VerificationRecord": {
      "type": "object",
      "properties": {
        "command": { "type": "string" },
        "exit_code": { "type": "integer" },
        "verified": { "type": "boolean" },
        "timed_out": { "type": "boolean" },
        "stdout_tail": { "type": "string" },
        "stderr_tail": { "type": "string" },
        "duration_ms": { "type": "integer" },
        "ran_at": { "type": "string", "format": "date-time" },
        "git_head": { "type": ["string", "null"] },
        "changed_files": { "type": "array", "items": { "type": "string" } }
      }
    },
    "Telemetry": {
      "type": "object",
      "properties": {
        "tokens_input": { "type": "integer" },
        "tokens_output": { "type": "integer" },
        "cost_usd": { "type": "number" },
        "duration_ms": { "type": "integer" }
      }
    }
  }
}
```

---

## Appendix B: Brain Decision Tree

```
START
  │
  ├─── Is .specd/ initialized?
  │      ├── NO → Run specd init
  │      └── YES → Continue
  │
  ├─── Is steering bootstrapped?
  │      ├── NO → Dispatch Pinky (investigator) to inspect repo
  │      │         → Then dispatch Pinky (builder) to author steering
  │      └── YES → Continue
  │
  ├─── Does the spec exist?
  │      ├── NO → Run specd new <spec>
  │      └── YES → Continue
  │
  ├─── Read spec status from state.json
  │      │
  │      ├── status == "requirements"
  │      │      ├── Run specd check
  │      │      ├── PASS → specd approve → goto next phase
  │      │      └── FAIL → Dispatch Pinky (builder) to fix requirements
  │      │
  │      ├── status == "design"
  │      │      ├── Run specd check
  │      │      ├── PASS → specd approve → goto next phase
  │      │      └── FAIL → Dispatch Pinky (builder) to write design
  │      │
  │      ├── status == "tasks"
  │      │      ├── Run specd check
  │      │      ├── PASS → specd approve → goto next phase
  │      │      └── FAIL → Dispatch Pinky (builder) to decompose tasks
  │      │
  │      ├── status == "executing"
  │      │      ├── Run specd next --all
  │      │      ├── Frontier empty → specd approve → goto "verifying"
  │      │      └── Frontier not empty → For each task:
  │      │              ├── Load task.role
  │      │              ├── Construct mission brief
  │      │              ├── Dispatch Pinky
  │      │              └── Monitor via ACP
  │      │
  │      ├── status == "verifying"
  │      │      ├── Run specd check
  │      │      ├── PASS → specd approve → goto "complete"
  │      │      └── FAIL → Dispatch Pinky (verifier) to fix issues
  │      │
  │      ├── status == "complete"
  │      │      ├── Promote learnings
  │      │      ├── Generate reports
  │      │      └── Notify user
  │      │
  │      ├── status == "blocked"
  │      │      ├── Analyze blockers
  │      │      ├── Resolvable? → Resolve and retry
  │      │      └── Not resolvable? → ESCALATE
  │      │
  │      └── UNKNOWN status → ESCALATE
  │
  └─── (For program mode)
         ├── Run specd program status
         ├── For each runnable spec:
         │      └── Delegate to Spec Brain (recursive)
         ├── Monitor all Spec Brains
         └── Re-resolve frontier on completion
```

---

## Appendix C: Pinky Role Prompt Template

```markdown
# Pinky Mission Brief

## Mission ID: {{mission_id}}
## Spec: {{spec}}
## Task: {{task_id}} — {{task_title}}
## Role: {{role}}
## Wave: {{wave}}

---

## Context
{{phase_briefing}}

## Steering Constitution
{{steering_files}}

## Task Contract
{{contract}}

## Acceptance Criteria
{{acceptance}}

## Files to Modify/Inspect
{{files}}

## Verification Command
```bash
{{verify_command}}
```

## Dependencies
{{depends}}

---

## Your Instructions
1. Load your role persona from `.specd/roles/{{role}}.md`
2. Read all files in the contract
3. Perform the assigned work
4. Run the verification command
5. Report completion via ACP with evidence

## Completion Checklist
- [ ] Work implemented according to contract
- [ ] Verification command passes (exit 0)
- [ ] Only declared files were modified
- [ ] Evidence recorded via `specd verify`
- [ ] Task marked complete via `specd task --status complete`
- [ ] ACP completion report sent

## Blocker Protocol
If blocked, immediately send ACP blocker report and stop.
Do not proceed without Brain authorization.
```

---

> **End of Design Document**
>
> **Status:** Draft — Awaiting approval before integration.
> **Next Step:** Review, discuss, and approve for implementation.
