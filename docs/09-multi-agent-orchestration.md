# Multi-Agent Orchestration (Brain/Pinky)

This document describes the design of the opt-in orchestration tier in `specd` (v2), detailing the async Brain/Pinky model, lease safety, cost brakes, and file-backed log transport.

---

## 1. Conductor vs. Orchestrator Modes

`specd` supports two distinct modes of execution, matching the classification in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf) (p.31):

*   **Conductor Mode (`mode: simple`):** Designed for single-agent IDE interactions (Claude Code, Cursor). The human approves phase gates, and the agent executes tasks sequentially.
*   **Orchestrator Mode (`mode: orchestrated`):** Designed for multi-agent asynchronous workflows. A central **Brain** controller manages scheduling, and ephemeral **Pinky** workers execute tasks in parallel.

---

## 2. Ephemeral Workers & The Brain

The orchestration system separates orchestration decisions from implementation work:

### A. The Brain (Deterministic Controller)
The Brain manages the spec's state machine. 

*No-LLM Invariant:* **The Brain never calls an LLM and never executes unsafe code.** It is a pure, deterministic state machine implemented in `decide.go`. It reads the current snapshot and returns a control decision:
*   `dispatch`: Schedule a runnable task to a worker.
*   `wait`: Await completion of running tasks.
*   `await-approval`: Pause for human authorization.
*   `complete`: The specification has been fully realized.
*   `escalate`: A task has failed verification or lease retry limits.
*   `policy-violation`: An execution limit (e.g. cost) has been exceeded.

### B. Pinky (Ephemeral Workers)
Pinky workers are temporary processes spawned to complete a single task. A worker claims a task, downloads its context brief, performs the edits, runs local verification, and uploads checkpoints and completion reports.

---

## 3. File-Backed Agent Communication Protocol (ACP)

To maintain a zero-dependency, local-first architecture, all coordination between the Brain and workers is file-backed. Communication occurs via the **Agent Communication Protocol (ACP)**:

*   **State Ledger:** Orchestration state is saved in `.specd/specs/<slug>/orchestration/session.json` under CAS lock.
*   **ACP Stream:** Log records of worker actions, heartbeats, and status changes are appended to `.specd/specs/<slug>/orchestration/acp/*.jsonl`.
*   **Worker Inbox:** Worker directives are written to local files where workers scan for cancellation or updates.

*Origin:* Consolidated from the core orchestration and ACP files in [reference/internal/core/](file:///var/www/html/rai/up/specd/reference/internal/core/).

---

## 4. Safety Guardrails & Brakes

Multi-agent async execution requires robust guardrails to prevent infinite loops, resource wastage, or runaway costs:

### A. Task Leases & Heartbeats
When a worker claims a task, it acquires a lease for a given duration. The worker must regularly write heartbeat signals. If a lease expires without a heartbeat:
1.  The Brain reclaims the task.
2.  The task is rescheduled to a new worker.
3.  Rescheduling repeats up to `maxRetries`. If exceeded, the Brain escalates.

### B. Advisory Cost Brake
Workers report their accumulated API costs (`host-cost`) in their heartbeats. The Brain sums these reported costs and halts execution with a `policy-violation` if the total exceeds `hostReportedCostLimitUSD`.

### C. Time Brake
Tasks carry mission deadlines. If a worker fails to complete before the deadline, the harness terminates the worker's process group.

### D. Evidence Integrity
A worker's task completion report is accepted **only if** it references a local `pass` verification record matching the task and the current git `HEAD`. Workers cannot self-report completion without running verification.

*Origin:* Safety primitives from [cost_brake.go](file:///var/www/html/rai/up/specd/reference/internal/core/cost_brake.go) and [trajectory.go](file:///var/www/html/rai/up/specd/reference/internal/core/trajectory.go).

---

## 5. Command Surface

The orchestration commands are compiled in always but remain inert unless `orchestration.enabled` is set to `true` in `config.yml`.

### The Brain Interface
*   `specd brain start <slug>`: Initializes a session.
*   `specd brain step <slug>`: Executes a single decision step.
*   `specd brain run <slug>`: Launches the continuous loop runner.
*   `specd brain status <slug>`: Prints session status, active leases, and costs.
*   `specd brain approve <slug>`: Grants human authorization to advance phases.
*   `specd brain cancel <slug>`: Emits cancel signals to all workers.
*   `specd brain resume <slug>`: Recovers a session from the last checkpoint.

### The Pinky Interface
*   `specd pinky claim <slug> <task>`: Claims a task and locks the spec.
*   `specd pinky heartbeat <slug> <task>`: Sends progress/liveness signals.
*   `specd pinky report <slug> <task>`: Submits completion reports and evidence.
*   `specd pinky inbox <slug> <task>`: Checks for controller directives (cancel).
*   `specd pinky checkpoint <slug> <task>`: Saves intermediate work states.
