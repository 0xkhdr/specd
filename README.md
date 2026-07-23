# specd — Spec-Driven Coding Harness

> **Status:** Normative documentation for current `specd` behavior.

`specd` is a **spec-driven coding harness CLI** that fuses structured spec workflows
(requirements → design → tasks → evidence-gated execution) with rigid thinking discipline
for AI coding agents. It shifts process enforcement from the LLM's non-deterministic
context window to a strict, local, tool-gated pipeline.

> **The agent reasons. The harness enforces.**

---

## Key Features

- 🔄 **Strict Planning Ratchet**: Enforces human-approved phase transitions (Perceive → Analyze → Plan → Execute → Verify → Reflect).
- 🛡️ **Validation Gates**: Programmatic checks (`specd check`) — 25 core gates (EARS syntax, design sections, task schema, acyclic DAG, evidence, sync, context budget, typed intake, governance, memory lint, and more) plus a separate opt-in security gate.
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

Install latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh
```

Update an existing install:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh -s -- --update
```

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | sh
```

Installer flags:

```text
--version <tag>
--install-dir <dir>
--update
--force
--dry-run
```

Environment variables:

```text
SPECD_VERSION
SPECD_INSTALL_DIR
```

The installer supports Linux/macOS on amd64/arm64, verifies `checksums.txt`, and uses `sudo`
only when the install directory is not writable. Default install dir is `/usr/local/bin`.

### Building from Source

`specd` is written in Go (1.26+) and has **zero runtime dependencies**. It compiles into a single static binary:

```bash
# Clone the repository and build:
go build -o specd .

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

Start at the **[docs index](docs/README.md)** for fast paths, or jump straight in:

- 💡 [Concepts](docs/concepts.md) — The foundational split, the philosophy pillars, and spec lifecycle.
- 📖 [User Guide](docs/user-guide.md) — Walkthrough of the lifecycle, stubs, verify, and complete flow.
- 📑 [Command Reference](docs/command-reference.md) — Complete CLI syntax, flags, and exit codes.
- ✅ [Validation Gates](docs/validation-gates.md) — Details on all 25 core validation gates plus security gates.
- 🤖 [Agent Integration](docs/agent-integration.md) — Roles, steering files, dispatch, and the Brain/Pinky controller.
- 🔌 [MCP Guide](docs/mcp-guide.md) — The `specd mcp` stdio server, host config snippets, and handshake digests.
- 📦 [Open Spec Format](docs/open-spec-format.md) — The on-disk `.specd/` layout and `state.json` schema.
- ⚙️ [GitHub Action](docs/github-action.md) — Gate pull requests in CI with the composite action.
- 🩺 [Troubleshooting](docs/troubleshooting.md) — Blocked tasks, the escalation ratchet, lock and CAS errors.
- 🧑‍💻 [Contributing](CONTRIBUTING.md) — First-change quick-start: setup, the gate loop, house rules.
- 🛠️ [Contributor Guide](docs/contributor-guide.md) — Codebase architecture, invariants, and CLI design decisions.
- 🧪 [Testing](TESTING.md) — Suite commands, the coverage floor, regression harnesses, and stress jobs.
- 📈 [Observability](docs/observability.md) — The deterministic reporting surface and where worker metrics surface.
- 🏷️ [Versioning Policy](docs/versioning-policy.md) · [Changelog](CHANGELOG.md) — SemVer, the Go floor, release cuts.
- 🔐 [Security Policy](SECURITY.md) — Threat model (hostile spec/verify/dependency content), the verify isolation contract, and vulnerability disclosure.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
