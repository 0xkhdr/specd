# Spec 06 — Agent-Agnostic Integration

> **Authoring order:** 8 / 12 · **Critical path:** the P5 floor (feeds 07/09)
> **Sources:** `fresh-start/06-agent-agnostic-integration.md`, paper pp.28–29
> **ADRs:** ADR-8
> **Reference:** `reference/docs/agent-integration.md`, `reference/internal/integration/*`, `reference/internal/core/{agents,scaffold,paths}.go`, `reference/internal/core/embed_templates/{AGENTS.md,roles/*,steering/*}`

Any coding agent (Claude Code, Cursor, Copilot, a bespoke script) integrates with specd through
one standardized floor — a pasteable `--config` snippet — and, where a managed adapter exists,
gets safe detect/install/verify. Roles and steering give every host the same behavioral
contract regardless of vendor.

---

## 1. Purpose & principles
- **Principles owned:** P5 (Agent-Agnostic by Design), P8 (Steering as Constitution), P1
  (agent creates via injected role prompts; harness enforces).
- **Paper concept:** *instructions / rule files* — "Configuring the Harness" (pp.28–29): the
  developer "provides the Instructions and Rule Files … that the harness will load and make
  available to the model."

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Four roles + per-task binding + dedup injection | **KEEP** | Clean P5. `reference/internal/core/embed_templates/roles/*` |
| Steering constitution (agent-authored `product/tech/structure`) | **KEEP** | P8. `reference/.../steering/*` |
| AGENTS.md marker-based merge | **KEEP** | Preserves user content; safe, idempotent. `reference/internal/core/agents.go` |
| `--config` snippet floor | **KEEP as primary path** ("never remove" invariant) | The real portability guarantee |
| Host adapters | **SIMPLIFY** to a 5-method `HostAdapter`; ship 0–1 reference adapter in v1 | Prevent sprawl |
| `--inline-roles` fallback | **KEEP** | Back-compat floor for hosts without asset paths |

**Four roles:** 🔍 `scout` (read-only explore), 🛠️ `craftsman` (write + verify), 🧪 `validator`
(read-only test run), 🛡️ `auditor` (read-only diff audit). **Minimal surface:** `init`
scaffolds roles + steering + AGENTS.md; `handshake` (Spec 07) surfaces integration + policy.

## 3. Requirements (EARS)
- **R6.1** When a task declares `role: <name>`, the system shall make that role's prompt
  available to the host exactly once per response via the shared `assets` map.
- **R6.2** When a host cannot resolve asset paths and passes `--inline-roles`, the system shall
  emit the full role prompt text per packet.
- **R6.3** When `init` runs, the system shall scaffold `.specd/roles/*`, `.specd/steering/*`,
  and an `AGENTS.md` without overwriting user-authored regions outside the managed markers.
- **R6.4** When `init` merges AGENTS.md into an existing file, the system shall replace only
  the marker-delimited section and preserve all other content.
- **R6.5** When any host requests integration, the system shall be able to emit a working
  `--config` snippet even if no managed adapter exists for that host.
- **R6.6** When a managed adapter installs, the system shall keep all writes project-scoped by
  default, preserve unrelated JSON/TOML keys, and record ownership in `.specd/integrations.json`.
- **R6.7** When an adapter is registered, the system shall require it to pass the adapter
  conformance kit (detect/plan/install/inspect/verify).
- **R6.8** The system shall not bind a read-only role (scout/validator/auditor) to a write task.

## 4. Design

### Module boundaries
- `internal/integration/{registry.go,<host>.go}` — adapters. `internal/core/agents.go` —
  marker merge. `internal/core/scaffold.go` + `embed_templates/{roles,steering,AGENTS.md}`.

### Key types
- `HostAdapter interface { Detect; Plan; Install; Inspect; Verify }`; `IntegrationOwnership`
  (in `.specd/integrations.json`); `Role{ name, prompt path, read-only flag }`.
- Role-prompt injection deduplicated via a shared top-level `assets` map (`role/<name>` →
  path), reused across same-role packets. Subagent modes `inline` (default) / `delegate` via
  `roles.subagent_mode`.

### On-disk contracts
- `.specd/roles/*.md`, `.specd/steering/*.md` (`reasoning/workflow/product/tech/structure/
  memory`), `.specd/integrations.json`, `AGENTS.md` (managed markers).

### External interfaces
- The `--config` snippet format; `HostAdapter`; the role prompt asset map consumed by Spec 07
  (MCP/handshake) and Spec 09 (workers are role-bound).

## 5. Invariants preserved (ADR-8)
Snippet floor never removed; marker-merge preserves user content; role dedup; read-only roles
cannot be bound to write tasks.

## 6. Cross-domain dependencies
- Feeds: Spec 07 (handshake surfaces integration/policy; MCP shares the asset map), Spec 08
  (role prompt is a manifest item), Spec 09 (workers role-bound).
- Depends on: Spec 02 (`init` scaffolds into `.specd/`).

## 7. Risks & open questions
- **Risk:** adapter proliferation re-bloats the tree. → snippet is primary; ship ≤1 reference
  adapter in v1; conformance kit gates any new one.
- **Decision:** ship snippet-only integration for MVP, with a tiny conformance kit. Managed
  adapters are follow-up/plugin work unless a real partner proves one necessary.
- **Doc cleanup:** move all Brain/Pinky material out of the integration doc into Spec 09's doc.
