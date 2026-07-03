# specd — Fresh Start Brief & Claude Code Analysis Plan

> **Purpose of this document.** Two things in one file:
> 1. **Part A — Foundational analysis** of the *current* `specd` at its present position: its core philosophy, use cases, the reason a deterministic agent-agnostic layer must exist, and an honest read of where the codebase has accreted redundancy.
> 2. **Part B — The instruction set for Claude Code**: a domain-by-domain plan telling Claude Code exactly how to run *its own* analysis, which `specd` files to learn from, what to optimize, and — critically — that every domain analysis it writes is a **precursor artifact** that will be translated into a `spec.md` + `tasks.md` and executed in waves to build the fresh version.
>
> **North star.** Rebuild `specd` to its *final-form* value with the **minimal accurate path** — keep the core thesis, drop the accretion, and ground every decision in *The New SDLC with Vibe Coding* (Agent = Model + Harness) and context-engineering foundations.

---

## Part A — Foundational Analysis of specd (current position)

### A.1 The one-sentence thesis

> **The agent reasons. The harness enforces.**

`specd` is an **agent-agnostic, spec-driven coding harness CLI**. It moves process integrity *off* the LLM's non-deterministic context window and *onto* a strict, local, tool-gated pipeline written in dependency-free Go, shipped as a single static binary.

This is a direct, concrete implementation of the paper's central equation:

```
Agent = Model + Harness
```

The paper argues the model is ~10% of what determines agent behavior; the **harness** — instructions/rule files, tools, sandboxes, orchestration logic, guardrails/hooks, observability, and context policy — is the other ~90%, and it is *the team's* surface area, not the model provider's. `specd` **is** that harness, expressed as a CLI that any coding agent (Claude Code, Cursor, Codex, Aider, or any MCP client) can drive.

### A.2 Why a *deterministic, agent-agnostic* layer must exist

The paper names the failure modes precisely; `specd` answers each with a deterministic gate rather than a prompt:

| Paper's failure mode | Why the model can't self-police it | specd's deterministic answer |
|---|---|---|
| **"80% problem"** — model nails 80%, fumbles edge cases, integration, subtle correctness | Correctness lives outside the token stream | **Evidence gates**: a task completes only against a *passing `verify` record* (exit code + git HEAD), never a free-text claim |
| **Context stuffed with noise** → drift | The model cannot reliably budget its own context | **Context manifest engine** (`BuildContextManifest`) emits a budgeted, token-measured load list |
| **Config failures blamed on the model** | Non-determinism hides process breaks | **7 core validation gates** (`specd check`) run as pure functions over on-disk state |
| **"Trust me, it works"** | LLMs assert success | **Trust is recorded, not assumed** — status changes require verifiable proof |
| **Vibe coding = implicit scaffolding** | Casual prompting has no structure | **Spec artifacts on disk** (requirements → design → tasks) are the source of truth, versioned in git |
| **Agent-specific lock-in** | Every host has its own workflow | **Standardized CLI + MCP interface**; role-prompt injection makes it host-agnostic |

The key insight the fresh start must preserve: **determinism is the product.** The value is not "another AI wrapper" — it is a *reproducible* boundary where the same inputs yield the same gates, the same waves, the same reports, with zero LLM calls inside the harness itself.

### A.3 The eight principles (the invariants to carry forward)

1. **The Foundational Split** — agent creates; harness enforces.
2. **Specs as the Source of Truth** — plan lives as versioned Markdown on disk, not in context.
3. **Evidence Gates Every State Change** — verifiable proof required.
4. **Waves, Not Lines** — work is a DAG of concurrent batches, not a flat todo list.
5. **Agent-Agnostic by Design** — standardized interface via role-prompt injection.
6. **Human Gates at Phase Boundaries** — semantic transitions need explicit approval.
7. **Deterministic Reporting** — reports are projections of `state.json`, never LLM output.
8. **Steering as Constitution** — durable steering files outlive chat sessions.

### A.4 Core use cases (the value that must survive the rebuild)

- **A single agent driving a disciplined lifecycle** (Base/`simple` mode): `init → new → author requirements (EARS) → approve → design → approve → tasks (DAG) → approve → next → implement → verify → complete → approve`.
- **Parallel/async multi-agent execution** (paper's *orchestrator* mode): frontier dispatch of ready-to-run packets; optional Brain/Pinky controller for autonomous coordination.
- **Host onboarding in one command**: `specd init --agent auto` detects the coding agent and wires project-scoped MCP.
- **Deterministic PR gating & reporting**: network-free summaries, HTML/Markdown reports, live frontier stream — all computed, never generated.

### A.5 Architecture snapshot (current)

- **Language**: Go 1.22+, **stdlib only**, zero runtime deps, single static binary, all templates via `go:embed`.
- **Layers**: `internal/cli` (≈40-line arg parser, deliberately *not* Cobra) → `internal/cmd` (one file per command, dispatched via `cmd.Registry`) → `internal/core` (gates, state, DAG, parser, config, schema) → `internal/mcp` (JSON-RPC stdio/HTTP server) → `internal/testharness` (deterministic sandbox, in-process runner, `FakeClock`).
- **Key contracts**: `FindSpecdRoot` (walk up to `.specd/`), `LoadState`/`SaveState` (atomic + **CAS on `revision`**), `AtomicWrite` (temp+fsync+rename), `WithSpecLock` (reentrant per-spec advisory lock), `ParseTasksMd` (bespoke line parser, **100% byte round-trip stable**), `BuildContextManifest` (one engine feeding `context`, `next --dispatch`, and Pinky briefs so they never drift).

### A.6 Honest read: where the accretion is (what the fresh start must triage)

The current tree is **~56k LOC across `cmd`+`core` and ~40 command files.** The disciplined core is a fraction of that. Everything beyond the core lifecycle is *expansion* that a fresh start should re-justify from first principles — this is the "development depression and redundant workflows" to abandon:

- **Core lifecycle (keep, it's the product):** `init, new, check, approve, next, verify, task, context, status, waves, decision, midreq, memory, report`.
- **Agent-agnostic plumbing (keep, but simplify):** `mcp, handshake, dispatch`, host adapters in `internal/integration`.
- **Orchestration layer (keep the *idea*, re-scope the surface):** `brain*, pinky, conductor, orchestrate` — 5+ files. The deterministic Brain controller is philosophically central (it's the paper's orchestrator mode), but the current API surface (claim/heartbeat/progress/query/directive/checkpoint/inbox/report + resume + resilience config) is large. **Fresh start question: what is the *minimal* controller that still guarantees evidence integrity, lease safety, and cooperative cancel?**
- **Extended "flywheel" loop (defer / prove necessity):** `deploy, observe, eval, review, security, submit, ingest, migrate, program_schedule, watch_webhook, dashboard, harness, doc`. These realize the paper's *maintenance/feedback* phase, but each is a candidate for **v2**, a plugin, or deletion. They are the biggest source of surface bloat relative to core value.

> **The fresh-start mandate is subtractive first.** Before adding better architecture, decide what the *minimal accurate tool* is. Target: the smallest command set that fully delivers principles 1–8, with the flywheel and heavy orchestration as clearly separated, opt-in tiers.

---

## Part B — Instructions for Claude Code

> **Read this section as your operating brief, Claude Code.** You will produce your *own* analysis. Do not copy Part A's conclusions — **verify them against the code**, then decide. Your output is a set of per-domain analysis files that become specs.

### B.0 Your mission

Produce the analysis-and-design layer for a **fresh, from-scratch reimplementation of `specd`** that:
1. Achieves the eight principles and the paper's *Agent = Model + Harness* thesis with the **minimal accurate command/code surface**.
2. Improves architecture, testability, and flexibility over the current tree.
3. Sheds redundant workflows — every retained feature must earn its place against a core use case.
4. Is grounded in **context-engineering foundations** (budgeted, targeted, minimal-sufficient context — the paper's "real skill").

### B.1 The translation contract (this governs *how* you analyze)

**Every domain analysis you write will be translated into a `spec.md` (requirements + design) and a `tasks.md` (a DAG of waves) and then executed.** Therefore, as you analyze each domain, you must already be writing in a way that decomposes cleanly:

- Express requirements as **EARS-shaped statements** ("When <trigger>, the system shall <response>") so they drop straight into `requirements.md`.
- Name concrete **design decisions, module boundaries, and data contracts** — these become `design.md` sections.
- End every domain file with a **proposed task DAG**: discrete tasks, their `role` (scout/craftsman/validator/auditor), `files:` they touch, `depends-on:` edges, and a `verify:` command for each. This is the seed of `tasks.md`.
- For every claim, cite the **exact `specd` reference file** you learned it from and state **keep / simplify / cut / redesign** with a one-line reason.

### B.2 Working method (do this in order)

1. **Preflight (read-only scout pass).** Confirm or correct Part A against the code. Read `docs/concepts.md`, `docs/contributor-guide.md`, `docs/agent-integration.md`, `docs/validation-gates.md`, and skim every file in `internal/core/` and `internal/cmd/`. Produce a **scope-triage table**: every current command → keep / simplify / cut / defer, with a reason tied to a principle or use case.
2. **Domain analysis.** For each domain in B.3, write `fresh-start/<domain>.md` using the template in B.4.
3. **Cross-cut pass.** Reconcile overlaps (e.g., the shared context manifest engine spans several domains) and record decisions in `fresh-start/00-decisions.md` (ADR style).
4. **Sequencing.** Write `fresh-start/00-roadmap.md`: the order specs should be authored and the cross-spec dependency DAG (which specs block which), mirroring `specd status --program`.
5. **Stop at analysis.** Do **not** implement yet. These files are inputs to spec authoring. Await approval to translate them into `spec.md`/`tasks.md`.

Use read-only sub-agents for the scout/preflight sweeps where available; keep your own context lean (practice the context engineering you're designing for).

### B.3 The domains (one analysis file each)

Write one `fresh-start/<n>-<domain>.md` per item. For each, the **learn-from** files are your primary sources; the **optimize** column is your mandate.

| # | Domain | Learn from (specd reference files) | Optimize / decide |
|---|---|---|---|
| 01 | **Product & Philosophy Core** | `README.md`, `docs/concepts.md`, `The_New_SDLC_With_Vibe_Coding.pdf` (Harness Engineering, pp. 26–34) | Define the *minimal* product. Draw the keep/cut line. Map each retained feature to one of the 8 principles + a paper concept. Kill anything unmapped. |
| 02 | **Spec Lifecycle & State Model** | `internal/core/state.go`, `phases.go`, `internal/cmd/{new,approve,status}.go` | The phase ratchet + `state.json` as machine truth. CAS/atomic-write invariants. Simplify state schema; make execution-mode (simple vs orchestrated) a clean first-class field. |
| 03 | **Validation Gates Engine** | `docs/validation-gates.md`, `internal/core/{ears,specfiles,dag}.go`, `internal/cmd/check.go` | The 7 core gates as pure functions. Design a **pluggable gate interface** so acceptance/scope/custom/context-budget gates are opt-in modules, not hardcoded branches. |
| 04 | **Task DAG & Wave Execution** | `internal/core/{tasksparser,dag}.go`, `internal/cmd/{next,waves,dispatch}.go` | Frontier computation, critical path, byte-stable parser. Decide: keep the bespoke Markdown parser vs a cleaner canonical format. Preserve round-trip stability as a hard requirement. |
| 05 | **Evidence & Verification** | `internal/cmd/{verify,task}.go`, verify sandbox/rollback logic | The evidence gate (passing record → complete). Sandbox (`bwrap`/container, fail-closed) and `--revert-on-fail`. Simplify the record format; keep evidence integrity absolute. |
| 06 | **Agent-Agnostic Integration** | `docs/agent-integration.md`, `internal/integration/*`, `internal/core/embed_templates/{AGENTS.md,roles,steering}` | Roles, steering constitution, AGENTS.md merge, host adapters, `--config` snippet fallback. Design the *smallest* adapter contract (detect/plan/install/inspect/verify) with the snippet as the universal floor. |
| 07 | **MCP & Handshake Surface** | `internal/mcp/*`, `internal/cmd/{mcp,handshake}.go`, `docs/mcp-guide.md` | stdio + HTTP JSON-RPC. Decide the minimal parity-tested tool set + intent-level tools. Handshake/policy digests. Cut redundant passthroughs. |
| 08 | **Context Engineering** | `internal/cmd/context.go`, `BuildContextManifest`, paper's "Context engineering: the real skill" (pp. 15–18) | The single shared manifest engine feeding `context`, `dispatch`, and worker briefs. Modes: `read-full`/`read-targeted`/`run-command`/`reference-if-needed`. Budget enforcement. Make this a **first-class, central module** — it's the paper's core skill. |
| 09 | **Orchestration (Brain/Pinky)** | `docs/agent-integration.md` (Brain/Pinky), `internal/cmd/{brain*,pinky,conductor,orchestrate}.go`, `internal/worker/*` | The **deterministic controller** (never calls an LLM) + ephemeral workers over a file-backed ACP. **Aggressively minimize the surface**: define the smallest command set that still guarantees evidence integrity, lease safety, cost/time brakes, and cooperative cancel. Make it a clean opt-in tier. |
| 10 | **CLI Architecture & Foundations** | `main.go`, `internal/cli/args.go`, `internal/cmd/registry.go`, `internal/core/{io,lock,paths,config_loader}.go` | Zero-dep parser + flat registry + registry↔help single-source guard. Atomic writes, advisory locks, config cascade (YAML). Re-evaluate: keep custom parser? Config format? Keep the "help can't drift" test. |
| 11 | **Reporting & Observability** | `internal/cmd/{report,status}.go`, `docs/dashboard.md`, `internal/obs/*` | Deterministic Markdown/HTML/PR-summary, live frontier stream (NDJSON/SSE/webhook), history/diff. All projections of state — **no LLM**. Decide which live surfaces are core vs deferred. |
| 12 | **Extended Loop / Flywheel (triage tier)** | `docs/flywheel.md`, `internal/cmd/{deploy,observe,eval,review,security,submit,ingest,migrate}.go` | Realizes the paper's maintenance/feedback phase. **Default posture: defer to v2 or plugin.** For each command, justify inclusion against a core use case or move it out of the MVP. This is the primary redundancy-shedding domain. |

> If your preflight reveals a domain that belongs on this list and isn't here, **add it** and note why in `00-decisions.md`. The list is a floor, not a ceiling.

### B.4 Per-domain file template (use verbatim)

```markdown
# Domain: <name>

## 1. Purpose & value mapping
- Which of the 8 principles this domain serves.
- Which paper concept (Agent=Model+Harness component: instructions / tools /
  sandboxes / orchestration / guardrails / observability / context) it realizes.
- The core use case(s) it enables. If none → recommend CUT.

## 2. Current-state analysis (from specd)
- Reference files read (exact paths).
- What exists today; the key contracts/invariants.
- Redundancy / complexity / drift found. Evidence, not opinion.

## 3. Fresh-start decision
- Verdict per capability: KEEP / SIMPLIFY / REDESIGN / CUT / DEFER — each with a reason.
- The minimal accurate surface (commands, modules, data contracts).
- Architecture & flexibility improvements over current tree.

## 4. Requirements (EARS-shaped) — seed for requirements.md
- "When <trigger>, the system shall <response>." (numbered, testable)

## 5. Design notes — seed for design.md
- Module boundaries, key types, data/on-disk contracts, invariants to preserve
  (e.g. CAS, atomic write, byte round-trip), external interfaces.

## 6. Proposed task DAG — seed for tasks.md
- Tasks with: id, role, files:, depends-on:, verify: command, acceptance.
- Group into waves (concurrent batches).

## 7. Risks, open questions, cross-domain dependencies
- What must be decided in 00-decisions.md. Which other domains this blocks / needs.
```

### B.5 Guardrails (non-negotiable)

- **Determinism first.** No feature may put an LLM call inside the harness's decision path. Gates, DAG, reports, Brain decisions stay pure functions of on-disk state.
- **Evidence integrity is absolute.** No task completes without a passing verify record (read-only roles use the explicit `--unverified --evidence` escape hatch only).
- **Preserve the hard invariants** unless you make an explicit, recorded ADR to change them: atomic writes, CAS on `revision`, reentrant per-spec lock, `ParseTasksMd` byte round-trip, embedded templates, zero runtime deps.
- **Subtractive bias.** When unsure whether something is core, default to CUT/DEFER and record the reasoning. The goal is the minimal accurate path.
- **Context discipline.** Practice what you design: lean context, targeted reads, cite sources.
- **Analysis only.** Produce `fresh-start/*.md`. Do not write production code or scaffold specs until these are approved.

### B.6 Definition of done for this phase

- [ ] `fresh-start/00-scope-triage.md` — every current command classified keep/simplify/cut/defer.
- [ ] `fresh-start/00-decisions.md` — cross-cutting ADRs (parser, config format, orchestration surface, flywheel tiering, invariant changes).
- [ ] `fresh-start/01…12-*.md` — one analysis file per domain, following the B.4 template.
- [ ] `fresh-start/00-roadmap.md` — spec-authoring order + cross-spec dependency DAG.
- [ ] Every retained feature traces to a principle + a paper concept + a use case. Everything else is explicitly cut or deferred with a reason.

---

## Appendix — Quick reference map (for Claude Code's preflight)

**Docs:** `docs/{concepts,contributor-guide,agent-integration,validation-gates,command-reference,mcp-guide,flywheel,dashboard,open-spec-format,spec-packs,custom-gates}.md`
**Core contracts:** `internal/core/{paths,io,lock,state,phases,tasksparser,dag,ears,specfiles,config_loader,commands}.go`
**Command surface (~40 files):** `internal/cmd/*.go` — dispatched via `registry.go`, help metadata in `internal/core/commands.go` (kept in sync by `TestRegistryMatchesHelp`).
**Agent-facing templates:** `internal/core/embed_templates/{AGENTS.md,roles/,steering/,skills/,specStubs/}`
**Test harness:** `internal/testharness/*` (sandbox repo, in-process runner, `FakeClock`, fluent spec builder).
**The paper:** `The_New_SDLC_With_Vibe_Coding.pdf` — anchor on *Harness Engineering* (pp. 26–34), *Context engineering: the real skill* (pp. 15–18), *conductor vs orchestrator* (pp. 31–34), *the 80% problem* (p. 34).
</content>
</invoke>
