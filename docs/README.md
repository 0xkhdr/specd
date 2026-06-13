# specd Documentation

Pick the guide that matches what you are doing:

| Guide | Read it when you want to… |
|---|---|
| 📖 [User Guide](./user-guide.md) | Use `specd` inside a target repo — lifecycle, writing EARS/design/tasks artifacts, the `verify → complete` flow, `dispatch`, cross-spec `program`, env vars, and troubleshooting. |
| 🤖 [Agent Integration Guide](./agent-integration.md) | Wire a coding agent to `specd` — the two `AGENTS.md` files, steering constitution, role personas, `inline`/`delegate` + `dispatch` orchestration, and context engineering. |
| 🛠️ [Contributor Guide](./contributor-guide.md) | Hack on `specd` itself — codebase walkthrough, the validation-gate pipeline, concurrency/durability model, parser internals, and extension recipes. |

## Fast paths

- **First time?** → [User Guide §1 Getting Started](./user-guide.md#1-getting-started)
- **"How do I complete a task?"** → [User Guide §6 Task Execution](./user-guide.md#6-task-execution-commands) (`specd verify` then `specd task --status complete`)
- **Running parallel subagents?** → [Agent Integration §4 Subagent Coordination](./agent-integration.md#4-subagent-coordination-modes)
- **Adding a command or gate?** → [Contributor Guide §5 Extending the CLI](./contributor-guide.md#5-extending-the-cli)
- **Hit an error?** → [User Guide §11 Troubleshooting](./user-guide.md#11-environment-variables--troubleshooting)
