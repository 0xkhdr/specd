# specd Documentation Index & Navigational Guide

Welcome to the design and architectural documentation for the rebuilt **`specd`** (v2) harness. This documentation is structured around the 12 system domains of the codebase, aligning with the principles of **Harness Engineering** outlined in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf) (pp.26–34).

---

## 1. Documentation Map & Navigation

Below is the directory index of documentation files. Follow this guide to locate details on spec lifecycle, validation, agent integrations, or CLI architecture.

| Doc File | Target Domain | Key Topics & System Capabilities |
| :--- | :--- | :--- |
| **[01-philosophy-charter.md](file:///var/www/html/rai/up/specd/docs/01-philosophy-charter.md)** | `01-product-philosophy-core` | The Foundational Split (Agent vs Harness), the 8 guiding invariants, and the 16-verb Harness Charter command-map. |
| **[02-spec-lifecycle-state.md](file:///var/www/html/rai/up/specd/docs/02-spec-lifecycle-state.md)** | `02-spec-lifecycle-state` | Phase transitions (forward phase ratchet), state schema (`state.json`), lock discipline, CAS revisions, and atomic writes. |
| **[03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md)** | `03-validation-gates-engine` | Pluggable gate registry, the 7 Core Gates, opt-in/custom gates, and stdlib security scanners. |
| **[04-task-dag-waves.md](file:///var/www/html/rai/up/specd/docs/04-task-dag-waves.md)** | `04-task-dag-wave-execution` | The `tasks.md` checklist syntax, lossless Markdown parsing/serialization round-trip, DAG cycle validation, and the execution frontier. |
| **[05-evidence-verification.md](file:///var/www/html/rai/up/specd/docs/05-evidence-verification.md)** | `05-evidence-verification` | Sandbox execution (`bwrap`/containers), environment scrubbing, immutable verification record schema, and git-linked evidence ledger. |
| **[06-agent-integration.md](file:///var/www/html/rai/up/specd/docs/06-agent-integration.md)** | `06-agent-agnostic-integration` | Host integration floor (the `--config` snippet), agent roles (`scout`/`craftsman`/`validator`/`auditor`), steering rules, and adapter safety contract. |
| **[07-mcp-surface.md](file:///var/www/html/rai/up/specd/docs/07-mcp-surface.md)** | `07-mcp-handshake-surface` | The native Go stdio JSON-RPC MCP server, tool specifications, bootstrap version handshake, and per-spec tool policies. |
| **[08-context-engineering.md](file:///var/www/html/rai/up/specd/docs/08-context-engineering.md)** | `08-context-engineering` | Context manifests, item modes (`read-full`, `read-targeted`, `run-command`, `reference`), token heuristics, and the context-budget gate. |
| **[09-multi-agent-orchestration.md](file:///var/www/html/rai/up/specd/docs/09-multi-agent-orchestration.md)** | `09-orchestration-brain-pinky` | Brain/Pinky async multi-agent loop, task leases/heartbeats, cost and time brakes, and the Agent Communication Protocol (ACP). |
| **[10-cli-foundations.md](file:///var/www/html/rai/up/specd/docs/10-cli-foundations.md)** | `10-cli-architecture-foundations` | CLI arg parser, single-source dispatch registry, YAML loader, exit code mappings, and `SPECD_*` environment control. |
| **[11-reporting-observability.md](file:///var/www/html/rai/up/specd/docs/11-reporting-observability.md)** | `11-reporting-observability` | Pure projection formatting (no model, no network in reports), PR summaries, Prometheus metrics textfiles, and deferred live streams. |
| **[12-deferred-flywheel-security.md](file:///var/www/html/rai/up/specd/docs/12-deferred-flywheel-security.md)** | `12-flywheel-triage-tier` | Triaged flywheel feature status (eval, review, deploy, ingest), and the two-seam re-entry contract (Gates and Records map). |

---

## 2. Reading Paths

Depending on your objective, we recommend the following reading sequences:

*   **For Host/Agent Integrations (IDE Tooling, MCP Clients):**
    Start with [01-philosophy-charter.md](file:///var/www/html/rai/up/specd/docs/01-philosophy-charter.md) for the high-level philosophy, then read [06-agent-integration.md](file:///var/www/html/rai/up/specd/docs/06-agent-integration.md) (roles/ steering) and [07-mcp-surface.md](file:///var/www/html/rai/up/specd/docs/07-mcp-surface.md) (the JSON-RPC interface).
*   **For Core Harness Development:**
    Read [02-spec-lifecycle-state.md](file:///var/www/html/rai/up/specd/docs/02-spec-lifecycle-state.md) (lifecycle state), [03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md) (pluggable gates), [04-task-dag-waves.md](file:///var/www/html/rai/up/specd/docs/04-task-dag-waves.md) (DAG scheduler), and [10-cli-foundations.md](file:///var/www/html/rai/up/specd/docs/10-cli-foundations.md) (file locks, CAS, and parser foundations).
*   **For Multi-Agent Orchestration Integration:**
    Review [09-multi-agent-orchestration.md](file:///var/www/html/rai/up/specd/docs/09-multi-agent-orchestration.md) alongside the context manifest details in [08-context-engineering.md](file:///var/www/html/rai/up/specd/docs/08-context-engineering.md) and evidence records in [05-evidence-verification.md](file:///var/www/html/rai/up/specd/docs/05-evidence-verification.md).

---

## 3. Structural Relationships (The Dependency DAG)

The design documents and code domains form a directed dependency graph. You should assume that concepts described upstream are inherited downstream:

```
              ┌─────────────────────────────┐
              │ 01 Product & Philosophy     │ (Guiding Charter)
              └──────────────┬──────────────┘
                             │
              ┌──────────────▼──────────────┐
              │ 10 CLI Foundations          │ (I/O, Locks, CAS, Config)
              └──────┬───────────────┬──────┘
                     │               │
     ┌───────────────▼──┐            ▼
     │ 02 Spec Lifecycle│◄─────── (uses locks/CAS)
     └───┬───────┬──────┘
         │       │
┌────────▼──┐ ┌──▼────────────────┐
│ 04 Task   │ │ 05 Evidence &      │
│  DAG      │ │    Verification    │
└────┬──────┘ └──┬────────────────┘
     │           │
     │    ┌──────▼───────────────┐
     └───►│ 03 Validation Gates  │◄──── 08 (Context-budget gate)
          │    Engine            │
          └──────┬───────────────┘
                 │
┌────────────────▼──────┐    ┌──────────────────────────┐
│ 08 Context Engineering│    │ 06 Agent Integration     │
└───────┬───────────────┘    └───────────┬──────────────┘
        │                                │
        │            ┌───────────────────▼──┐
        │            │ 07 MCP Surface       │
        │            └─────────┬────────────┘
        │                      │
┌───────▼──────────────────────▼───────┐
│ 09 Multi-Agent Orchestration         │ (Brain/Pinky control plane)
└───────────────────┬──────────────────┘
                    │
     ┌──────────────▼───────────┐    ┌───────────────────────────┐
     │ 11 Observability         │    │ 12 Deferred Flywheel      │
     └──────────────────────────┘    └───────────────────────────┘
```
