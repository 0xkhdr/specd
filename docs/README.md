# specd — Documentation

> **Status:** Normative documentation for current `specd` behavior.

`specd` is a **spec-driven coding harness CLI** — Go, standard library only, zero runtime
dependencies, one static binary. It moves process enforcement out of the LLM's context window
into a deterministic, local, tool-gated pipeline: **requirements → design → tasks →
evidence-gated execution**.

> **The agent reasons. The harness enforces.**

## Fast paths

| I want to… | Start here |
|---|---|
| Understand *why* specd exists | [concepts.md](concepts.md) |
| Run a spec end to end | [user-guide.md](user-guide.md) |
| Look up a verb, flag, or exit code | [command-reference.md](command-reference.md) |
| Know what a gate checks / why it failed | [validation-gates.md](validation-gates.md) |
| Wire up an agent, roles, or orchestration | [agent-integration.md](agent-integration.md) |
| Connect an MCP client | [mcp-guide.md](mcp-guide.md) |
| See what specd reports & how metrics surface | [observability.md](observability.md) |
| Read the on-disk `.specd/` format | [open-spec-format.md](open-spec-format.md) |
| Gate pull requests in CI | [github-action.md](github-action.md) |
| Fix a blocked task / gate / lock error | [troubleshooting.md](troubleshooting.md) |
| Hack on the codebase | [contributor-guide.md](contributor-guide.md) |

## All documents

- 💡 **[concepts.md](concepts.md)** — the foundational split, the philosophy pillars, the phase
  lifecycle, and base vs. orchestrated execution.
- 📖 **[user-guide.md](user-guide.md)** — install → `init`/`new` → the phase lifecycle →
  the verify→complete loop → mid-stream changes → review & submit.
- 📑 **[command-reference.md](command-reference.md)** — the **source-of-truth** doc: every verb,
  flag, exit code, and allowed phase, generated to match `internal/core/commands.go`.
- ✅ **[validation-gates.md](validation-gates.md)** — the 23 core gates plus the opt-in security
  gates: what each checks, when it fires, how to fix a failure.
- 🤖 **[agent-integration.md](agent-integration.md)** — the `AGENTS.md` loop, the four roles,
  steering, the context manifest, dispatch packets, Brain/Pinky orchestration, cross-spec
  programs.
- 🔌 **[mcp-guide.md](mcp-guide.md)** — `specd mcp` stdio server, host config snippets, and the
  handshake digests.
- 📦 **[open-spec-format.md](open-spec-format.md)** — the on-disk `.specd/` layout and the
  `state.json` schema.
- ⚙️ **[github-action.md](github-action.md)** — the composite PR-check action and the
  `report --pr` summary in CI.
- 📈 **[observability.md](observability.md)** — the deterministic reporting surface, logging /
  telemetry strategy, and where worker `--tokens`/`--cost`/`--duration-ms` surface in reports.
- 🩺 **[troubleshooting.md](troubleshooting.md)** — blocked tasks, the escalation ratchet, lock
  contention, CAS conflicts, verify/sandbox failures.
- 🛠️ **[contributor-guide.md](contributor-guide.md)** — codebase walkthrough by domain, the
  non-negotiable invariants, the concurrency/durability model, and extension recipes.
- 📐 **[scale-envelope.md](scale-envelope.md)** — intended limits (tasks/spec, specs/program) with
  the measured benchmark numbers backing them.
- 🧪 **[../TESTING.md](../TESTING.md)** — how to run the suite, the coverage floor, the regression
  harnesses + their cadence, and the stress/crash-safety jobs.
- 🧑‍💻 **[../CONTRIBUTING.md](../CONTRIBUTING.md)** — first-change quick-start (setup, the gate
  loop, house rules).
- 🏷️ **[versioning-policy.md](versioning-policy.md)** — SemVer, the Go floor, and how releases are
  cut. Changes are logged in **[../CHANGELOG.md](../CHANGELOG.md)**.
- 🔐 **[../SECURITY.md](../SECURITY.md)** — threat model, verify isolation contract, and
  vulnerability-disclosure policy.

## The non-negotiables

Every doc here is written to preserve these; they are the whole point of the tool:

1. **Determinism first** — no LLM in any gate, DAG, or report path.
2. **Evidence integrity** — a task completes *only* against a passing verify record (exit 0
   pinned to a real git HEAD). **No bypass flag exists.**
3. **Structural invariants** — atomic writes, CAS on `state.json` revision, reentrant per-spec
   lock, byte-stable tasks parser, `go:embed` templates, zero runtime dependencies.

---

← back to the [project README](../README.md)
