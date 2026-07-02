# specd v0.2.0 Evolution Plan
## From Spec-Driven Harness to Full-Cycle Agentic Engineering Platform

**Date:** 2026-07-02  
**Target Version:** v0.2.0  
**Author:** AI Analysis  
**Status:** Draft for Review  

---

## 1. Executive Summary

This document defines the evolution of `specd` from a spec-driven coding harness (v0.1.0) into a full-cycle agentic engineering platform (v0.2.0) that completely implements the AI-driven SDLC theory described in *The New SDLC with Vibe Coding* (Google, May 2026). The plan centers on two dual operating modes — **Conductor** (real-time, synchronous, in-IDE) and **Orchestrator** (asynchronous, multi-agent, high-level delegation) — while closing all identified gaps across the SDLC phases: Requirements, Design, Implementation, Testing, Review, Deployment, and Maintenance.

---

## 2. SDLC Theory vs. specd v0.1.0: Complete Gap Analysis

### 2.1 Requirements & Planning
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| AI-assisted requirements refinement | Partial | Medium | `specd` enforces EARS syntax but does not use AI to generate user stories from product briefs, identify edge cases, or produce API schemas from natural language. |
| Specs as eval criteria | Missing | High | Requirements are human-written only. There is no mechanism to convert approved requirements into an automated evaluation rubric for later verification. |
| Interactive prototyping | Missing | Medium | No bridge from natural-language feature description to working prototype before formal spec creation. |

### 2.2 Design & Architecture
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| Architecture trade-off documentation | Partial | Low | `design.md` has 7 mandatory headers, but no structured section for *decision rationale* (why this over that) with AI-generated alternatives. |
| AI scaffolding from architecture docs | Missing | Medium | Once design is approved, AI does not auto-scaffold the codebase structure; agent must manually implement. |
| Guardrails as code | Partial | Medium | Steering files exist but are static text. No executable guardrails (e.g., "never import `crypto/md5`") enforced at gate time. |

### 2.3 Implementation
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| Conductor mode (real-time, in-IDE) | Missing | **Critical** | `specd` is CLI-only. No IDE integration for inline completion, diff review, or keystroke-level guidance. |
| Orchestrator mode (async, multi-agent) | Partial | High | Brain/Pinky exists but is single-project, single-model. No cross-agent delegation via A2A or MCP-based subagent diversity. |
| The 80% problem detection | Missing | **Critical** | No mechanism to detect when a task is likely in the "last 20%" (ambiguous, edge-case heavy) and auto-escalate to human conductor mode. |
| Intelligent model routing | Missing | **Critical** | All tasks flow through the host's default model. No cost-aware routing between frontier (complex) and small (deterministic) models. |

### 2.4 Testing & Quality Assurance
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| Output evaluation (tests) | Strong | Low | `verify:` commands are deterministic tests. Well implemented. |
| Trajectory evaluation (evals) | Missing | **Critical** | No scoring of *how* the agent arrived at the solution: tool use quality, reasoning path, hallucination detection. |
| Continuous quality flywheel | Missing | High | No automated benchmark suite, failure clustering, prompt optimization loop, or regression monitoring. |
| AI-generated test cases | Missing | Medium | Agent can write tests, but `specd` does not explicitly prompt or gate for AI-generated edge-case/property-based tests. |

### 2.5 Code Review & Deployment
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| AI-first code review | Missing | **Critical** | `specd approve` is human-only. No AI pre-review for bugs, style, security, or performance before human judgment. |
| Deployment health monitoring | Missing | **Critical** | `specd` stops at local verification. No deployment pipeline integration, canary analysis, or auto-rollback. |
| AI-aware CI/CD gates | Partial | Medium | GitHub Action exists but only runs `specd check`. No eval gates, no trajectory logging, no cost/quality dashboards. |

### 2.6 Maintenance & Evolution
| Theory Concept | Current specd | Gap Severity | Description |
|---|---|---|---|
| Legacy codebase ingestion | Missing | **Critical** | `specd` is forward-spec only. No mode to ingest, understand, and spec an existing legacy codebase. |
| Automated refactoring & migration | Missing | High | No built-in skills for framework migration, API modernization, or systematic tech-debt reduction. |
| Production feedback loops | Missing | **Critical** | No observability integration. Production errors do not auto-generate `mid-requirements.md` or new specs. |

---

## 3. Common Areas: What specd Already Nails

1. **The Foundational Split** — Agent reasons, harness enforces. This is the core DNA of `specd`.
2. **Specs as Source of Truth** — Markdown on disk, not context window. Best-in-class.
3. **Evidence-Gated Completion** — `verify:` + exit-code recording. Industry-leading rigor.
4. **Human Gates at Phase Boundaries** — `specd approve` ratchet. Essential discipline.
5. **Agent-Agnostic Interface** — MCP + CLI. Avoids vendor lock-in.
6. **Deterministic Reporting** — `state.json` → reports. No LLM hallucination in status.
7. **Context Engineering (Static)** — Steering files (`product.md`, `tech.md`, `structure.md`).
8. **Context Engineering (Dynamic)** — Skills with progressive disclosure (`specd-requirements`, `specd-design`, etc.).
9. **Sandboxed Execution** — `bwrap` isolation for `verify:`. Security-conscious.
10. **DAG-Based Planning** — Wave execution, concurrent frontier.

---

## 4. The Dual-Mode Architecture: Conductor vs. Orchestrator

### 4.1 Philosophy
`specd` v0.2.0 must explicitly support both modes as **first-class, switchable workflows** within the same project. The developer fluidly moves between them based on task ambiguity, stakes, and personal preference. The harness must detect which mode is appropriate and adapt its behavior.

### 4.2 Conductor Mode: "Hands-On, Real-Time Direction"

**Characteristics:**
- Real-time, synchronous, in-IDE
- Keystroke-level control
- Immediate feedback
- Single-file or small scope
- Developer always in the loop
- Best for: complex logic, debugging, unfamiliar codebases, the "last 20%"

**specd v0.2.0 Implementation:**

| Component | v0.2.0 Addition |
|---|---|
| **IDE Extension** | Native VS Code / JetBrains / Cursor extension that embeds `specd` state and commands inside the editor. |
| **Inline Spec Lens** | CodeLens above functions showing linked spec task ID, status, and `verify:` result. Click to open `tasks.md`. |
| **Live Diff Review** | As agent generates code, inline diff gutters appear. Developer accepts/rejects per-hunk, not per-file. |
| **Micro-Task Mode** | `specd conductor start <spec>` breaks tasks into sub-minute micro-tasks. Agent suggests one line/function at a time; developer approves each before continuation. |
| **Context Window HUD** | Visual indicator of current static + dynamic context load (tokens used, skills loaded, steering files active). |
| **Hot-Reload Steering** | Edit `.specd/steering/tech.md` → IDE immediately updates agent context without restart. |
| **Breakpoint Integration** | When `verify:` fails, IDE auto-opens debugger at failure point with agent reasoning trace. |
| **Conductor Ledger** | Records every micro-approval, rejection, and correction into `state.json` for later pattern analysis ("developer tends to reject AI error handling"). |

**CLI Commands:**
```bash
specd conductor start my-feature    # Enter real-time mode
specd conductor step                # Agent proposes next micro-change
specd conductor accept              # Accept current proposal
specd conductor reject --reason "..." # Reject with feedback
specd conductor switch orchestrator # Hand off to async mode
```

### 4.3 Orchestrator Mode: "Async, Multi-Agent Delegation"

**Characteristics:**
- Asynchronous, high-level, multi-agent
- Goal-level control
- Delayed feedback
- Multi-file, cross-spec scope
- Developer reviews outcomes, not keystrokes
- Best for: well-defined features, migrations, test generation, routine maintenance

**specd v0.2.0 Implementation:**

| Component | v0.2.0 Addition |
|---|---|
| **Multi-Model Router** | `specd orchestrator` routes tasks by complexity: frontier models (GPT-5, Gemini 2.5) for architecture/requirements; small models (Llama 4B, Qwen 7B) for linting, test generation, doc updates. Cost tracked per wave. |
| **A2A Delegation** | Implement Agent2Agent protocol for cross-agent task handoff. A "Scout" agent (research) can delegate to a "Craftsman" agent (implementation) via A2A, with `specd` as the orchestration hub. |
| **Background Worker Pool** | Brain/Pinky expanded to support worker pools across multiple machines/containers, not just local subagents. |
| **Auto-Escalation** | If a task fails `verify:` twice, or agent confidence score is low, auto-pause orchestration and surface to Conductor mode with full context. |
| **Batch PR Generation** | Orchestrator bundles completed waves into PRs with AI-generated summaries, test evidence, and decision logs. |
| **Nightly Maintenance Agents** | Scheduled orchestrator runs that scan for deprecated APIs, outdated dependencies, or failing CI jobs and auto-generate specs. |

**CLI Commands:**
```bash
specd orchestrator start my-feature --models mixed --budget $5.00
specd orchestrator status             # View worker pool + queue
specd orchestrator escalate T3        # Force conductor handoff
specd orchestrator schedule --cron "0 2 * * *" --skill specd-maintenance
```

### 4.4 Mode Switching & Unified State

Both modes share the **same `state.json` and spec artifacts**. Switching modes is a state transition, not a context switch:

```
Conductor ──► Orchestrator: Developer marks "this task is well-defined, delegate it"
Orchestrator ──► Conductor: Auto-escalation on failure, ambiguity, or low confidence
```

**Unified Dashboard:**
```bash
specd dashboard --mode unified      # Shows both conductor micro-tasks and orchestrator waves
```

---

## 5. v0.2.0 Roadmap: Closing All SDLC Gaps

### Phase 1: Foundation (Weeks 1-4) — "The Harness Expands"

**Goal:** Make `specd` aware of dual modes and introduce the evaluation layer.

| Feature | Priority | Description |
|---|---|---|
| **Eval Framework (`specd eval`)** | P0 | New command suite. Trajectory evals: log every tool call, reasoning step, and file edit. Score against rubrics ( hallucination, tool misuse, security smell). Output evals: already covered by `verify:`. Store eval results in `state.json`. |
| **Eval Rubric DSL** | P0 | Allow `.specd/evals/<name>.yml` defining scoring criteria (e.g., `no_hardcoded_secrets: 10pts`, `proper_error_handling: 5pts`). Run via `specd eval my-feature --suite security`. |
| **LM Judge Integration** | P1 | Optional integration with LM judge (local or API) for semantic quality scoring of agent outputs. Disabled by default to keep determinism. |
| **Model Router Core** | P0 | Internal `router` package. Config in `.specd/config.yml`: `models.tiers: {frontier: [gpt-5, gemini-2.5], fast: [qwen-7b, llama-4b]}`. Rules map task `role` + `contract` complexity to tier. |
| **Token Economics Ledger** | P1 | Extend telemetry to track cost per task, per wave, per spec. `specd report --cost` shows CapEx vs OpEx breakdown. |
| **Executable Guardrails** | P1 | `.specd/guardrails.yml`: YAML list of forbidden patterns (imports, regexes, file paths). `specd check` enforces deterministically before any agent runs. |

### Phase 2: Conductor Mode (Weeks 5-8) — "Real-Time Control"

**Goal:** Build the synchronous, in-IDE experience for the "last 20%."

| Feature | Priority | Description |
|---|---|---|
| **VS Code Extension** | P0 | Extension that embeds `specd` CLI. Tree view of specs, tasks, and waves. Inline task status badges. |
| **LSP Integration** | P1 | Lightweight LSP server that exposes spec metadata to IDEs: hover to see linked requirement, go-to-definition from task ID to `tasks.md`. |
| **Micro-Task Protocol** | P0 | Extend `tasks.md` schema with `micro_tasks:` under each task. Conductor mode iterates through micro-tasks with human approval at each step. |
| **Live Context HUD** | P1 | Panel showing loaded steering files, active skills, token count, and model in use. |
| **Inline Diff & Accept** | P0 | Agent edits appear as inline diffs. Developer clicks `Accept` / `Reject` / `Modify` per hunk. Rejections are logged as training signal. |
| **Conductor Session Replay** | P1 | `specd conductor replay` reconstructs a conductor session from ledger for audit or onboarding. |

### Phase 3: Orchestrator Mode (Weeks 9-12) — "Autonomous Scale"

**Goal:** Make async multi-agent delegation production-grade.

| Feature | Priority | Description |
|---|---|---|
| **A2A Protocol Support** | P0 | Implement Google A2A protocol for cross-agent communication. `specd` acts as the orchestration hub routing tasks between specialized agents. |
| **MCP Server Marketplace** | P1 | Curated registry of MCP servers (security scanner, dependency checker, performance profiler) that orchestrator can auto-install and route to. |
| **Worker Pool Scaling** | P1 | Brain/Pinky supports remote workers via Redis/Postgres backends (already behind build tags). Enable distributed task execution. |
| **Auto-Escalation Engine** | P0 | Rules: `if verify_fail_count > 2 then escalate; if task_contract_complexity > threshold then use frontier model; if agent_confidence < 0.7 then pause`. Confidence derived from eval scores. |
| **Batch PR Workflow** | P1 | `specd orchestrator submit --bundle wave-1,wave-2` generates a PR with deterministic summary, eval evidence, and decision links. |
| **Scheduled Maintenance** | P2 | `specd schedule --skill specd-maintenance` runs nightly scans. Auto-generates specs for tech debt found. |

### Phase 4: Review & Security (Weeks 13-16) — "Trust but Verify"

**Goal:** Close the review and security gaps.

| Feature | Priority | Description |
|---|---|---|
| **AI Review Agent** | P0 | New role: `reviewer`. Before human `approve`, `specd review my-feature` runs an AI agent with `reviewer.md` role to check for bugs, style, security, and hallucinated dependencies. Output: `review_report.md` with findings. |
| **Security Gate Suite** | P0 | Built-in gates: `secrets` (gitleaks-style), `dependencies` (known CVE scan), `injection` (SQL/XSS pattern detection), `slopsquatting` (package name similarity). Run in `specd check --security`. |
| **Review Checklist Generator** | P1 | From `design.md` and `tasks.md`, auto-generate a human review checklist ("Check error handling in contract X", "Verify file Y matches architecture Z"). |
| **Deterministic PR Summary** | P1 | `specd report --pr-summary` already exists; enhance to include eval scores, security gate results, and cost metrics. |

### Phase 5: Deployment & Maintenance (Weeks 17-20) — "Close the Loop"

**Goal:** Connect `specd` to production and legacy codebases.

| Feature | Priority | Description |
|---|---|---|
| **Deployment Integration** | P0 | `specd deploy` command family. Pluggable backends: GitHub Actions, GitLab CI, ArgoCD, Kubernetes. `specd deploy preview` creates preview env; `specd deploy production` with canary. |
| **Production Observability Hook** | P0 | Webhook endpoint (`specd observe --listen`) receives production error traces. Auto-correlates to spec tasks and generates `mid-requirements.md` or new bug specs. |
| **Legacy Ingestion Mode** | P0 | `specd ingest --path ./legacy-module` reverse-engineers a codebase module into a spec: generates `requirements.md` from code behavior, `design.md` from architecture, `tasks.md` for refactoring. |
| **Auto-Refactor Skills** | P1 | Pre-built skills: `specd-migrate-react`, `specd-upgrade-go`, `specd-modernize-tests`. Orchestrator runs these as scheduled or on-demand jobs. |
| **Feedback Flywheel** | P1 | Production errors → auto-spec → orchestrator implements → eval + review → deploy → monitor. Closed loop. |

### Phase 6: Platform & Ecosystem (Weeks 21-24) — "The Factory Model"

**Goal:** Make `specd` the system that builds systems.

| Feature | Priority | Description |
|---|---|---|
| **Spec Pack Registry** | P1 | Public registry of `specd init --pack` templates for common stacks (Next.js + Prisma, Go + gRPC, Rust + Axum). |
| **Team Harness Sharing** | P0 | `.specd/harness/` directory for reusable eval suites, guardrails, and model router configs. Versioned and shared across repos via Git submodules or package manager. |
| **Organization Dashboard** | P2 | Web UI (`specd dashboard --org`) showing all team specs, cost per project, eval trends, and agent failure mode clustering. |
| **Certified Agent Partners** | P2 | Compliance program: agents that pass `specd` eval suites get "certified" badge in registry. Ensures quality across the ecosystem. |

---

## 6. Core Philosophy Alignment: Every v0.2.0 Feature Mapped

| Principle | v0.2.0 Enforcement |
|---|---|
| **1. The Foundational Split** | Conductor mode keeps human as the reasoning filter; Orchestrator mode keeps human as the goal-setter. Harness enforces in both. |
| **2. Specs as Source of Truth** | All modes mutate only via CLI. IDE extensions are thin wrappers over CLI. Legacy ingestion auto-generates specs, never skips them. |
| **3. Evidence Gates Every State Change** | Evals + tests + security gates + review reports = multi-layer evidence. No phase advances without recorded proof. |
| **4. Waves, Not Lines** | Orchestrator waves remain DAG-based. Conductor micro-tasks are sub-DAGs within a task. Both render as waves in reporting. |
| **5. Agent-Agnostic by Design** | Model router supports any OpenAI-compatible API. A2A support means any A2A-compliant agent can participate. IDE extension is host-agnostic. |
| **6. Human Gates at Phase Boundaries** | Conductor mode adds *micro-gates* (per hunk approval). Orchestrator keeps *macro-gates* (per wave approval). Both require human judgment at boundaries. |
| **7. Deterministic Reporting** | Eval scores, security gate results, and cost ledgers are all deterministic data in `state.json`. Reports remain LLM-free. |
| **8. Steering as Constitution** | Guardrails become executable extensions of steering. Model router configs are versioned steering. Both outlive sessions. |

---

## 7. Data Model Changes for v0.2.0

### 7.1 `state.json` Extensions

```json
{
  "version": 2,
  "mode": "orchestrator",
  "evals": {
    "trajectory_score": 0.92,
    "output_score": 1.0,
    "security_score": 0.85,
    "eval_suite": "default",
    "last_run": "2026-07-02T10:00:00Z"
  },
  "routing": {
    "model_tier": "frontier",
    "model_name": "gpt-5",
    "cost_usd": 0.42,
    "tokens_in": 12000,
    "tokens_out": 3400
  },
  "conductor": {
    "session_id": "cond-abc123",
    "micro_task_index": 3,
    "ledger": [
      {"action": "propose", "time": "...", "accepted": true},
      {"action": "propose", "time": "...", "accepted": false, "reason": "incorrect error handling"}
    ]
  },
  "orchestrator": {
    "worker_pool": ["pinky-1", "pinky-2"],
    "escalation_reason": null,
    "scheduled": false
  }
}
```

### 7.2 New Artifacts

| Artifact | Purpose |
|---|---|
| `.specd/evals/<suite>.yml` | Eval rubric definitions |
| `.specd/guardrails.yml` | Executable constraints |
| `.specd/reviews/<spec>-<timestamp>.md` | AI + human review reports |
| `.specd/deploy/<env>.yml` | Deployment configuration per environment |
| `.specd/observability/webhooks.yml` | Production webhook endpoints |
| `.specd/ingested/` | Legacy ingestion output specs |

---

## 8. Command Reference Additions (v0.2.0)

```bash
# === EVAL ===
specd eval <spec> [--suite <name>] [--trajectory] [--output]
specd eval init <spec> --template default    # Scaffold eval suite

# === CONDUCTOR ===
specd conductor start <spec> [--ide vscode|cursor|jetbrains]
specd conductor step
specd conductor accept [--hunk <id>]
specd conductor reject --reason "..."
specd conductor switch orchestrator

# === ORCHESTRATOR ===
specd orchestrator start <spec> [--models mixed|frontier|fast] [--budget <usd>]
specd orchestrator status
specd orchestrator escalate <task> --reason "..."
specd orchestrator schedule --cron "..." --skill <name>
specd orchestrator submit --bundle <waves>

# === REVIEW ===
specd review <spec> [--ai-only] [--human-only]
specd review report <spec> --format md|html

# === SECURITY ===
specd check <spec> --security    # Run all security gates
specd security scan <path>       # Standalone security scan

# === DEPLOY ===
specd deploy preview <spec>
specd deploy production <spec> --strategy canary|blue-green
specd deploy rollback <spec>

# === INGEST ===
specd ingest --path <dir> --out <spec-slug> [--framework auto]
specd ingest analyze <path>      # Print what would be generated

# === OBSERVE ===
specd observe --listen <port>    # Start webhook listener
specd observe correlate <error-id>  # Link error to spec task

# === DASHBOARD ===
specd dashboard --mode unified|conductor|orchestrator
specd dashboard --serve --port 8080
specd dashboard --org <team-id>    # Organization view
```

---

## 9. Migration Path from v0.1.0

1. **Backward Compatibility:** All v0.1.0 commands remain unchanged. New features are additive.
2. **State Migration:** `specd init --migrate` auto-upgrades `state.json` to version 2 schema.
3. **Config Migration:** `specd init --migrate` converts `config.json` to `config.yml` with new sections (`evals`, `routing`, `orchestrator`).
4. **Steering Enhancement:** Existing steering files are preserved. New executable guardrails are opt-in via `specd init --guardrails`.
5. **IDE Onboarding:** `specd init --ide vscode` scaffolds extension config and installs marketplace extension.

---

## 10. Success Metrics for v0.2.0

| Metric | Target | Measurement |
|---|---|---|
| First-pass verify success rate | >85% | Via model routing + eval feedback |
| Security gate catch rate | >90% | Synthetic vulnerable code injection tests |
| Conductor → Orchestrator handoff friction | <30s | Time to switch modes without context loss |
| Legacy ingestion accuracy | >80% | Human-rated spec fidelity against known codebase |
| Cost per feature (orchestrator) | 40% lower | Compared to v0.1.0 single-model approach |
| Production defect correlation | 100% | Every production error linked to originating spec |
| Eval suite coverage | 100% | Every shipped spec has trajectory + output eval |

---

## 11. Conclusion

`specd` v0.1.0 is the best-in-class harness for **disciplined spec-driven development**. v0.2.0 evolves it into a **complete agentic engineering platform** that:

1. **Implements the full AI-driven SDLC** from requirements ingestion to production observability.
2. **Embodies dual-mode fluency** — Conductor for the last 20%, Orchestrator for the first 80%.
3. **Closes every identified gap** — evals, model routing, security, review, deployment, maintenance.
4. **Preserves core philosophy** — the agent reasons, the harness enforces, evidence gates everything, and specs remain the source of truth.

The future of software engineering is not choosing between human expertise and AI capability. It is designing systems where both contribute their unique strengths. `specd` v0.2.0 is that system.

---

*End of Plan*
