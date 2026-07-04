# specd — Coding Harness for Agent-Agnostic Process Enforcement

`specd` is an agent-agnostic, spec-driven coding harness delivered as a deterministic, dependency-free Go CLI binary. 

The core thesis of the project: **`Agent = Model + Harness`**. In this paradigm, the AI agent reasons and creates (specs, design, and code), while the harness (`specd`) enforces process rules, manages spec state, executes sandboxed validation gates, and records cryptographic verification evidence. It keeps process enforcement off the model's context window and enforces it as a pure function of local, on-disk state.

---

## 1. Repository Layout

| Path / Directory | What it is |
| :--- | :--- |
| **[`AGENTS.md`](file:///var/www/html/rai/up/specd/AGENTS.md)** | The operating brief and guidelines for coding agents. |
| **[`docs/`](file:///var/www/html/rai/up/specd/docs/)** | Self-contained design and architectural documents. See **[`docs/README.md`](file:///var/www/html/rai/up/specd/docs/README.md)** for navigation. |
| **[`reference/`](file:///var/www/html/rai/up/specd/reference/)** | The **frozen v1 implementation** — read-only, used to observe past behavior. |

---

## 2. Core Concepts

*   **The Foundational Split:** The agent operates in an untrusted execution environment. The harness performs deterministic validation, keeping process gates entirely free of LLM non-determinism.
*   **Local Specs on Disk:** Specifications (`requirements.md`), designs (`design.md`), and tasks (`tasks.md`) live directly in the repository as human-readable, version-controlled Markdown.
*   **Evidence-Gated Completion:** Tasks are only marked complete when local verification scripts run successfully (exit `0`) and their cryptographic output is logged.
*   **Wave Execution Frontiers:** Tasks form a Directed Acyclic Graph (DAG) parsed from the Markdown file. The scheduler computes parallel execution "frontiers" representing concurrent work waves.

---

## 3. Project Documentation Directory

For complete details on requirements, design patterns, invariants, and syntax, navigate to the **[`docs/`](file:///var/www/html/rai/up/specd/docs/)** folder. 

Refer to the **[Documentation Index & Navigational Guide](file:///var/www/html/rai/up/specd/docs/README.md)** to access:
*   [01-philosophy-charter.md](file:///var/www/html/rai/up/specd/docs/01-philosophy-charter.md) — Product split & guiding principles.
*   [02-spec-lifecycle-state.md](file:///var/www/html/rai/up/specd/docs/02-spec-lifecycle-state.md) — State machine & CAS/locking persistence.
*   [03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md) — Pluggable gates engine.
*   [04-task-dag-waves.md](file:///var/www/html/rai/up/specd/docs/04-task-dag-waves.md) — Lossless tasks parser & DAG scheduling.
*   [05-evidence-verification.md](file:///var/www/html/rai/up/specd/docs/05-evidence-verification.md) — Sandboxed execution & evidence ledgers.
*   [06-agent-integration.md](file:///var/www/html/rai/up/specd/docs/06-agent-integration.md) — Agent configuration, roles, and steering rules.
*   [07-mcp-surface.md](file:///var/www/html/rai/up/specd/docs/07-mcp-surface.md) — Model Context Protocol stdio JSON-RPC server.
*   [08-context-engineering.md](file:///var/www/html/rai/up/specd/docs/08-context-engineering.md) — Context manifests & token budgets.
*   [09-multi-agent-orchestration.md](file:///var/www/html/rai/up/specd/docs/09-multi-agent-orchestration.md) — Asynchronous multi-agent loops (Brain/Pinky).
*   [10-cli-foundations.md](file:///var/www/html/rai/up/specd/docs/10-cli-foundations.md) — Argument parsing, exit codes, and lock primitives.
*   [11-reporting-observability.md](file:///var/www/html/rai/up/specd/docs/11-reporting-observability.md) — Observability summaries and Prometheus metrics.
*   [12-deferred-flywheel-security.md](file:///var/www/html/rai/up/specd/docs/12-deferred-flywheel-security.md) — Pluggable security scans & re-entry contracts.

---

## 4. Looking for the previous (shipped) version?

Everything that was `specd` v1 now lives under [`reference/`](file:///var/www/html/rai/up/specd/reference/) — source (`reference/internal/`, `reference/main.go`), docs (`reference/docs/`), the built binary (`reference/specd`), and its own `README.md`/`AGENTS.md`. It is preserved to learn from, not to extend.
