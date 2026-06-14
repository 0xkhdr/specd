# specd — Spec-Driven Coding Harness

`specd` is an **agent-agnostic, spec-driven coding harness CLI**. It fuses a structured spec workflow (requirements → design → tasks → evidence-gated execution) with a rigid thinking discipline for AI coding agents. 

By defining clear phase transitions and validating structural correctness with deterministic gates, `specd` shifts the burden of process enforcement from the LLM's non-deterministic context to a strict, local, tool-gated pipeline.

> **The agent reasons. The harness enforces.**

---

## Key Features

- 🔄 **Strict Planning Ratchet**: Enforces human-approved phase gates (Analyze → Plan → Execute → Verify → Reflect).
- 🛡️ **7 Validation Gates**: Programmatic checks (`specd check`) including EARS requirement syntax, complete design templates, acyclic task DAGs, and sync checks.
- 📉 **DAG-Based Task Execution**: Computes the concurrent runnable frontier of waves so agents only work on tasks whose dependencies are fully resolved.
- 💾 **Evidence-Gated Completion**: `specd verify` runs the task's own `verify:` command and records the exit code + git HEAD. A task completes only against a passing record — never on a free-text claim alone.
- 🚦 **Frontier Dispatch & Cross-Spec DAG**: `specd dispatch` emits ready-to-run packets (role prompt + contract + verify) for parallel subagents; `specd program` resolves which whole specs are runnable across a multi-spec program.
- 🔌 **Agent-Agnostic**: Teaches any command-running agent (Claude Code, Cursor, Aider, etc.) how to execute the workflow via a localized prompt pack and steering constitution.
- 📊 **Deterministic Status Reporting**: Generates markdown and self-contained HTML reports representing the state and wave DAG without any LLM dependencies or runtime overhead.

---

## Core Philosophy

`specd` is built on eight core principles designed to make AI software engineering reliable, structured, and predictable:

1. **The Foundational Split**: The agent does the creative thinking; the harness enforces process integrity.
2. **Specs as the Source of Truth**: The active plan lives as versioned Markdown on disk, not floating in the LLM's context window.
3. **Evidence Gates Every State Change**: *Trust is recorded, not assumed.* Status changes require verifiable proof.
4. **Waves, Not Lines**: Work is structured as a Directed Acyclic Graph (DAG) of concurrent batches (waves) rather than flat todo lists.
5. **Agent-Agnostic by Design**: Standardized command interface integrated via role prompt injection.
6. **Human Gates at Phase Boundaries**: Semantic transitions require explicit human approval (`specd approve`).
7. **Deterministic Reporting**: Reports are generated programmatically from `state.json` and artifact files.
8. **Steering as Constitution**: Durable steering files outlive individual chat sessions.

---

## Installation

### Quick Install (Linux / macOS)
```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash
```

### With Options
```bash
# Force reinstall
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --force

# Install specific version
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash -s -- --version 0.2.0
```

### Uninstall
```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | bash
```

### Update
```bash
specd update
specd update --force
```

### Requirements
- Linux or macOS (amd64 / arm64)
- Git (optional — tarball fallback available)

## For Agents

`specd` is designed to be fully drivable by AI agents:

- Set `SPECD_JSON=1` to receive structured JSON output for all commands.
- Use `specd help --json` to discover the full command schema programmatically.
- All state mutations are atomic and versioned — safe for concurrent agent access.
- Exit codes are deterministic: `0`=ok, `1`=validation, `2`=usage, `3`=not found.

## Quick Start

### Initializing a Project

After installing, run `specd init` in any project root to get started:

```sh
# Scaffolds .specd/ structure, roles, steering config, and AGENTS.md
specd init
```

### Creating and Running a Spec

1. **Start a new spec**:
   ```sh
   specd new my-feature --title "Implement Feature X"
   ```
2. **Author Requirements**: Open `.specd/specs/my-feature/requirements.md` and document requirements in EARS format. Validate and approve:
   ```sh
   specd check my-feature
   specd approve my-feature # Advances spec status from 'requirements' to 'design'
   ```
3. **Create Design**: Fill out `.specd/specs/my-feature/design.md` covering all mandatory sections. Approve:
   ```sh
   specd approve my-feature # Advances spec status from 'design' to 'tasks'
   ```
4. **Decompose into Tasks**: Author the task list under `.specd/specs/my-feature/tasks.md` defining dependency waves. Approve:
   ```sh
   specd approve my-feature # Advances spec status from 'tasks' to 'executing'
   ```
5. **Execute Tasks**: Get the next runnable task from the frontier, implement it, then let `specd` run the task's `verify:` command itself and record the result. A task can only complete on a **passing verify record** — not a free-text claim:
   ```sh
   specd next my-feature
   # [Implement the changes...]
   specd verify my-feature T1                  # specd runs the task's verify: command, records exit code + git HEAD
   specd task my-feature T1 --status complete   # allowed only because the verify record passed (exit 0)
   ```
   Read-only roles (investigator/reviewer) whose `verify` is `N/A` complete with the manual escape hatch: `specd task my-feature T1 --status complete --unverified --evidence "<proof>"`.
6. **Final Verification**: Once all tasks are complete, run final checks and sign off:
   ```sh
   specd approve my-feature # Closes the spec, promoting learnings and generating final reports
   ```

---

## Repository & Documentation Map

```
.
├── README.md               # This overview guide
├── AGENTS.md               # Workflow guide for AI agents working on the specd repo itself
├── TESTING.md              # Deterministic test-harness guide
└── docs/                   # Detailed documentation
    ├── README.md              # Documentation index / navigation
    ├── concepts.md            # Philosophy, the eight principles, architecture
    ├── user-guide.md          # Install, lifecycle, artifacts, execution, troubleshooting
    ├── command-reference.md   # Every command, flag, exit code, env var, config key
    ├── validation-gates.md    # What each of the 7 (+2 repo-global) gates checks
    ├── agent-integration.md   # Steering, roles, subagent modes, context, programs
    └── contributor-guide.md   # CLI architecture, concurrency model, extension recipes
```

### Quick Links:
- 💡 [Concepts](docs/concepts.md) — The foundational split, eight principles, architecture overview.
- 📖 [User Guide](docs/user-guide.md) — Getting started, EARS requirements, design headers, task DAG, the verify → complete flow.
- 📑 [Command Reference](docs/command-reference.md) — Commands, flags, exit codes, environment variables, and `config.json`.
- ✅ [Validation Gates](docs/validation-gates.md) — The 7 spec gates plus the 2 repo-global freshness gates.
- 🤖 [Agent Integration Guide](docs/agent-integration.md) — Roles, steering files, subagent modes, context engineering, cross-spec programs.
- 🛠️ [Contributor Guide](docs/contributor-guide.md) — Codebase architecture, the concurrency model (advisory lock + CAS), parser internals, adding commands/gates.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
