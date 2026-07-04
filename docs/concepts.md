# Concepts

> *The agent reasons. The harness enforces.*

## What is specd?

`specd` is a **spec-driven coding harness CLI** that fuses structured spec workflows
with rigid thinking discipline for AI coding agents. It shifts the burden of process
enforcement off the LLM's non-deterministic context and onto a strict, local,
tool-gated pipeline.

### Key capabilities

| Capability | Description |
|---|---|
| **Planning Ratchet** | Enforces human-approved phase gates (Perceive → Analyze → Plan → Execute → Verify → Reflect). |
| **Validation Gates** | Programmatic `specd check` — 12 core gates (task-ids, dependencies, DAG, roles, files, verify, evidence, context-budget, EARS, approval, sync, design) plus an opt-in security gate. |
| **DAG-Based Execution** | Computes the concurrent runnable frontier of waves so agents only work on tasks whose dependencies are fully resolved. |
| **Evidence-Gated Completion** | Tasks complete only against a passing `verify` record — never on a free-text claim. |
| **Revert-on-Fail** | Optionally stash the working tree on verify failure (`--revert-on-fail`). |
| **Sandboxed Verify** | Run `verify:` under `bwrap`/container isolation (`--sandbox`). |
| **Frontier Dispatch** | `specd next --dispatch` emits the bounded context manifest for the first ready task. |
| **Brain Orchestration** | Opt-in deterministic controller (`specd brain`) that dispatches frontier tasks and maintains leases — no LLM in its decision path. |
| **Memory Flywheel** | `specd memory` appends and promotes durable steering-memory patterns from spec learnings. |
| **MCP Surface** | `specd mcp` serves all non-forbidden commands as MCP JSON-RPC 2.0 tools over stdio. |
| **Deterministic Reporting** | Markdown / evidence-backed reports and PR summaries (`specd report`). |
| **Agent-Agnostic** | Works with Claude Code, Cursor, Antigravity, Codex, or any command-running agent via the MCP server. |
| **Zero Runtime Deps** | Single static binary — Go stdlib only, no external packages. |

---

### The Foundational Split

```
┌─────────────────┐     ┌─────────────────┐
│  AGENT (LLM)    │     │  HARNESS (specd)│
│  ─────────────  │     │  ─────────────  │
│  • Reasons      │     │  • Enforces     │
│  • Creates      │◄───►│  • Validates    │
│  • Designs      │     │  • Gates        │
│  • Implements   │     │  • Records      │
└─────────────────┘     └─────────────────┘
```

The agent does the creative thinking. The harness enforces process integrity.

---

## The Eight Principles

1. **The Foundational Split** — The agent does the creative thinking; the harness enforces process integrity.
2. **Specs as the Source of Truth** — The active plan lives as versioned Markdown on disk, not floating in the LLM's context window.
3. **Evidence Gates Every State Change** — *Trust is recorded, not assumed.* Status changes require verifiable proof.
4. **Waves, Not Lines** — Work is a Directed Acyclic Graph (DAG) of concurrent batches (waves), not a flat todo list.
5. **Agent-Agnostic by Design** — A standardized command interface integrated via role prompt injection.
6. **Human Gates at Phase Boundaries** — Semantic transitions require explicit human approval (`specd approve`).
7. **Deterministic Reporting** — Reports are generated programmatically from `state.json` and artifact files.
8. **Steering as Constitution** — Durable steering files outlive individual chat sessions.

### Design philosophy in practice

```
Traditional AI Coding          specd-Driven Coding
─────────────────────          ───────────────────
❌ Free-form prompts           ✅ Structured spec artifacts
❌ Context window as source    ✅ Markdown files as source of truth
❌ "Trust me, it works"        ✅ Evidence-gated completion
❌ Linear todo lists           ✅ DAG-based wave execution
❌ Agent-specific workflows    ✅ Agent-agnostic CLI interface
```

---

## The Spec Lifecycle

A spec moves through six ordered statuses. Each transition requires explicit approval:

```
requirements → design → tasks → executing → verifying → complete
    ↕ (blocked can be set at any point)
```

| Status | Phase | Description |
|---|---|---|
| `requirements` | perceive | Author EARS-shaped requirements in `requirements.md`. |
| `design` | analyze | Fill out module boundaries, on-disk contracts, and invariants in `design.md`. |
| `tasks` | plan | Decompose work into a DAG in `tasks.md`. |
| `executing` | execute | Agents run tasks; `specd verify` records evidence. |
| `verifying` | verify | Final gate checks before closure. |
| `complete` | reflect | All tasks done, spec closed. |
| `blocked` | reflect | Unblocked signal; recorded in state. |

Approval gates are **human-only**: `specd approve <spec> <gate>` refuses if any core gate
fails. Gates never auto-advance — no heuristic flips a phase.

---

## Architecture Overview

> **Implementation language:** Go 1.26+, standard library only — zero external
> dependencies. Ships as a single static binary with all templates embedded via
> `go:embed`.

### Repository structure

```
specd/
├── main.go                       # Entry point — arg router, dispatch via cmd.Registry
├── internal/
│   ├── cli/
│   │   └── args.go               # Flag/positional parser (Args)
│   ├── cmd/                      # One file per CLI command (runXxx handlers)
│   │   ├── registry.go           # Command → handler dispatch table + ErrUnknownCommand
│   │   ├── lifecycle.go          # new, approve, task, midreq, decision, help
│   │   ├── brain.go / brain_run.go / brain_worker.go  # Orchestration controller
│   │   └── memory.go             # Learning flywheel commands
│   ├── core/                     # Domain logic — all pure functions of on-disk state
│   │   ├── commands.go           # Stable command palette (help metadata + JSON schema)
│   │   ├── state.go              # state.json load/save + CAS (SaveStateCAS)
│   │   ├── phases.go             # Phase ↔ Status mapping, AdvanceStatus
│   │   ├── tasksparser.go        # Bespoke line parser — ParseTasksMd, byte round-trip stable
│   │   ├── dag.go                # Wave DAG, Frontier, NewTaskDAG
│   │   ├── frontier.go           # Frontier computation helpers
│   │   ├── evidence.go           # AppendEvidence, LoadEvidence, HasPassingEvidence
│   │   ├── io.go                 # AtomicWrite (temp + fsync + rename), O_APPEND ledger
│   │   ├── lock.go               # WithSpecLock — reentrant per-spec advisory lock
│   │   ├── paths.go              # SpecdDir, StatePath, EvidencePath, SpecMemoryPath
│   │   ├── config_loader.go      # Config cascade: defaults → YAML → env
│   │   ├── config_validate.go    # ValidateConfig
│   │   ├── agents.go             # AGENTS.md marker-based merge
│   │   ├── scaffold.go           # WriteScaffold — idempotent init writer
│   │   ├── report.go / report_metrics.go  # BuildReportModel, RenderStatus, PRSummary
│   │   ├── md.go                 # Markdown render helpers
│   │   ├── memory.go             # Learning flywheel core (AppendMemory, PromoteMemory)
│   │   ├── roles.go              # Role validation
│   │   ├── slug.go               # ValidateSlug
│   │   ├── task_complete.go      # CompleteTask — evidence check + marker update
│   │   ├── gates/                # Gate registry and all gate implementations
│   │   │   ├── registry.go       # Gate Registry + HasErrors
│   │   │   ├── core.go           # CoreRegistry — 12 built-in gates
│   │   │   ├── approval.go       # Approval gate
│   │   │   ├── sync.go           # Sync gate (tasks.md ↔ state.json agreement)
│   │   │   ├── ears.go           # EARS requirements linter
│   │   │   ├── contextbudget.go  # Context-budget gate
│   │   │   └── security/         # Opt-in security gate
│   │   ├── verify/               # Verify executor (sandbox support)
│   │   └── embed_templates/      # Shipped templates (embedded in binary)
│   │       ├── AGENTS.md         # Agent prompt pack written to user repos
│   │       ├── roles/            # Role persona prompts (scout, craftsman, validator, auditor)
│   │       └── steering/         # Constitution files
│   ├── context/                  # Context manifest builder (BuildManifest, RenderHUD)
│   ├── mcp/                      # MCP JSON-RPC 2.0 stdio server (specd mcp)
│   ├── orchestration/            # Brain orchestration state, leases, ACP ledger
│   └── integration/              # Agent integration helpers
├── scripts/                      # regress-all.sh, regress-domains.sh, regress-lint.sh
├── reference/                    # Frozen v1 implementation — read-only museum
├── AGENTS.md                     # Workflow guide for AI agents working on specd itself
└── main.go / go.mod / LICENSE
```

### Target project structure (after `specd init`)

```
your-project/
├── .specd/
│   ├── specs/
│   │   └── my-feature/
│   │       ├── state.json          # Machine-truth state (never hand-edit)
│   │       ├── requirements.md     # EARS requirements
│   │       ├── design.md           # Design document
│   │       ├── tasks.md            # Task DAG (Markdown table)
│   │       ├── memory.md           # Local learnings (flywheel)
│   │       └── .lock               # Per-spec advisory lock file
│   ├── roles/                      # Role persona prompts
│   │   ├── scout.md
│   │   ├── craftsman.md
│   │   ├── validator.md
│   │   └── auditor.md
│   └── steering/                   # Constitution files (durable rules)
│       ├── reasoning.md
│       ├── workflow.md
│       ├── product.md
│       ├── tech.md
│       └── structure.md
└── AGENTS.md                       # Agent workflow guide (written by specd init)
```

---

## Execution Modes

Every spec records a `mode` in `state.json`:

| Mode | Description |
|---|---|
| `default` | Plain spec-driven lifecycle; the host agent drives every step itself (`specd next` → implement → `specd verify`). |
| `orchestrated` (opt-in) | The Brain controller may drive the spec. Requires `orchestration.enabled: true` in project config and `spec.mode = "orchestrated"`. |

**Brain is fail-closed**: without `--authority`, it observes and writes nothing. A
deferred `triage` command exists in the registry but is not yet wired.

See [contributor-guide.md](./contributor-guide.md) for the concurrency model and
[agent-integration.md](./agent-integration.md) for roles and steering details.
