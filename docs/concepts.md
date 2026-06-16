# Concepts

> *The agent reasons. The harness enforces.*

## What is specd?

`specd` is a **spec-driven coding harness CLI** that fuses structured spec
workflows with rigid thinking discipline for AI coding agents. It shifts the
burden of process enforcement off the LLM's non-deterministic context and onto a
strict, local, tool-gated pipeline.

### Key capabilities

| Capability | Description |
|---|---|
| **Planning Ratchet** | Enforces human-approved phase gates (Analyze → Plan → Execute → Verify → Reflect). |
| **Validation Gates** | Programmatic `specd check` — 7 core gates (EARS, design, task schema, DAG, evidence, sync, traceability) plus opt-in acceptance, scope, and [custom](./custom-gates.md) gates. |
| **DAG-Based Execution** | Computes the concurrent runnable frontier of waves for parallel task execution. |
| **Evidence-Gated Completion** | Tasks complete only against a passing `verify` record — never on a free-text claim. |
| **Frontier Dispatch** | Emits ready-to-run packets for parallel subagents with role prompts and contracts. |
| **Verify Sandboxing & Rollback** | Run `verify:` under `bwrap`/container isolation (fail-closed) and optionally stash the tree on failure (`--revert-on-fail`). |
| **Agent-Agnostic** | Works with Claude Code, Cursor, Aider, any command-running agent, or any [MCP](https://modelcontextprotocol.io) client (`specd mcp`). |
| **Deterministic Reporting** | Markdown / self-contained HTML reports, a read-only live dashboard (`specd serve`), and a network-free PR summary — no LLM dependency. |
| **Live Frontier Stream** | `specd watch` emits a `FrontierEvent` on every runnable-set change over NDJSON / SSE / webhook. |
| **Replay & Diff** | Reconstruct a deterministic audit timeline (`specd replay`) or diff a spec's artifacts across git refs (`specd diff`). |
| **Open Spec Format** | A versioned, embedded JSON Schema for all on-disk artifacts (`specd schema` / `specd validate --schema`). |
| **Spec Packs** | Share a steering/role baseline as a declarative, file-only scaffold (`specd init --pack`). |
| **Cost & Telemetry Ledger** | Per-task duration/retries plus annotated token/cost rolled up per wave/spec (stored, never computed). |
| **Pluggable State Backend** | File backend by default; git-native, or Redis/Postgres behind build tags — the default binary links no DB driver. |

### The foundational split

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

## The eight principles

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

## Architecture overview

> **Implementation language:** Go (1.22+), standard library only — zero external
> dependencies. Ships as a single static binary with all templates embedded via
> `go:embed`.

### Repository structure

```
specd/
├── main.go                       # Entry point, arg router, dispatch switch
├── internal/
│   ├── cli/                      # Flag/positional parser (Args)
│   ├── cmd/                      # One file per CLI command (Run<Command>)
│   ├── core/                     # Domain logic (gates, state, runners, schema, backends)
│   │   ├── schema/               # Embedded open-spec-format JSON Schema (go:embed)
│   │   ├── embed_packs/          # Built-in spec packs (go:embed)
│   │   └── embed_templates/      # Shipped templates (embedded in binary)
│   │       ├── AGENTS.md         # Agent prompt pack for user repos
│   │       ├── config.json       # Default config scaffold
│   │       ├── steering/         # Constitution files
│   │       ├── roles/            # Role persona prompts
│   │       ├── specStubs/        # Spec artifact stubs
│   │       └── skills/           # Companion skills (e.g. specd-execute)
│   ├── mcp/                      # MCP JSON-RPC 2.0 stdio server (specd mcp)
│   └── testharness/              # Deterministic test infrastructure
│                                 # (sandbox repo, in-process runner, FakeClock)
├── .github/actions/specd-pr/     # Composite GitHub Action (PR gates + summary)
├── scripts/                      # install.sh / uninstall.sh / stress.sh
├── docs/                         # This documentation
├── README.md / AGENTS.md / TESTING.md / SECURITY.md
├── Makefile / go.mod / LICENSE / .goreleaser.yml
```

> The default binary is stdlib-only with no DB driver. The git-native state
> backend needs no Go dependency; the Redis/Postgres adapters compile in only
> under the `specd_redis` / `specd_postgres` build tags.

See the [Contributor Guide](./contributor-guide.md) for a file-by-file map and
the key code contracts.

### Target repository structure (after `specd init`)

```
your-project/
├── .specd/
│   ├── config.json               # Project configuration
│   ├── program.json              # Cross-spec dependencies
│   ├── state.json                # Machine state (auto-managed)
│   ├── skills/                   # The skill pack (specd-foundations, specd-steering, per-stage)
│   ├── steering/                 # Constitution (durable rules)
│   │   ├── reasoning.md  workflow.md  product.md
│   │   ├── tech.md       structure.md memory.md
│   ├── roles/                    # Role prompts
│   │   ├── investigator.md  builder.md
│   │   └── reviewer.md       verifier.md
│   └── specs/
│       └── my-feature/
│           ├── state.json            # Spec-specific state
│           ├── requirements.md       # EARS requirements
│           ├── design.md             # Design document
│           ├── tasks.md              # Task DAG
│           ├── decisions.md          # ADRs
│           ├── mid-requirements.md   # Requirement updates
│           └── memory.md             # Local learnings
└── AGENTS.md                     # Agent workflow guide
```
