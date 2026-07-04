# specd Documentation

> *The agent reasons. The harness enforces.*

This directory contains the authoritative documentation for `specd` — the spec-driven coding harness.

---

## Navigation

| Document | Purpose |
|---|---|
| [concepts.md](./concepts.md) | Philosophy, the eight principles, architecture overview |
| [user-guide.md](./user-guide.md) | Install, lifecycle, artifacts, task execution, evidence |
| [command-reference.md](./command-reference.md) | Every command, flag, exit code, env var |
| [validation-gates.md](./validation-gates.md) | What each gate checks and when it fires |
| [agent-integration.md](./agent-integration.md) | Roles, steering files, MCP surface, context manifests |
| [contributor-guide.md](./contributor-guide.md) | Codebase map, key contracts, adding commands/gates |

---

## Quick Links

- 💡 [Concepts](./concepts.md) — The foundational split, eight principles, architecture
- 📖 [User Guide](./user-guide.md) — Install → init → spec lifecycle → evidence
- 📑 [Command Reference](./command-reference.md) — Commands, flags, exit codes, env vars
- ✅ [Validation Gates](./validation-gates.md) — Core gates + opt-in security gate
- 🤖 [Agent Integration](./agent-integration.md) — Roles, steering, MCP, context manifests
- 🛠️ [Contributor Guide](./contributor-guide.md) — Architecture, concurrency model, extension recipes

---

> **New?** Start with [user-guide.md](./user-guide.md) for the install and first-spec walkthrough.
> Agents should read [agent-integration.md](./agent-integration.md) first; the embedded `AGENTS.md`
> (written to your project on `specd init`) is the authoritative runtime briefing.
