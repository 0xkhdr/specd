# specd — Coding Harness for Agent-Agnostic Process Enforcement

> **This repository is undergoing a from-scratch rebuild (v2).** `specd` is being reimplemented on the **minimal accurate path**: keeping the core thesis (`Agent = Model + Harness`), dropping secondary accretion, and grounding every decision in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf).

The core thesis of `specd`: **the agent reasons and creates; the harness enforces process rules.** 

`specd` is an agent-agnostic, spec-driven coding harness delivered as a deterministic, dependency-free Go CLI binary. It moves process integrity out of the LLM's context window and onto local, tool-gated, evidence-based validations.

---

## 1. Repository Layout

| Path / Directory | What it is |
| :--- | :--- |
| **[`AGENTS.md`](file:///var/www/html/rai/up/specd/AGENTS.md)** | The active operating brief for coding agents working in this tree. |
| **[`docs/`](file:///var/www/html/rai/up/specd/docs/)** | Comprehensive design documents organized by system domain. See **[`docs/README.md`](file:///var/www/html/rai/up/specd/docs/README.md)** for navigation. |
| **[`fresh-start/`](file:///var/www/html/rai/up/specd/fresh-start/)** | The rebuild inputs: 12 domain analyses, cross-spec roadmaps, and scope triage. |
| **[`reference/`](file:///var/www/html/rai/up/specd/reference/)** | The **frozen v1 implementation** — read-only, used to observe past behavior. |
| **[`The_New_SDLC_With_Vibe_Coding.pdf`](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf)** | The PDF paper outlining the new SDLC philosophy. |

---

## 2. Rebuild & Execution Pipeline

The rebuild is structured around 12 decoupled system domains:

```
  domain analyses            spec authoring              implementation
  (fresh-start/*.md)   ──►   (spec.md + tasks.md)  ──►   (Go source + waves)
  ┌───────────────┐          ┌──────────────────┐        ┌──────────────────┐
  │ COMPLETE      │          │ NEXT (on approval)│        │ AFTER specs green │
  │ 12 domains +  │          │ per 00-roadmap    │        │ build in waves    │
  │ roadmap/ADRs  │          │ order (01→10→02…) │        │ A→H               │
  └───────────────┘          └──────────────────┘        └──────────────────┘
```

*   **Stage 1 — Domain Analysis (Complete):** The requirements, triaged command lists, and task structures are staged under `fresh-start/`.
*   **Stage 2 — Spec & Tasks Authoring (Pending Human Approval):** Translating domain analyses into structured, executable specs (`spec.md` + `tasks.md`) in topological order.
*   **Stage 3 — Wave Implementation (Pending Specs Verification):** Implementing the Go source files in cross-domain execution waves.

---

## 3. Project Documentation

For a detailed review of each component's requirements, designs, invariants, and CLI syntax, navigate to the **[`docs/`](file:///var/www/html/rai/up/specd/docs/)** directory. 

Start with the **[Documentation Index & Navigational Guide](file:///var/www/html/rai/up/specd/docs/README.md)** to locate specifics on:
*   [01-philosophy-charter.md](file:///var/www/html/rai/up/specd/docs/01-philosophy-charter.md) — Product split & guiding principles.
*   [02-spec-lifecycle-state.md](file:///var/www/html/rai/up/specd/docs/02-spec-lifecycle-state.md) — State machine & CAS/locking persistence.
*   [03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md) — Pluggable gates engine.
*   [04-task-dag-waves.md](file:///var/www/html/rai/up/specd/docs/04-task-dag-waves.md) — Lossless tasks parser & DAG scheduling.
*   [05-evidence-verification.md](file:///var/www/html/rai/up/specd/docs/05-evidence-verification.md) — Sandboxed execution & evidence ledgers.
*   [06-agent-integration.md](file:///var/www/html/rai/up/specd/docs/06-agent-integration.md) — Agent configuration, roles, and steering.
*   [07-mcp-surface.md](file:///var/www/html/rai/up/specd/docs/07-mcp-surface.md) — Model Context Protocol stdio server.
*   [08-context-engineering.md](file:///var/www/html/rai/up/specd/docs/08-context-engineering.md) — Manifests & token context budgets.
*   [09-multi-agent-orchestration.md](file:///var/www/html/rai/up/specd/docs/09-multi-agent-orchestration.md) — Asynchronous multi-agent loops (Brain/Pinky).
*   [10-cli-foundations.md](file:///var/www/html/rai/up/specd/docs/10-cli-foundations.md) — Argument parsing, exit codes, and primitives.
*   [11-reporting-observability.md](file:///var/www/html/rai/up/specd/docs/11-reporting-observability.md) — Observability summaries and Prometheus metrics.
*   [12-deferred-flywheel-security.md](file:///var/www/html/rai/up/specd/docs/12-deferred-flywheel-security.md) — Re-entry contracts for post-build loop.

---

## 4. Looking for the previous (shipped) version?

Everything that was `specd` v1 now lives under [`reference/`](file:///var/www/html/rai/up/specd/reference/) — source (`reference/internal/`, `reference/main.go`), docs (`reference/docs/`), the built binary (`reference/specd`), and its own `README.md`/`AGENTS.md`. It is preserved to learn from, not to extend.
