# Agent Integration Guide

This guide details how an AI coding agent (e.g. Claude Code, Cursor, Antigravity, or custom LLM wrappers) integrates with `specd` to drive or follow the spec-driven development loop.

---

## 1. AGENTS.md: Root vs. Scaffolded Template

There are two distinct `AGENTS.md` files in a `specd` workflow:

| File Location | Target Audience | Purpose |
|---|---|---|
| **Root `AGENTS.md`** | The AI agent developing the `specd` tool itself. | Contains rules and guidelines for working on the `specd` codebase (e.g. standard library only, testing requirements). |
| **Scaffolded `AGENTS.md`** | The AI agent working in the target project. | Written by `specd init` at the project's root directory. It instructs the agent on how to use `specd` to manage its own development tasks. |

---

## 2. Steering Constitution

Steering files under `.specd/steering/` define the durable operational rules of a project. They persist across individual chat sessions and guide agent reasoning:

- **`reasoning.md`**: Outlines the thinking discipline required of agents, including the six-phase spec ratchet (Perceive → Analyze → Plan → Execute → Verify → Reflect) and the backpropagation protocol.
- **`workflow.md`**: Defines spec status transitions, the validation gates, and requirements for advancement.
- **`product.md`**: Specifies the domain, target audience, business constraints, and goals.
- **`tech.md`**: Outlines the technical stack, runtime environments, languages, dependency policies, and test suites.
- **`structure.md`**: Prescribes folder layouts, module boundaries, architectural patterns, and naming conventions.

*Note: In `specd`, steering files act as the system prompt context. On `specd init`, boilerplate versions of these files are scaffolded for modification.*

---

## 3. Role Personas

Every task in the `tasks.md` Directed Acyclic Graph (DAG) is bound to a specific `role`. Roles are configured as Markdown persona prompts under `.specd/roles/`:

- **🔍 scout**: A read-only explorer. Role is strictly limited to reading the codebase, mapping dependencies, and documenting findings. Cannot make edits or write code.
- **🛠️ craftsman**: The writer. Implements features, writes tests, edits files defined within the task contract, and executes local verifications.
- **🧪 validator**: A read-only verification agent. Independent tester that runs verify suites and collects verification records.
- **🛡️ auditor**: A read-only reviewer. Evaluates git diffs, verifies acceptance criteria alignment, and logs code quality findings.

The `roles` validation gate checks that all tasks in the DAG specify one of these roles.

---

## 4. Context Manifests and Bounded Contexts

To keep the agent's context window clean and focused, `specd` provides context-engineering tools:

```bash
specd context <slug> <task-id>
```

By default, this command returns the exact list of files the active task requires (roles, steering config, task files, etc.).

### Command Flags

- **`--json`**: Emits a structured context manifest:
  ```json
  {
    "items": [
      {
        "path": "/absolute/path/to/source.go",
        "role": "craftsman"
      }
    ]
  }
  ```
- **`--hud`**: Renders an operator HUD to standard output, indicating total file count, size, and estimated tokens to load:
  ```
  Target File: /var/www/html/rai/up/specd/internal/context/hud.go
  Task ID: T1
  Total Estimated Tokens: 3450 / 12000
  ```

The context token limit is governed by the `context.max_tokens` configuration parameter (default `12000`). If the estimate exceeds the budget, the `context-budget` validation gate fails during `specd check`.

---

## 5. Model Context Protocol (MCP) Server

`specd` can act as an MCP server using:

```bash
specd mcp
```

This runs a stdio-based MCP JSON-RPC 2.0 server.

### Tools Discovery
The MCP server exposes all registered commands in `core.Commands` as tools, except for forbidden meta-commands (`mcp` and `handshake`). 

Tool inputs follow a flexible schema:
```json
{
  "type": "object",
  "additionalProperties": true
}
```
All parameters passed by the MCP client are forwarded directly to the command handler as CLI flags (e.g. `{"revert-on-fail": true}` becomes `--revert-on-fail`).

---

## 6. Deterministic Orchestration (Brain)

For projects with automated execution capability, the harness includes a deterministic orchestration controller:

```bash
specd brain <start|step|run|status|cancel|resume> <spec> [--authority]
```

### Preconditions for Orchestration
1. **Config capability**: `orchestration.enabled: true` must be set in `project.yml` at
   the repository root.
2. **Spec state**: The spec's `mode` must be `orchestrated` in `state.json`. Without
   both preconditions, `specd brain` fails closed (exit 1) and writes nothing.

### How Brain Orchestration Works
No LLM determines the execution state. Instead:
- **`Sense`**: The brain reads the current `state.json`, task DAG, active leases, and local files.
- **`Decide`**: It determines which frontier tasks are ready to run and whether leases are expired.
- **`Dispatch`**: It issues a task dispatch.

#### Leases and ACP Event Log
When a task is dispatched, the brain writes a `dispatch` event to `acp.jsonl` (Activity Control Protocol) and registers a lease in `session.json` with a 15-minute Time-To-Live (TTL). 

If a task is not verified and completed within the TTL, the lease expires, and the brain will re-dispatch the task on the next step.

#### Crash Safety: Checkpoint, Resume, Cancel
Each dispatch fsyncs a **write-ahead checkpoint** naming a deterministic mission id
(`session/step/task`) *before* the dispatch becomes visible in `acp.jsonl`. If the
process dies between the two, `specd brain resume <spec>` finds the mission id
absent from the ledger and re-issues exactly that dispatch; if it died after the
ledger append, resume finds it present and does not re-issue. Recovery converges
with **zero double-dispatch**. Racing resumes are serialized by a session-revision
CAS, so exactly one holder proceeds.

`specd brain status` derives `crashed` from a checkpoint that outran the ledger — it
is never a persisted state. `specd brain cancel` drives the session to a terminal
`cancelled` state and releases its lease without touching task or evidence state;
`step`/`run`/`resume` are refused on a terminal session. See ADR 0006 for the full
recovery contract and the `pause`/`directive` deferral.

### Authority Gate (Fail-Closed)
By default, the brain runs in read-only mode to prevent unintended system mutations. To allow task dispatching and write operations:
- Use the **`--authority`** flag to grant dispatch authority.
- Without it, `specd brain step` will log planned actions but execute no changes.

### Config Snippets and Digest Drift
- `specd mcp --config claude-code [--root <path>] [--spec <slug>]` prints a
  paste-ready MCP server config — no hand-written JSON. Unknown hosts exit 2
  listing the known ones.
- `specd handshake bootstrap` reports a **palette digest** (SHA-256 of the
  `help --json` command palette) and a **config digest**. Cache them; on the next
  handshake pass them back with `--expect-palette-digest`/`--expect-config-digest`.
  A mismatch exits 1 naming which drifted, so you know to re-fetch the palette
  before relying on a stale cached copy.
- `specd init --repair`/`--refresh` re-sync specd-managed regions (wrapped in
  `<!-- specd:managed:<asset>:v<N> -->` markers) from the embedded templates.
  Content **inside** the markers is regenerated; content **outside** is preserved.
  Use `--dry-run` to preview before writing.

### Cost Telemetry (Stored, Never Computed)
Host workers report their own usage; specd only records it. Pass the optional
`--tokens <int> --cost <decimal> --duration-ms <int>` flags to `specd verify` and
`specd task complete` and the values are stored **verbatim** on the evidence
record. specd never counts tokens, estimates, or does float math on money —
aggregation in `report --metrics` uses exact decimal arithmetic. Every field is
optional; malformed values fail closed (exit 2). ACP claim/report ledger records
carry the same telemetry alongside a per-task attempt number (a countable fact,
not a stored counter), the git HEAD, the changed-files list, and a verify-record
reference.
