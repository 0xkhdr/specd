# Domain: Agent-Agnostic Integration

## 1. Purpose & value mapping
- **Principles served:** P5 (Agent-Agnostic by Design), P8 (Steering as Constitution),
  P1 (agent creates via injected role prompts; harness enforces).
- **Paper concept realized:** *instructions / rule files* — the "Configuring the Harness"
  phase (pp.28–29): AGENTS.md, architectural constraints, and the roles the agent may take.
  The paper: the developer "provides the Instructions and Rule Files … that the harness
  will load and make available to the model."
- **Core use case:** any coding agent (Claude Code, Cursor, Copilot, a bespoke script)
  integrates with specd through one standardized floor — a pasteable config snippet — and,
  where a managed adapter exists, gets safe detect/install/verify. Roles and steering give
  every host the same behavioral contract regardless of vendor.
- **If none → CUT:** N/A — P5 is a core principle.

## 2. Current-state analysis (from specd)
- **Reference files read:** `docs/agent-integration.md`, `internal/integration/*`,
  `internal/core/embed_templates/{AGENTS.md,roles/*,steering/*}`, `internal/core/agents.go`
  (`MergeSection`), `internal/core/scaffold.go`, `internal/core/paths.go`.
- **What exists today; key contracts/invariants:**
  - **Four roles** under `.specd/roles/`, bound per task via `role:`: 🔍 `scout`
    (read-only explore), 🛠️ `craftsman` (write + verify), 🧪 `validator` (read-only test
    run), 🛡️ `auditor` (read-only diff audit).
  - **Role-prompt injection is deduplicated:** prompt bytes emitted once per response via a
    shared top-level `assets` map (`role/<name>` → path), reused across same-role packets; a
    5-task wave no longer repeats the prompt 5×. Hosts without asset resolution pass
    `--inline-roles` for full-text fallback. Subagent modes `inline` (default) / `delegate`
    via `roles.subagent_mode`.
  - **Steering constitution** under `.specd/steering/`: `reasoning.md`, `workflow.md`,
    `product.md`, `tech.md`, `structure.md`, `memory.md`; `product/structure/tech` are
    agent-authored (guided by the `specd-steering` skill) — the harness scaffolds and
    enforces but does not perceive the stack.
  - **AGENTS.md marker-based merge** (`agents.go`, `MergeSection`): two files — repo-root
    (for developing specd itself) and template (written to user repos by `init`).
  - **Host adapter contract** (`internal/integration`): a bespoke adapter is justified only
    to safely **detect / plan / install / inspect / verify** without owning unrelated user
    config; register in `DefaultRegistry()`; record ownership in `.specd/integrations.json`.
    The **`--config` snippet is the universal floor** — "never remove it to force adapter
    use."
- **Redundancy / complexity / drift found (evidence):**
  - `docs/agent-integration.md` is 694 lines spanning roles, steering, adapters, subagent
    modes, MCP, *and* the entire Brain/Pinky surface — the doc conflates integration with
    orchestration (domain 09).
  - Adapter surface risks sprawl (one bespoke adapter per host); the snippet floor is the
    real portability guarantee and should be primary, adapters secondary.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Four roles + per-task binding + dedup injection — **KEEP** (clean realization of P5).
  - Steering constitution (agent-authored `product/tech/structure`) — **KEEP** (P8).
  - AGENTS.md marker-merge — **KEEP** (preserves user content; safe, idempotent).
  - `--config` snippet floor — **KEEP as the primary integration path** and hold the
    "never remove" rule as an invariant.
  - Host adapters — **SIMPLIFY** to the smallest contract: a `HostAdapter` interface with
    exactly five methods (`Detect / Plan / Install / Inspect / Verify`), all writes
    project-scoped, ownership recorded, unrelated keys preserved. Ship **zero or one**
    reference adapter in v1; everything else uses the snippet.
  - `--inline-roles` fallback — **KEEP** (back-compat floor for hosts without asset paths).
- **Minimal accurate surface:**
  - Command: `handshake` (domain 07) surfaces integration + policy; `init` scaffolds roles +
    steering + AGENTS.md.
  - Modules: `internal/integration` (`HostAdapter` + `DefaultRegistry`), `agents.go`
    (merge), `scaffold.go` (templates), embedded `roles/*` + `steering/*`.
  - On-disk: `.specd/roles/*.md`, `.specd/steering/*.md`, `.specd/integrations.json`,
    `AGENTS.md`.
- **Architecture & flexibility improvements:**
  - **Adapter conformance test kit:** a shared table-driven test every adapter must pass
    (detect / idempotent install / inspect / verify) — makes "smallest safe adapter" an
    enforced contract, not a guideline.
  - **Snippet-first docs:** every host's integration page leads with the snippet; the
    managed adapter is an optional convenience section.
  - Move all Brain/Pinky material out of the integration doc into domain 09's doc — keep
    this domain about *how any agent talks to the harness*, not orchestration.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When a task declares `role: <name>`, the system shall make that role's prompt available
   to the host exactly once per response via the shared `assets` map.
2. When a host cannot resolve asset paths and passes `--inline-roles`, the system shall
   emit the full role prompt text per packet.
3. When `init` runs, the system shall scaffold `.specd/roles/*`, `.specd/steering/*`, and
   an `AGENTS.md` without overwriting user-authored regions outside the managed markers.
4. When `init` merges AGENTS.md into an existing file, the system shall replace only the
   marker-delimited section and preserve all other content.
5. When any host requests integration, the system shall be able to emit a working `--config`
   snippet even if no managed adapter exists for that host.
6. When a managed adapter installs, the system shall keep all writes project-scoped by
   default, preserve unrelated JSON/TOML keys, and record ownership in
   `.specd/integrations.json`.
7. When an adapter is registered, the system shall require it to pass the adapter
   conformance kit (detect/plan/install/inspect/verify).

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/integration/{registry.go,<host>.go}` (adapters),
  `internal/core/agents.go` (merge), `internal/core/scaffold.go` +
  `embed_templates/{roles,steering,AGENTS.md}`.
- **Key types:** `HostAdapter interface { Detect; Plan; Install; Inspect; Verify }`;
  `IntegrationOwnership` (in `.specd/integrations.json`); `Role` (name, prompt path,
  read-only flag).
- **Data/on-disk contracts:** `.specd/roles/*.md`, `.specd/steering/*.md`,
  `.specd/integrations.json`, `AGENTS.md` markers.
- **Invariants to preserve:** snippet floor never removed; marker-merge preserves user
  content; role dedup; read-only roles cannot be bound to write tasks.
- **External interfaces:** the `--config` snippet format; `HostAdapter`; the role prompt
  asset map consumed by the MCP/handshake surface (domain 07) and orchestration (domain 09).

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — roles & steering scaffold
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T6.1 | craftsman | `internal/core/embed_templates/roles/*`, `internal/core/scaffold.go` | — | `go run . init && ls .specd/roles | wc -l` | four roles scaffolded |
| T6.2 | craftsman | `internal/core/embed_templates/steering/*` | T6.1 | `test -f .specd/steering/workflow.md` | steering constitution scaffolded |
| T6.3 | craftsman | `internal/core/agents.go` | — | `go test ./internal/core -run TestAgentsMergePreservesUser` | marker-merge preserves user content |
### Wave 2 — adapters & injection
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T6.4 | craftsman | `internal/integration/registry.go` | — | `go test ./internal/integration -run TestSnippetFallback` | snippet emitted with no adapter |
| T6.5 | craftsman | `internal/integration/<host>.go` | T6.4 | `go test ./internal/integration -run TestAdapterConformance` | reference adapter passes the kit |
| T6.6 | craftsman | role-injection wiring | T6.1 | `go test ./internal/core -run TestRolePromptDedup` | prompt emitted once per response |
| T6.7 | validator | `internal/integration/conformance_test.go` | T6.5 | `go test ./internal/integration -run TestAdapterConformance` | idempotent install; ownership recorded |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** adapter proliferation re-bloats the tree. Mitigation: snippet is primary; ship
  ≤1 reference adapter in v1; conformance kit gates any new one.
- **Open question:** do we ship any managed adapter in v1, or snippet-only? Proposed:
  snippet-only for MVP, one reference adapter as a follow-up wave.
- **Cross-domain deps:** domain 07 (handshake surfaces integration/policy; MCP shares the
  asset map), domain 08 (role prompt is part of the context manifest), domain 09 (workers
  are role-bound), domain 02 (`init` scaffolds into `.specd/`).
