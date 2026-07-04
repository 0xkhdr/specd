# Product Philosophy & Harness Charter

This document outlines the core product philosophy, guiding principles, and harness charter of `specd` (v2). 

`specd` is built upon a single, central thesis: **`Agent = Model + Harness`**. In this paradigm, the artificial agent reasons, plans, and generates code, while the harness (`specd`) enforces process rules, manages state, coordinates execution, and records verifiable execution evidence.

---

## 1. The Foundational Split (P1)

As outlined in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf) (Harness Engineering, pp.26–34), most agent failures are process or configuration failures rather than logic failures. The Foundational Split addresses this by drawing a strict line between the Agent and the Harness:

*   **The Agent** creates, reasons, design specs, and implements code. It operates in an untrusted execution environment.
*   **The Harness (`specd`)** is a deterministic process-enforcement engine. It never uses a Large Language Model (LLM) or network call for routing, gate checks, or state updates. It treats all agent-created artifacts (`requirements.md`, `design.md`, `tasks.md`) as untrusted inputs and validates them using pure, deterministic functions.

---

## 2. The Eight Guiding Principles

The architecture and implementation of `specd` are governed by eight invariants:

1.  **Determinism First (P1):** No model or network call controls the harness's decision path or state transitions.
2.  **Specs as the Source of Truth (P2):** Spec specifications, plans, and decisions exist as human-readable, git-diffable Markdown files on disk. The harness does not hold the entire plan in a transient model context.
3.  **Evidence-Gated Completion (P3):** No task or spec may transition to `complete` without a verifiable, cryptographically hashed evidence trace (a verify command returning exit code `0` on the exact git `HEAD`).
4.  **Waves, Not Lines (P4):** Task plans are Directed Acyclic Graphs (DAGs) representing parallel execution waves. Execution proceeds along the wavefront, allowing concurrent execution by multiple worker agents.
5.  **Agent-Agnostic by Design (P5):** Integration is built on standard configuration and tools. The harness communicates with any agent host (Claude Code, Cursor, custom scripts) via standard stdin/stdout protocols.
6.  **Human Gates at Phase Boundaries (P6):** Semantic boundaries (e.g., advancing from requirements to design, or design to execution) require explicit human approval actions.
7.  **Deterministic Reporting (P7):** Human-facing reports, logs, and summaries are pure projections of the underlying machine state.
8.  **Steering as Constitution (P8):** Agent behavior is steered by explicit rule-files (constitution) injected by the harness into the agent's context.

---

## 3. Harness Charter & Verb Map

The MVP commands are strictly categorized under the **Harness Charter**. Any feature or command must map to one of the seven harness components and at least one principle:

| Verb | Harness Component | Primary Principle | Verdict & Origin |
| :--- | :--- | :--- | :--- |
| `init` | Workspace Bootstrap | P2: Local Truth | **SIMPLIFY** from [init.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/init.go) (803 lines) — Scaffold `.specd/`, write templates, emit one plan JSON. |
| `new` | Spec Creation | P2: Local Truth | **KEEP** from [new.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/new.go) (92 lines) — Validates slug, creates spec directories. |
| `check` | Validation Gate | P1: Determinism First | **REDESIGN** from [gates.go](file:///var/www/html/rai/up/specd/reference/internal/core/gates.go) — Move from hardcoded branches to a pluggable gate registry. |
| `approve` | Process Gate | P6: Human Approval | **KEEP** from [approve.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/approve.go) (237 lines) — Transition phases under lock. |
| `next` | DAG Scheduler | P4: Waves, Not Lines | **KEEP** from [next.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/next.go) — Computes the next executable wavefront. Merges `waves` output. |
| `verify` | Sandbox Execution | P3: Evidence Integrity | **SIMPLIFY** from [verify.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/verify.go) (399 lines) — Run sandboxed commands, generate evidence ledger. |
| `task` | State Management | P3: Evidence Integrity | **KEEP** from [task_complete.go](file:///var/www/html/rai/up/specd/reference/internal/core/task_complete.go) — Mark task complete based on verified evidence. |
| `status` | Observability | P7: Deterministic Reports | **KEEP** from [status.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/status.go) — Projections of `state.json`. |
| `context` | Context Engine | P8: Context Discipline | **REDESIGN** from [contextpkg](file:///var/www/html/rai/up/specd/reference/internal/context) — Elevate to first-class package supplying manifests. |
| `decision` | Ledger | P2: Specs-on-Disk | **KEEP** from [decision.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/decision.go) — Appends ADR records to the spec log under lock. |
| `midreq` | Process Gate | P6: Human Approval | **KEEP** from [midreq.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/midreq.go) — Handles critical mid-requirement changes. |
| `memory` | Steering | P8: Steering Constitution| **KEEP & MERGE** — Manages agent memory/steering rules. Absorbs the old `promote` command. |
| `report` | Observability | P7: Deterministic Reports | **SIMPLIFY** from [report_actions.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/report_actions.go) (611 lines) — Retain static reports; defer live streams. |
| `handshake` | On-ramp | P5: Agent-Agnostic | **KEEP** from [handshake.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/handshake.go) — Surfaces integration info and policy digests to host. |
| `mcp` | Tool Interface | P5: Agent-Agnostic | **SIMPLIFY** from [mcp.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/mcp.go) — Native stdio JSON-RPC server; cut raw passthrough tools. |
| `brain` | Orchestration | P1: Determinism First | **REDESIGN** from [brain.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/brain.go) (Opt-in tier) — Manage Brain multi-agent control loops. |
| `pinky` | Orchestration | P1: Determinism First | **SIMPLIFY** from [pinky.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/pinky.go) (Opt-in tier) — Claim and checkpoint tasks. |

---

## 4. Triage and Scope Decisions (v2)

To reclaim the core engineering principles of `specd`, the following features were triaged:

*   **External DB Backends:** Postgres and Redis backends ([reference/internal/core/backend_postgres.go](file:///var/www/html/rai/up/specd/reference/internal/core/backend_postgres.go)) are **CUT** from the default binary to ensure zero runtime dependencies. Git-native remains the default and only default backend.
*   **Flywheel Surface:** Deferred most of the telemetry/feedback flywheel (`eval`, `review`, `deploy`, `observe`, `ingest`, `harness`) to v2 or plugin models. Only `security` scanning ([reference/internal/core/security/](file:///var/www/html/rai/up/specd/reference/internal/core/security/)) returns in v1, specifically integrated as a pluggable validation gate.
*   **Conductor & Orchestrate:** The rejection-analytics `conductor` command is **DEFERRED**, and auto-escalation resolution (`orchestrate`) is **CUT** and folded directly into Brain controller decisions.
