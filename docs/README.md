# specd Documentation

> **Agent-agnostic, spec-driven coding harness CLI.**
> *The agent reasons. The harness enforces.*

Pick the guide that matches what you are doing.

| Guide | Read it when you want to… |
|---|---|
| 💡 [Concepts](./concepts.md) | Understand *why* specd exists — the foundational split, the eight principles, and the architecture at a glance. |
| 📖 [User Guide](./user-guide.md) | Use `specd` inside a target repo — install, the spec lifecycle, writing EARS/design/tasks artifacts, the `verify → complete` flow, and troubleshooting. |
| 📑 [Command Reference](./command-reference.md) | Look up a command, its flags, exit codes, environment variables, or `config.json` keys. |
| ✅ [Validation Gates](./validation-gates.md) | Learn what each spec gate checks and why it fails — the 7 core gates plus the opt-in acceptance, scope, and custom gates. |
| 🤖 [Agent Integration](./agent-integration.md) | Wire a coding agent to `specd` — steering constitution, role personas, `inline`/`delegate` dispatch, context engineering, and cross-spec programs. |
| 🧩 [Custom Gates](./custom-gates.md) | Add project-specific checks as external subprocesses with a JSON stdin/stdout contract. |
| 📦 [Spec Packs](./spec-packs.md) | Share a steering/role baseline as a declarative, file-only scaffold bundle (`specd init --pack`). |
| 📐 [Open Spec Format](./spec-format.md) | The versioned JSON Schema for specd's on-disk artifacts (`specd schema` / `specd validate --schema`). |
| 🐙 [GitHub Action](./github-action.md) | Run the gates on a PR and upsert a deterministic summary comment (`specd report --pr-summary`). |
| 🛠️ [Contributor Guide](./contributor-guide.md) | Hack on `specd` itself — codebase walkthrough, the concurrency/durability model, parser internals, and extension recipes. |

## Fast paths

- **First time?** → [User Guide → Installation](./user-guide.md#installation--setup)
- **"How do I complete a task?"** → [User Guide → The Verify → Complete Flow](./user-guide.md#the-verify--complete-flow) (`specd verify` then `specd task --status complete`)
- **"What does this command do?"** → [Command Reference](./command-reference.md)
- **"Why is my spec gated?"** → [Validation Gates](./validation-gates.md) and [Troubleshooting](./user-guide.md#troubleshooting)
- **Running parallel subagents?** → [Agent Integration → Subagent Coordination](./agent-integration.md#subagent-coordination-modes)
- **Live dashboard / event stream?** → `specd serve` and `specd watch` ([Command Reference](./command-reference.md#inspection-commands))
- **Driving specd from an MCP client?** → `specd mcp` ([Command Reference](./command-reference.md#meta-commands))
- **Adding a command or gate?** → [Contributor Guide → Extending the CLI](./contributor-guide.md#extending-the-cli)
