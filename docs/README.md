# specd Documentation

> **Agent-agnostic, spec-driven coding harness CLI.**
> *The agent reasons. The harness enforces.*

Pick the guide that matches what you are doing.

| Guide | Read it when you want to… |
|---|---|
| 💡 [Concepts](./concepts.md) | Understand *why* specd exists — the foundational split, the eight principles, and the architecture at a glance. |
| 📖 [User Guide](./user-guide.md) | Use `specd` inside a target repo — install, the spec lifecycle, writing EARS/design/tasks artifacts, the `verify → complete` flow, and troubleshooting. |
| 📑 [Command Reference](./command-reference.md) | Look up a command, its flags, exit codes, environment variables, or `config.json` keys. |
| ✅ [Validation Gates](./validation-gates.md) | Learn what each of the 7 spec gates (plus the 2 repo-global freshness gates) checks and why it fails. |
| 🤖 [Agent Integration](./agent-integration.md) | Wire a coding agent to `specd` — steering constitution, role personas, `inline`/`delegate` dispatch, context engineering, and cross-spec programs. |
| 🛠️ [Contributor Guide](./contributor-guide.md) | Hack on `specd` itself — codebase walkthrough, the concurrency/durability model, parser internals, and extension recipes. |

## Fast paths

- **First time?** → [User Guide → Installation](./user-guide.md#installation--setup)
- **"How do I complete a task?"** → [User Guide → The Verify → Complete Flow](./user-guide.md#the-verify--complete-flow) (`specd verify` then `specd task --status complete`)
- **"What does this command do?"** → [Command Reference](./command-reference.md)
- **"Why is my spec gated?"** → [Validation Gates](./validation-gates.md) and [Troubleshooting](./user-guide.md#troubleshooting)
- **Running parallel subagents?** → [Agent Integration → Subagent Coordination](./agent-integration.md#subagent-coordination-modes)
- **Adding a command or gate?** → [Contributor Guide → Extending the CLI](./contributor-guide.md#extending-the-cli)
