# Agent-Agnostic Integration

This document outlines how agents (e.g., Claude Code, Cursor, Copilot, or custom scripts) integrate with the `specd` harness, use steering rules, and assert role boundaries.

---

## 1. The Standard Config Snippet (The Universal Floor)

The primary integration path for any agent host is a standard, pasteable config snippet. This configuration establishes the harness commands as native tools for the agent.

*Portability Invariant:* Managed adapters are conveniences; the raw command snippet is the universal floor and **must never be removed or disabled** to force adapter adoption.

---

## 2. Injected Agent Roles

Every task in `tasks.md` specifies a `role:`. The harness defines four roles under `.specd/roles/`:

| Role Icon | Role Name | Allowed Operations |
| :--- | :--- | :--- |
| 🔍 | `scout` | Read-only exploration of the codebase. |
| 🛠️ | `craftsman` | Workspace file modification and verification testing. |
| 🧪 | `validator` | Read-only test execution and regression checking. |
| 🛡️ | `auditor` | Read-only diff auditing and code review. |

### Role Prompt Deduplication
To preserve context tokens, role prompts are not repeated. Instead, the harness exports them to a shared `assets` map (e.g., mapping `role/craftsman` to `.specd/roles/craftsman.md`). Hosts without asset path resolution pass the `--inline-roles` flag to inject full prompt text fallback.

*Origin:* Implemented via asset mappings and fallbacks in [agent-integration.md](file:///var/www/html/rai/up/specd/reference/docs/agent-integration.md).

---

## 3. Steering Constitution

The agent's behavior is guided by steering documents located in `.specd/steering/`:

*   `reasoning.md`: Prescribes model thought patterns and verification steps.
*   `workflow.md`: Enforces task selection and lock ordering.
*   `product.md` / `tech.md` / `structure.md`: User-authored architectural stack boundaries.
*   `memory.md`: Agent memory and local context retention.

The harness automatically scaffolds these files during `init` and injects them into the agent's system prompt or workspace context manifest.

---

## 4. AGENTS.md Marker-Based Merge

The `AGENTS.md` file at the repository root contains the active operating brief for agents working in the tree. The harness updates this file using marker-based merging:

```markdown
<!-- specd-brief-start -->
[Managed content here, overwritten by init/update]
<!-- specd-brief-end -->
[User content here, preserved by the merge]
```

The merge algorithm in `agents.go` (`MergeSection`) replaces only the region between the HTML comment markers, ensuring user additions, notes, or overrides are never lost.

*Origin:* Ported from [agents.go](file:///var/www/html/rai/up/specd/reference/internal/core/agents.go).

---

## 5. Host Adapters

For supported environments, a host adapter can automate setup. The `HostAdapter` interface enforces strict safety constraints:

```go
type HostAdapter interface {
	Detect(dir string) bool
	Plan(dir string) (*InstallPlan, error)
	Install(dir string) error
	Inspect(dir string) (*HostStatus, error)
	Verify(dir string) error
}
```

### Safety Rules for Adapters
1.  **Project-Scoped Writes:** Adapters must never write files outside the workspace directory (e.g. no global system settings edits).
2.  **Preserve Unrelated Keys:** When editing host configuration files (like VSCode `settings.json` or Cursor configs), the adapter must load, modify only its owned keys, and write back, preserving user configurations.
3.  **Ownership Ledger:** Installed integrations are registered in `.specd/integrations.json`.
4.  **Conformance Kit:** Every adapter must pass the conformance suite verifying detect, idempotent install, and project-scoped writes.

*Origin:* Modeled from the integration logic in [internal/integration/](file:///var/www/html/rai/up/specd/reference/internal/integration/).
