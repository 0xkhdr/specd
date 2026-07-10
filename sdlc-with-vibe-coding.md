# Comparative Analysis: `specd` vs. Google's "The New SDLC With Vibe Coding"

Source basis:
- `specd` local repository at `/var/www/html/rai/up/specd`, inspected July 10, 2026.
- Google paper text extracted in the previous version of this file. The binary PDF was not present in the workspace; paper citations therefore use extracted line references plus the paper's own table-of-contents page markers.
- The request mentions `src/cli.ts`; this repository is Go, not TypeScript. The equivalent implementation surface is `main.go`, `internal/cli`, `internal/cmd`, `internal/core`, `internal/core/gates`, `internal/orchestration`, `internal/context`, and `internal/mcp`.

Copyright note: the paper is cited with short evidence phrases and line references, not long quotations.

## Executive Summary

- `specd` strongly embodies the paper's central move from vibe coding to agentic engineering: it turns informal prompting into a gated lifecycle of requirements, design, tasks, evidence, and approvals.
- Its strongest match is harness engineering. `specd` is not a model; it is the deterministic machinery around models: roles, context manifests, gates, state, verification, locks, MCP exposure, orchestration, and reports.
- Its critical gap is eval maturity. The paper says production agentic engineering needs deterministic tests and non-deterministic evals, including trajectory and rubric-based assessment. `specd verify` currently records shell-command exit evidence; that is rigorous for tests but not sufficient for LM judges, trajectory scoring, or qualitative rubrics.
- A realistic positioning is: `specd` covers roughly 70% of the paper's production-grade SDLC framework for coding-agent work, but closer to 55-60% for production autonomous-agent systems because it lacks first-class evals, model routing, cost metering, deployment integration, and A2A-native multi-agent operation.

## Phase 1: Comparative Source Ingestion Findings

This section is intentionally comparative: it records the ingestion details only where they change the mapping between `specd` and the paper.

| Ingestion Item | Comparative Finding |
|---|---|
| Repository language and CLI surface | The prompt asks for `src/cli.ts`, but `specd` is Go. This matters because the paper's "harness" is implemented as a static CLI binary, not a TypeScript agent framework. Entry flows from `main.go` to `internal/cli` to `internal/cmd`; domain enforcement lives in `internal/core`. |
| Lifecycle | The requested lifecycle `requirements -> design -> tasks -> executing -> verifying -> complete` maps to `specd`'s ratchet, but the code distinguishes analysis/planning/execution phases and task evidence states. The paper describes compressed SDLC phases; `specd` deliberately slows them into approval gates. |
| Core principles | `specd`'s practical principles map to the paper's production discipline: deterministic enforcement, evidence integrity, planning ratchet, role separation, bounded context, DAG/frontier execution, atomic state/locking, and subtractive scope control. These are implemented across `docs/concepts.md`, `docs/contributor-guide.md`, and `internal/core`. |
| Validation gates | The prompt's "7 validation gates" is stale for the current repo. The current docs/code include a broader gate set: EARS requirements, design structure, task schema, acyclic DAG/frontier, evidence, sync, context budget, approval, review, and opt-in security gates. This is stronger alignment with the paper's harness/guardrail model than a seven-gate system would be. |
| Steering constitution | `.specd/steering/*.md` and scaffolded `AGENTS.md` correspond to the paper's static context: rule files, persona definitions, project memory, architectural constraints, and team conventions. |
| Role personas | `scout`, `craftsman`, `validator`, and `auditor` map to the paper's conductor/orchestrator boundary. They encode allowed authority per task so a delegated agent does not silently expand scope. |
| Evidence-gated completion | `specd` implements a strict test-style completion contract: a task is complete only with passing evidence tied to a git HEAD. This is stronger than vibe coding but narrower than the paper's eval-plus-test requirement. |
| `specd context` | The context command is the clearest implementation of the paper's context-engineering economics: it generates a bounded task manifest instead of dumping a whole repository into every model call. |

## Phase 2: Common Areas Mapping

| Dimension | Google SDLC Paper Concept | `specd` Implementation | Evidence |
|---|---|---|---|
| Process Philosophy | Spectrum from Vibe Coding to Agentic Engineering; structure, verification, and human judgment distinguish production practice. | Planning ratchet and gated lifecycle. `README.md` defines `specd` as a "spec-driven coding harness CLI"; `docs/concepts.md` describes the ratchet: requirements, design, tasks, then evidence-gated execution. | Paper: extracted lines 121-140, p. 11-12 markers. `specd`: `README.md:3-4`; `docs/concepts.md`; `docs/validation-gates.md`. |
| Requirements Phase | AI participates in requirements refinement, edge-case discovery, and prototype feedback-loop compression. | `requirements.md` is the first lifecycle artifact; EARS validation gate checks requirement shape. `internal/core/gates/ears.go` and `docs/validation-gates.md` enforce structured requirements. | Paper: lines 204, 472. `specd`: `docs/user-guide.md` requirements step; `docs/validation-gates.md`; `internal/core/gates/ears.go`. |
| Design Phase | Architecture remains a human-centric decision boundary. | `design.md` is phase-gated and structurally validated; design approval advances only through gates. `internal/cmd/lifecycle.go` and `internal/core/gates/design.go` enforce this. | Paper: lines 196, 287-293, 514. `specd`: `docs/user-guide.md`; `docs/validation-gates.md`; `internal/core/gates/design.go`. |
| Implementation Phase | Coding agents work across codebases, run tools/tests, and iterate under harness constraints. | `tasks.md` decomposes work into atomic tasks with `files`, `dependencies`, `role`, and `verify`; `specd next`, `specd dispatch`, and orchestration use the DAG frontier. | Paper: lines 75, 295-309, 372-374. `specd`: `internal/core/tasksparser.go`; `internal/core/dag.go`; `internal/core/frontier.go`; `internal/cmd/lifecycle.go`; `internal/cmd/dispatch.go`; `internal/orchestration`. |
| Testing/QA | Tests plus evals; output evaluation and trajectory evaluation are both necessary. | `specd verify` records deterministic shell-command evidence, exit code, and git HEAD. This maps to tests and output checks, not full evals or trajectory scoring. | Paper: lines 140, 222-226, 482. `specd`: `internal/core/evidence.go`; `internal/core/verify/exec.go`; `internal/core/task_complete.go`; `docs/concepts.md`; `docs/open-spec-format.md`. |
| Review/Deploy | AI-augmented review, deployment guardrails, deterministic hooks, observability. | Reviewer/auditor roles, `specd check`, opt-in security gate, deterministic reports, and GitHub Action docs exist. No native deployment/rollback pipeline. | Paper: lines 228-234, 311-317, 494. `specd`: `docs/agent-integration.md`; `docs/github-action.md`; `docs/observability.md`; `internal/cmd/review.go`; `internal/core/gates/security`. |
| Maintenance | AI-assisted maintenance, legacy navigation, memory, and feedback loops. | Steering and memory files, `specd memory`, decisions, reports, and cross-spec state support project evolution, but not a complete modernization workflow. | Paper: lines 226, 232, 384-388. `specd`: `.specd/steering/*` templates; `internal/core/memory.go`; `internal/cmd/memory.go`; `docs/concepts.md`; `docs/observability.md`. |
| Harness Engineering | "Agent = Model + Harness"; harness includes prompts, tools, context policies, hooks, sandboxes, sub-agents, observability. | `specd` is the harness around any coding agent: state machine, gate registry, role prompts, steering, context builder, verify execution, locks, dispatch, orchestration, MCP server, and reports. | Paper: lines 258-281, 319-321. `specd`: `README.md:3-4`; `docs/agent-integration.md`; `internal/core/gates/core.go`; `internal/context`; `internal/mcp`; `internal/orchestration`. |
| Context Engineering | Six context types: Instructions, Knowledge, Memory, Examples, Tools, Guardrails; static vs dynamic context; Agent Skills. | Static context: `AGENTS.md`, `.specd/roles`, `.specd/steering`. Dynamic context: `specd context <slug> <task>` builds bounded, cited manifests. Guardrails are gates. Tools are CLI/MCP commands. Skills are not first-class, though role prompts approximate procedural packages. | Paper: lines 142-180, 444-452. `specd`: `docs/agent-integration.md`; `internal/context`; `docs/concepts.md`; `docs/mcp-guide.md`; `.specd/roles`; `.specd/steering`. |
| Developer Roles | Conductor vs Orchestrator. | Conductor mode maps to a human using `specd context`, editing, then `specd verify`. Orchestrator mode maps to `specd dispatch`, `brain` run loops, leases, workers, and role-scoped packets. | Paper: lines 325-349, 360. `specd`: `docs/agent-integration.md`; `internal/cmd/dispatch.go`; `internal/cmd/brain_run.go`; `internal/orchestration/lease.go`; `internal/orchestration/decide.go`. |
| Multi-Agent | MCP for tools, A2A for cross-agent delegation, background agents, shared session state. | `specd mcp` exposes the command palette via MCP; dispatch packets and Pinky workers support delegated execution. A2A is not implemented as a protocol. | Paper: lines 412, 496. `specd`: `docs/mcp-guide.md`; `internal/mcp`; `docs/agent-integration.md`; `internal/cmd/dispatch.go`; `internal/orchestration`. |
| Economics | CapEx vs OpEx; token burn; context engineering as financial lever; intelligent model routing. | `specd` reduces token waste through generated context manifests and bounded budgets, but it does not meter token cost or route models. | Paper: lines 420-458. `specd`: `internal/context`; `internal/core/gates/contextbudget.go`; `docs/observability.md`; `docs/scale-envelope.md`. |

## Phase 3: Adherence Scoring Matrix

Scores: 1 = weak/no support, 3 = partial, 5 = strong production-grade alignment.

| Category | Score | Assessment |
|---|---:|---|
| 1. Structured Intent over Vibe Coding | 5 | `specd` forces intent into `requirements.md`, `design.md`, `tasks.md`, and task-level verify lines. The paper's ideal is that production work needs specifications, tests, guardrails, and human oversight (paper lines 510-514). `specd` is built around exactly that discipline. |
| 2. Harness-Centric Design | 5 | `specd` is a harness, not a model. The paper's formula "Agent = Model + Harness" appears at line 268; `specd` supplies the harness pieces: roles, gates, context, execution state, verify records, dispatch, and MCP. |
| 3. Context Engineering Maturity | 4 | Strong support for instructions, memory, tools, guardrails, and dynamic bounded task context. Weaker support for examples and portable Agent Skills. The paper's six context types are listed at lines 148-153; `specd` implements most through `AGENTS.md`, roles, steering, context manifests, and CLI/MCP tools. |
| 4. Factory Model Alignment | 4 | The paper's factory model makes the developer design the system that produces code (lines 242-254). `specd` matches this through task decomposition, gates, roles, and verify contracts. It loses a point because it manages coding tasks more than the complete factory for deployed agent products. |
| 5. Verification Rigor | 3 | `specd` has rigorous deterministic evidence. It does not have first-class non-deterministic evals, trajectory scoring, labelled datasets, LM judges, or rubrics. The paper explicitly separates tests and evals at lines 140 and 222-226. |
| 6. Phase Compression & Blurring | 3 | `specd` acknowledges and controls phase progression rather than fully embracing blurred phases. Its ratchet is valuable for safety, but less flexible than the paper's view that requirements-to-prototype and implementation-to-review loops compress unevenly (paper lines 196, 204). |
| 7. Human-in-the-Loop Architecture | 5 | `specd approve`, design gates, auditor/validator roles, decisions, and review commands preserve human boundaries. The paper states human judgment remains constant even as implementation compresses (line 196). |
| 8. Multi-Agent Orchestration | 3 | `specd` supports dispatch, leases, workers, frontier waves, and MCP. It does not support A2A-native cross-agent delegation or model-specific routing. Paper lines 341-349 and 412 define a broader target. |
| 9. Economic Sustainability | 3 | Bounded context and deterministic reporting reduce token burn, matching the paper's context-as-financial-lever argument (lines 444-452). Missing: token/cost metering, latency budgets, model routing, and cost-aware orchestration. |
| 10. Production Readiness | 4 | For coding-agent harnessing, `specd` is production-minded: deterministic gates, CAS state, locks, zero runtime dependencies, evidence, CI docs, and reports. For production autonomous agents at scale, it lacks eval, deployment, observability, and governance depth described at lines 384-388 and 494. |

Weighted conclusion: `specd` is a strong implementation of the paper's agentic-engineering discipline for source-code delivery, but an incomplete implementation of the paper's broader production-agent operating model.

## Phase 4: Gap Analysis

### Category A: Conceptual Gaps

#### A1. First-class eval theory and rubric design

- Paper requirement: evals should assess task success, tool-use quality, trajectory compliance, hallucination, and response quality; see extracted lines 140, 222-226, 480-482.
- `specd` today: `verify:` is a shell command with exit-code evidence, recorded by `internal/core/verify/exec.go` and `internal/core/evidence.go`.
- Why it matters: deterministic tests catch many code failures, but they do not prove that an agent used allowed tools, followed a safe trajectory, or met qualitative review rubrics.
- Recommendation: add `evals.md` or `eval:` blocks to task schema. Support deterministic shell evals first, then JSON-rubric evals, then optional LM-judge adapters with labelled datasets and stored scoring artifacts.

#### A2. Intelligent model routing

- Paper requirement: use larger models for architecture and complex implementation, smaller models for tests, review, and CI monitoring; see lines 454-458.
- `specd` today: agent/model selection is external to the harness. Dispatch packets can define role and context, not model policy.
- Why it matters: without routing, teams either overpay for simple work or underpower high-risk reasoning tasks.
- Recommendation: add a `models.toml` policy file and role/task complexity routing metadata. Keep deterministic selection outside gate logic, but record chosen model/provider in evidence and reports.

#### A3. A2A-native inter-agent protocol

- Paper requirement: MCP and A2A are emerging connective tissue for multi-agent systems; see lines 412 and 496.
- `specd` today: MCP exists for command palette exposure; delegation uses `specd` dispatch and internal worker protocols.
- Why it matters: proprietary delegation makes cross-vendor and cross-harness orchestration harder.
- Recommendation: keep current dispatch packets, but add A2A-compatible envelope import/export and map `role`, `task`, `context`, `authority`, and `evidence_ref` into the protocol.

### Category B: Implementation Gaps

#### B1. Evals vs tests: `verify:` is necessary but insufficient

- Paper requirement: tests and evals together define agentic engineering; see lines 140 and 472.
- `specd` today: verify evidence is a command result pinned to a git HEAD. This is excellent for "does it pass?" and weaker for "was the agent's path acceptable?"
- Why it matters: the 80% problem often hides in edge cases, integration assumptions, or skipped verification steps.
- Recommendation: split completion evidence into `test_evidence`, `eval_evidence`, and `trajectory_evidence`. Allow a task to require one or more evidence classes.

#### B2. Observability and tracing

- Paper requirement: logs, traces, evaluations, cost and latency metering; see lines 281, 317, 494.
- `specd` today: `docs/observability.md` documents deterministic local event/report concepts; state and evidence live in `.specd`. There is no full trace viewer, token-cost accounting, or provider latency telemetry.
- Why it matters: teams cannot optimize, audit, or detect drift in production agent workflows without run-level traces and cost data.
- Recommendation: add a run ledger schema with spans: context build, model call, tool call, file edit, verify run, eval run, approval. Export OpenTelemetry JSON and HTML reports.

#### B3. Security guardrails are opt-in and narrow

- Paper requirement: deterministic hooks should block unsafe actions such as hard-coded secrets; see lines 280 and 317.
- `specd` today: `internal/core/gates/security` exists and docs describe opt-in security gates. Structural gates are stronger than security gates.
- Why it matters: generated code can introduce secrets, dependency confusion, injection risks, and unsafe permission changes.
- Recommendation: promote a baseline security profile into default checks: secret scan, dependency diff scan, dangerous shell-pattern detection, generated-code permission audit, and task-scope file-write enforcement.

#### B4. Deployment and rollback integration

- Paper requirement: deployment pipelines become AI-aware, including monitoring, rollback, and risk prediction; see lines 232 and 494.
- `specd` today: GitHub Action docs and reports exist, but deployment is outside scope.
- Why it matters: agentic engineering does not end at merge; production feedback is part of the loop.
- Recommendation: define optional deployment adapters: `deploy.verify`, `deploy.observe`, `deploy.rollback`, with evidence records tied to release IDs.

#### B5. Long-running maintenance and modernization

- Paper requirement: production agents need persistent memory, governance, observability, and refinement loops; see lines 384-388 and 414.
- `specd` today: supports decisions, memory promotion, reports, steering files, and multiple specs, but lifecycle is centered on a single spec reaching completion.
- Why it matters: real codebases evolve through recurring migrations, incidents, regressions, and architectural drift.
- Recommendation: add portfolio-level views: cross-spec dependency graph, recurring invariant checks, drift reports, architecture-decision aging, and maintenance task generation from incidents.

#### B6. The 80% problem is only partially addressed

- Paper requirement: edge cases, integration points, error handling, and subtle correctness dominate the final 20%; see lines 352-360.
- `specd` today: task-level files, dependencies, acceptance, and verify commands reduce the risk, but only as much as the human-authored tests and gates cover.
- Why it matters: evidence-gated completion can still certify an incomplete concept if the verify line is shallow.
- Recommendation: add verify-quality lint: require edge-case tests for high-risk tasks, mutation/property-test hooks where applicable, and reviewer rubrics for "what would break in production?"

### Category C: Scope Gaps

#### C1. Production autonomous-agent runtime

- Paper requirement: agents serving real users need deployment infrastructure, scoped permissions, governance, and observability; see lines 384-390.
- `specd` today: a coding harness CLI for generating and governing code, not a runtime for deployed AI agents.
- Why it matters: the paper spans both building software with agents and building agents as software products.
- Recommendation: do not turn `specd` into an agent runtime. Instead, integrate with runtimes through adapters and evidence contracts.

#### C2. Organization-level workforce and operating-model change

- Paper requirement: hiring, team norms, on-call, and hybrid human-agent teams evolve; see lines 498-500.
- `specd` today: provides project-level mechanics and docs, not org design.
- Why it matters: the paper's adoption model includes sociotechnical practices beyond CLI enforcement.
- Recommendation: add templates for team policies, review ownership, role assignment, and production readiness checklists.

#### C3. Full skill marketplace / portable Agent Skills

- Paper requirement: skills are portable procedural knowledge loaded on demand; see lines 165-178 and 468.
- `specd` today: role prompts and steering files are close, but there is no portable skill package system.
- Why it matters: skill reuse is a major compounding asset in the paper's economics.
- Recommendation: add `.specd/skills/<name>/skill.md`, metadata, triggers, and context-builder support. Keep skills file-based and vendor-neutral.

## Specific Probe Findings

- Evals vs tests: `specd` mostly covers tests. It does not natively implement LM judges, labelled datasets, trajectory scoring, or rubric scoring.
- Model routing: absent. Model choice is outside `specd`.
- Observability/tracing: partial. Deterministic reports and ledgers exist, but not cost/latency metering or complete trace spans.
- Security guardrails: partial. Opt-in security gates exist; default production-grade security policy is not comprehensive.
- Deployment integration: partial-to-absent. CI documentation exists; deploy/rollback control does not.
- Maintenance/evolution: partial. Memory, decisions, and specs help, but portfolio-scale modernization is not first-class.
- 80% problem: partially mitigated by gates and evidence; still depends heavily on test/eval quality authored by humans.
- A2A protocol: absent. MCP exists; A2A does not.
- LM judge integration: absent from verification pipeline.

## Phase 5: Synthesis and Strategic Positioning

### 1. Completeness as a reference implementation

`specd` is approximately 70% complete as a reference implementation of the paper's production-grade workflow for coding agents. It covers the core discipline: structured intent, harness design, context management, deterministic gates, role boundaries, evidence, and human approval.

It is not a complete reference implementation of the paper's entire AI-driven SDLC. For that broader scope, including autonomous-agent products, production observability, eval suites, model routing, deployment, A2A, and cost optimization, it is closer to 55-60%.

### 2. Strongest alignment

Harness engineering is the strongest alignment. The paper says the model is only one input and the surrounding machinery determines whether agentic work is reliable. `specd` operationalizes that with a deterministic CLI harness: gate registry, phase ratchet, context manifests, roles, locks, CAS state, verify evidence, MCP exposure, and orchestration.

### 3. Critical blind spot

The critical blind spot is first-class evals. `specd` can prove that a command passed at a git HEAD, but the paper's agentic-engineering threshold requires trajectory and output evals with rubrics. Without that, `specd` can certify "passed tests" while missing "agent behaved correctly and met qualitative intent."

### 4. Who should use `specd`

- Individual developers: strong fit for disciplined feature work, refactors, and agent-assisted implementation where unmanaged vibe coding would be risky.
- Team leads: strong fit for standardizing specs, task contracts, review boundaries, and evidence before scaling coding-agent usage.
- Organizations: useful as a foundation, but should add CI policy, eval infrastructure, security scanning, observability, and deployment integration before treating it as the whole production substrate.
- Prototypes: may feel heavier than necessary unless the prototype has safety, compliance, or future-production intent.
- Production code: appropriate when paired with strong tests, review, security checks, and CI/CD.
- Production autonomous agents: not sufficient alone; use as a build harness, not the runtime/governance platform.

## Recommended `specd v2.0` Roadmap

### P0: Close the production-confidence gap

1. Add first-class eval artifacts.
   - New files: `.specd/specs/<slug>/evals.md` or task-level `eval:` blocks.
   - Schema: `kind`, `dataset`, `rubric`, `runner`, `threshold`, `required`.
   - Evidence: `eval_evidence.jsonl` with score, rubric version, dataset hash, runner, model, git HEAD.

2. Add trajectory evidence.
   - Record tool-call sequence, commands, file touches, verify attempts, failures, and retries.
   - Gate task completion on required trajectory invariants: verify was run, no undeclared files touched, no forbidden tools used.

3. Add verify-quality lint.
   - Reject empty/shallow verify lines for write tasks.
   - Require high-risk tasks to include edge-case or integration checks.
   - Warn when `verify:` only checks formatting for behavior-changing tasks.

4. Make baseline security gates default.
   - Secret detection, dependency-risk diffing, injection-sensitive pattern scan, unsafe permission changes, generated shell command review.

### P1: Add economic and operational substrate

1. Add model routing policy.
   - `.specd/models.toml` with role/task routing rules.
   - Record chosen model/provider and estimated/actual token cost in run evidence.

2. Add OpenTelemetry-compatible traces.
   - Span types: context, model, tool, edit, verify, eval, approval, dispatch, deployment.
   - Export JSONL and HTML trace views.

3. Add deployment evidence adapters.
   - Optional `deploy:` blocks with verify/observe/rollback commands.
   - Release evidence tied to git SHA, environment, health checks, and rollback status.

4. Add CI-native eval/report gates.
   - GitHub Action that fails on missing required evals, failing eval thresholds, or unreviewed security findings.

### P2: Build multi-agent and organizational scale

1. Add A2A-compatible dispatch envelopes.
   - Preserve current deterministic dispatch, but provide A2A import/export.

2. Add file-based Agent Skills.
   - `.specd/skills/<name>/SKILL.md`, metadata, trigger rules, references, and context-builder progressive disclosure.

3. Add portfolio governance.
   - Cross-spec dashboards, recurring invariants, aging decisions, maintenance waves, drift detection, and incident-to-task generation.

4. Add team operating templates.
   - Role ownership, review SLAs, production readiness, on-call handoff, generated-code policy, and security exception workflow.

## Appendix: Key Evidence Index

### Google Paper Evidence

- Spectrum and agentic-engineering boundary: extracted lines 79-83, 121-140; TOC p. 11-12 markers.
- Context engineering six types: lines 142-153; TOC p. 12 marker.
- Static vs dynamic context: lines 157-167.
- AI-driven SDLC phase compression: lines 190-204.
- Testing, output eval, trajectory eval: lines 222-226.
- Factory model: lines 242-258; TOC p. 24 marker.
- Harness anatomy: lines 258-281; TOC p. 26 marker.
- Harness in SDLC: lines 287-319.
- Conductor/orchestrator: lines 325-349; TOC p. 30-32 markers.
- 80% problem: lines 352-360; TOC p. 33 marker.
- Production agents, deployment, observability: lines 384-414.
- Economics, token burn, CapEx/OpEx: lines 420-458; TOC p. 39-42 markers.
- Team adoption recommendations: lines 468-500.
- Final principle on structure: lines 510-514.

Short evidence phrases used:
- "Agent = Model + Harness" (line 268).
- "Structure scales, vibes don't" (line 510).
- "The Token Burn Rate" (line 432).

### `specd` Evidence

- `README.md:3-4`: defines `specd` as a spec-driven coding harness and names the lifecycle: requirements, design, tasks, evidence-gated execution.
- `README.md:82`: Go implementation and zero runtime dependencies.
- `docs/user-guide.md`: user lifecycle: init, requirements, design, tasks, next/context/verify/check/approve.
- `docs/concepts.md`: planning ratchet, evidence integrity, frontier, deterministic reports, context for agents.
- `docs/agent-integration.md`: agent-agnostic model, roles, context command, dispatch/Pinky worker integration, evidence reporting.
- `docs/contributor-guide.md`: architecture map: `main.go` to `internal/cli` to `internal/cmd`; core invariants; zero runtime dependencies; reference directory warning.
- `docs/validation-gates.md`: gate registry and semantics, including EARS, design, task schema, DAG, evidence, sync, context budget, approval, and opt-in security.
- `docs/open-spec-format.md`: task schema fields and evidence storage shape.
- `docs/observability.md`: deterministic local observability/reporting scope.
- `docs/mcp-guide.md`: MCP server exposure for `specd` commands.
- `docs/github-action.md`: CI integration path.
- `internal/core/gates/core.go`: central gate registry.
- `internal/core/gates/ears.go`: EARS requirement validation.
- `internal/core/gates/design.go`: design structure validation.
- `internal/core/gates/tasks.go`: task schema validation.
- `internal/core/gates/evidence.go`: evidence gate.
- `internal/core/gates/contextbudget.go`: context budget gate.
- `internal/core/gates/security/*`: opt-in security gates.
- `internal/core/state.go` and `internal/core/phases.go`: state machine and phase definitions.
- `internal/core/tasksparser.go`: byte-stable task parser.
- `internal/core/dag.go` and `internal/core/frontier.go`: task DAG and frontier computation.
- `internal/core/evidence.go`, `internal/core/task_complete.go`, `internal/core/verify/exec.go`: evidence and verify mechanics.
- `internal/context`: bounded, cited context manifest generation.
- `internal/cmd/lifecycle.go`: lifecycle command handling.
- `internal/cmd/dispatch.go`, `internal/cmd/brain_run.go`, `internal/orchestration`: dispatch/orchestration and leases.
- `internal/mcp`: MCP command palette server.
