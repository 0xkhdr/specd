# specd — Spec-Driven Coding Harness

`specd` is a **spec-driven coding harness CLI** that fuses structured spec workflows
(requirements → design → tasks → evidence-gated execution) with rigid thinking discipline
for AI coding agents. It shifts process enforcement from the LLM's non-deterministic
context window to a strict, local, tool-gated pipeline.

> **The agent reasons. The harness enforces.**

---

## Key Features

- 🔄 **Strict Planning Ratchet**: Enforces human-approved phase transitions (Perceive → Analyze → Plan → Execute → Verify → Reflect).
- 🛡️ **Validation Gates**: Programmatic checks (`specd check`) — 12 core gates (EARS syntax, design sections, task schema, acyclic DAG, evidence, sync, context budget, and more) plus an opt-in security gate.
- 📉 **DAG-Based Task Execution**: Computes the concurrent runnable frontier of waves so agents only work on tasks whose dependencies are resolved.
- 💾 **Evidence-Gated Completion**: Tasks complete only against a passing `verify` record (exit code 0 + git HEAD) — never on free-text claims.
- 🔒 **Verify Sandboxing & Rollback**: Run verification commands inside `bwrap`/container isolation and optionally stash the working tree on failure (`--revert-on-fail`).
- 🔌 **Agent-Agnostic & MCP Support**: Serve the command palette as a stdio MCP server (`specd mcp`) compatible with Claude Code, Cursor, Antigravity, or custom LLM clients.
- 🧠 **Orchestration Brain**: Opt-in deterministic controller (`specd brain`) to drive wave-based execution loops safely using leases.
- 🔄 **Learning Flywheel**: Append and promote durable steering-memory patterns from spec learnings.

---

## Core Philosophy

`specd` is built on eight core principles designed to make AI software engineering reliable, structured, and predictable:

1. **The Foundational Split**: The agent does the creative thinking; the harness enforces process integrity.
2. **Specs as the Source of Truth**: The active plan lives as versioned Markdown on disk, not floating in the LLM's context window.
3. **Evidence Gates Every State Change**: *Trust is recorded, not assumed.* Status changes require verifiable proof.
4. **Waves, Not Lines**: Work is structured as a Directed Acyclic Graph (DAG) of concurrent waves rather than flat todo lists.
5. **Agent-Agnostic by Design**: Standardized command interface integrated via role prompt injection.
6. **Human Gates at Phase Boundaries**: Semantic transitions require explicit human approval (`specd approve`).
7. **Deterministic Reporting**: Reports are generated programmatically from `state.json` and task artifacts.
8. **Steering as Constitution**: Durable steering files outlive individual chat sessions.

---

## Installation & Setup

### Building from Source

`specd` is written in Go (1.26+) and has **zero runtime dependencies**. It compiles into a single static binary:

```bash
# Clone the repository and build:
go build -o specd main.go

# Or run directly:
go run . help
```

### Initializing a Project

From your target project's root:

```bash
specd init
```

This scaffolds the `.specd/` folder (default role prompts and steering files) and writes `AGENTS.md` to the project root.

---

## Documentation Map

- 💡 [Concepts](docs/concepts.md) — The foundational split, eight principles, and spec lifecycle.
- 📖 [User Guide](docs/user-guide.md) — Walkthrough of the lifecycle, stubs, verify, and complete flow.
- 📑 [Command Reference](docs/command-reference.md) — Complete CLI syntax, flags, and exit codes.
- ✅ [Validation Gates](docs/validation-gates.md) — Details on all 12 core validation checks.
- 🤖 [Agent Integration](docs/agent-integration.md) — Roles, steering files, MCP setup, and brain controller.
- 🛠️ [Contributor Guide](docs/contributor-guide.md) — Codebase architecture, invariants, and CLI design decisions.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
