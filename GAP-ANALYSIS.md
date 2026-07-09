# specd — Gap Analysis & Action Plan

**Scope:** two domains the project must close before specd can claim "the agent never needs
`specd help` to drive the harness":

1. **Agent-native workflow knowledge** — the instructions an agent receives (AGENTS.md,
   roles, steering, MCP tool surface) must teach the full workflow: which command, at which
   phase, with which flags, and when to prefer MCP tools over the CLI.
2. **`specd init` host scaffolding** — what init writes for Claude Code and Codex hosts
   (agents, MCP server registration, host-native config) is today incomplete or silently
   inert.

Every finding below cites the code that proves it. Priorities: **P0** = violates a stated
specd principle (fail-closed, no silent no-op, single source of truth) or blocks the agent
loop; **P1** = real capability gap; **P2** = polish/consistency.

---

## Domain 1 — Agent-native workflow knowledge

### What exists today (the good foundation)

- `internal/core/commands.go` is a genuine single source of truth: every verb declares
  usage, description, flags (with types/enums/defaults), allowed phases, exit codes, and
  examples, versioned by `HelpSchemaVersion`. Help, dispatch enforcement, and the MCP tool
  schemas all derive from it.
- `specd init` writes `AGENTS.md` (from `internal/core/embed_templates/AGENTS.md`) plus
  `.specd/roles/*.md` and `.specd/steering/*.md` as managed regions with repair/refresh.
- `specd mcp` serves the palette as stdio MCP tools; `specd handshake bootstrap` gives
  digest pinning so a prompt can detect palette/config drift.

### Gap 1.1 — AGENTS.md is a philosophy card, not an operational manual (P0)

`embed_templates/AGENTS.md` is 39 lines: the 5-step loop, the four roles, four guardrails,
and the on-disk surface. An agent that loads only this file still does not know:

- the **command palette** (names, flags, positional shapes) — it must run `specd help`
  or `specd help <verb>` per command, which is exactly what we want to eliminate;
- the **valid `approve` gate names**. `runApprove` accepts any `core.Status`
  (`requirements`, `design`, `tasks`, `executing`, `verifying`, `complete`), but no
  agent-facing text lists them — help examples show only `requirements`/`design`
  (`internal/core/commands.go:152`), the user guide stops at `tasks`;
- **exit-code semantics** (0 success / 1 gate-verify failure / 2 usage or fail-closed) —
  declared per verb in metadata, never surfaced in AGENTS.md;
- the **escalation ratchet and override flow** (`specd task <id> --override --reason`),
  the **midreq/decision** audit commands, the **memory flywheel**, **criterion evidence
  mode** (`verify --criterion`), the **review → submit endgame**, and **cross-spec
  links/program view** — all present in docs/, absent from the file the agent actually loads;
- an explicit prohibition on **hand-editing `.specd/` state** (tasks.md ✅ markers,
  `state.json`). `docs/agent-integration.md` says "the agent never mutates `.specd/` state
  directly"; the embedded AGENTS.md never says it.

**Action:** generate a **"Command palette" managed section of AGENTS.md from
`core.Commands` at init time** — same generation discipline as `mcp.CoreTools()`, so the
file can never drift from the binary (a conformance test enforces it, see 1.6). Add
hand-authored sections for: phase→command map, approve gate list, escalation/override,
mid-stream records, endgame, and the "never hand-edit `.specd/`" guardrail.

### Gap 1.2 — No phase → command map anywhere agent-facing (P0)

`steering/workflow.md` numbers phases 0–5 but names commands only for intake and verify.
The agent needs one table it can follow mechanically:

| Phase (status) | Author | Then run |
|---|---|---|
| intake | — | `specd new <slug>` |
| requirements | `requirements.md` (EARS) | `specd check <slug>` → `specd approve <slug> requirements` |
| design | `design.md` past stub | `specd check <slug>` → `specd approve <slug> design` |
| tasks | `tasks.md` DAG | `specd check <slug>` → `specd approve <slug> tasks` |
| executing | code, one task at a time | `specd next` → `specd context` → edit → `specd verify` → `specd task complete` |
| verifying/complete | review report | `specd review <slug>` → `specd submit <slug>` |

**Action:** put this table (with the loop commands inlined) into both the generated
AGENTS.md and `steering/workflow.md`, and keep it in one embedded template rendered into
both so it cannot fork.

### Gap 1.3 — No MCP-vs-CLI instruction; the documented refusal list is wrong (P0)

`internal/core/manifest_tools.go:18` refuses **eight** verbs over MCP: `approve`, `brain`,
`decision`, `init`, `mcp`, `memory`, `report`, `task`. But:

- `docs/mcp-guide.md` documents only five (`init`, `approve`, `brain`, `task`, `mcp`) —
  `decision`, `memory`, `report` are refused undocumented. An MCP agent that calls
  `task complete` or `memory add` gets a bare `-32001 tool denied by policy` with no prior
  warning anywhere in its instructions.
- AGENTS.md never mentions MCP at all, so an MCP-connected agent has no rule for **when to
  use MCP tools vs the CLI**.

**Action:**
1. Fix `docs/mcp-guide.md` to list the true refusal set, with the rationale per verb.
2. Add an "MCP vs CLI" section to the generated AGENTS.md: *read/plan verbs
   (`status`, `next`, `context`, `check`, `verify`, `handshake`, `help`, `version`,
   `new`, `link`, `unlink`, `review`, `submit`, `midreq`) are callable as MCP tools;
   state-advancing and session verbs (`approve`, `task`, `decision`, `memory`, `report`,
   `init`, `brain`, `mcp`) must be run from the shell where human-in-the-loop applies.*
   Generate this split from `core.ForbiddenTool` so it also cannot drift.
3. Add a docs-lint-style check (or Go test) asserting the mcp-guide refusal list matches
   `ForbiddenTool`.

### Gap 1.4 — MCP tool descriptions are one-liners; "when to use" never ships (P1)

`mcp.CoreTools()` maps only `Command.Description` into the tool description
(`internal/mcp/tools_core.go:21-25`). An MCP-native agent sees "Select the next eligible
task or wave." but not the usage shape, allowed phases, examples, or exit-code meaning —
so it guesses arguments and learns by error.

**Action:** add a `WhenToUse string` field to `core.Command` (one canonical sentence per
verb: what stage, what precondition, what follows), and render
`Description + WhenToUse + Usage + Examples + AllowedPhases` into the MCP tool
description and into the generated AGENTS.md palette. One new metadata field feeds every
surface — no hand-restating (spec 03 C.8 discipline). Bump `HelpSchemaVersion`.

### Gap 1.5 — No deterministic "what do I do now" affordance (P1)

`specd status` reports state but never the next action. The workflow is a deterministic
function of on-disk state — which is exactly specd's thesis — so the harness itself can
answer "what now":

- status `requirements`, gates failing → "fix EARS findings, re-run `specd check`";
- status `tasks`, gates green → "run `specd approve <slug> tasks`";
- status `executing`, frontier non-empty → "run `specd next <slug>` → work `T<n>`";
- frontier empty, tasks incomplete → name the blocking/escalated tasks and the override
  command.

**Action:** add a `next_action` field to `status --json` and a final "Next:" line to the
text render, computed purely from state (no LLM — it's a lookup table over
status × gate-results × frontier). This removes the largest remaining class of
workflow-discovery lookups for both humans and agents.

### Gap 1.6 — Nothing enforces AGENTS.md ↔ palette agreement (P0, enabler for 1.1)

`docs-lint.sh` guards CHEATSHEET ↔ command-reference, and `parity_test.go` guards
MCP ↔ palette, but no check ties the **agent-loaded instructions** to the palette. That is
why AGENTS.md was able to rot into a summary.

**Action:** render the AGENTS.md palette section from `core.Commands` via a template
function, and add a Go conformance test (in `internal/integration/`, next to the existing
snippet-conformance tests) asserting the embedded template's managed palette block equals
the generated output for the current palette. `init --refresh` then updates deployed
projects.

### Gap 1.7 — In-binary usage strings drift from the palette (P2)

Handlers hand-write usage errors that already disagree with `commands.go`:

- `runNext` error says `usage: specd next <slug> [--json|--waves]` — omits `--dispatch`
  (`internal/cmd/registry.go:279` vs `commands.go:180`);
- `runContext` error says `[--json]` — omits `--hud` (`registry.go:247` vs `commands.go:256`).

**Action:** replace hand-written usage strings with `core.CommandByName(name).Usage` so
the metadata is the only author; add a small test that every handler usage error contains
the palette usage string.

### Gap 1.8 — Handshake pinning exists but nothing scaffolded uses it (P2)

`specd handshake bootstrap --expect-palette-digest` is designed for role prompts and CI,
but neither the scaffolded pinky agents nor AGENTS.md mention it.

**Action:** the generated AGENTS.md palette section should embed the current palette
digest and instruct: "before a long-running session, run
`specd handshake bootstrap --expect-palette-digest <digest>`; on exit 1, re-read
AGENTS.md (`specd init --refresh` regenerates it)." The digest is known at generation
time, so this closes the loop between the doc and the binary that wrote it.

---

## Domain 2 — `specd init` scaffolding for Claude and Codex

### What exists today

- `specd init` scaffolds `.specd/roles/`, `.specd/steering/` (managed regions) and merges
  `AGENTS.md` (`internal/core/scaffold.go:11-24`).
- `--agent=pinky` writes `.claude/agents/pinky-*.md`, `.codex/agents/pinky-*.toml`, and a
  managed block in `.codex/config.toml`.
- `specd mcp --config claude-code` prints an `mcpServers` JSON snippet
  (`internal/core/mcpconfig.go`) — but only prints; nothing installs it.
- `specd agents` reports install state — for pinky only (`internal/core/agents.go:91`).

### Gap 2.1 — `--agent=claude` / `--agent=codex` are silent no-ops (P0)

`WriteScaffold` only matches `pinky` (`scaffold.go:18-22`); any other value — including
the `claude`/`codex` hosts listed by `AgentHosts()` and a typo like `--agent=claud` —
returns success having written nothing agent-specific. This violates the project's own
fail-closed rule ("deferred verbs print a deferral notice… they never silently no-op").

**Action:** validate `--agent` against a known-host registry; unknown values exit 2
listing the known set (same pattern as `mcp --config`). Known-but-unwired hosts must
print an explicit notice, not succeed silently — until 2.3/2.4 wire them for real.

### Gap 2.2 — `specd new <name> --agent=<name>` is a dead flag (P0)

`commands.go:137-145` declares the flag with three examples; `runNew`
(`internal/cmd/lifecycle.go:18`) never reads `flags` at all. The palette advertises
behavior the binary does not have — the exact drift the metadata system exists to prevent.

**Action:** either implement (record the intended harness in `state.json` so dispatch and
reports can attribute work) or remove the flag and its examples. Decide and record via
`specd decision`; do not leave it advertised-but-inert.

### Gap 2.3 — Claude Code scaffolding is missing or malformed (P0)

Three concrete problems:

1. **No CLAUDE.md bridge.** Claude Code's primary instruction file is `CLAUDE.md`;
   init writes only `AGENTS.md`. Recent Claude Code versions also read AGENTS.md, but
   guaranteed pickup across versions needs `init --agent=claude` to write/merge a managed
   `CLAUDE.md` block that imports the contract (`@AGENTS.md`) — or at minimum the specd
   loop + guardrails.
2. **Pinky subagent files lack required frontmatter.** Claude Code subagents under
   `.claude/agents/*.md` must start with YAML frontmatter (`name`, `description`,
   optionally `tools`/`model`). `pinkyClaudeAgent()` (`scaffold.go:78-90`) emits a bare
   markdown body — Claude Code will not register these as proper subagents.
3. **Read-only roles aren't mechanically read-only.** Claude Code supports a real `tools:`
   allowlist, yet scout/validator/auditor scaffolds rely on prose ("You may NOT write").
   specd's whole philosophy is *harness enforcement over prompt trust*: emit
   `tools: Read, Grep, Glob, Bash` for read-only roles and the full set only for
   craftsman.

**Action:** rewrite `pinkyClaudeAgent` to emit frontmatter (name, description tuned for
auto-delegation, role-scoped tools); have `--agent=claude` write the CLAUDE.md managed
block. Extend the conformance tests to assert frontmatter presence and the tool split.

### Gap 2.4 — No MCP server registration at init; Codex MCP unsupported (P1)

- `specd mcp --config` knows exactly one host, `claude-code`, and only prints the snippet.
  Claude Code reads project-scope MCP servers from `.mcp.json` — init could merge a
  managed `specd` entry there directly (`init --agent=claude` or an explicit `--mcp` flag),
  making the palette available as tools with zero manual steps.
- Codex configures MCP via `[mcp_servers.*]` tables in `config.toml`; specd offers neither
  a `--config codex` snippet nor a merge into the `.codex/config.toml` block it already
  manages for pinky.

**Action:** (a) add `codex` (and plausibly `cursor`) to `MCPHosts()` with correct snippet
shapes; (b) teach `init --agent=claude` to merge `.mcp.json` and `init --agent=codex` to
merge the `mcp_servers` block, both through the existing managed-region machinery so
`--repair/--refresh/--dry-run` cover them. Verify the exact current host schemas against
upstream docs during implementation — especially the Codex `[agents.*]` table the pinky
scaffold already writes (`agents.go:52-67`), which predates current Codex config and needs
re-validation.

### Gap 2.5 — `specd agents` discovery only knows pinky (P1)

`DiscoverAgents` returns a single hardcoded pinky entry. Once claude/codex artifacts exist
(2.3/2.4), `specd agents` must report their install state too: CLAUDE.md block present,
`.mcp.json` entry present/valid, frontmatter valid, `.codex/config.toml` blocks present.
This is also the natural repair driver: `agents` names what is missing/invalid;
`init --agent=<host>` (or `--repair`) fixes it.

**Action:** table-drive `DiscoverAgents` from the same host registry as `--agent`
validation, one discovery spec per host artifact set.

### Gap 2.6 — `specd init` succeeds silently with no next step (P2)

Plain `specd init` prints nothing on success (`runInit` → `WriteScaffold` → `return nil`).
A first-time user (or agent) gets no confirmation of what was written and no pointer to
`specd new`.

**Action:** print the scaffold manifest (created vs preserved) and a one-line next step
("next: `specd new <slug>`, then read AGENTS.md"). Deterministic, derived from what was
actually written.

---

## Cross-cutting corrections

| # | Item | Priority |
|---|---|---|
| X1 | `docs/mcp-guide.md` refusal list ≠ `ForbiddenTool` — fix and add drift check (see 1.3) | P0 |
| X2 | Handler usage strings ≠ palette `Usage` — derive from metadata (see 1.7) | P2 |
| X3 | Docs mention approve gates only up to `tasks`; document the full status set incl. `executing`/`verifying`/`complete` in command-reference + CHEATSHEET (update both together — docs-lint) | P1 |
| X4 | `AgentHosts()` `Install: "none"` rows are dead metadata once 2.3/2.4 land — make it the single host registry driving `--agent` validation, scaffolding, discovery, and MCP snippets | P1 |

---

## Action plan

Dogfood the harness: run each wave as a specd spec (`specd new <slug>`), with the tasks
below as the task DAG. Waves are ordered so every later wave builds on machinery from the
earlier one; tasks inside a wave are parallelizable unless `depends-on` says otherwise.

### Wave 1 — `agent-palette` (P0 core: instructions become generated, not hand-written)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | Add `WhenToUse` to `core.Command`; author one sentence per verb; bump `HelpSchemaVersion` to 2 | `internal/core/commands.go` | `go test ./internal/core -run Command -count=1` |
| T2 | Palette renderer: `core.RenderAgentPalette()` → markdown table (usage, when-to-use, phases, exit codes, examples) + palette digest line | `internal/core/` (new file) | `go test ./internal/core -count=1` |
| T3 | Regenerate `embed_templates/AGENTS.md`: keep philosophy header; add generated palette section, phase→command table, approve-gate list, escalation/override, midreq/decision/memory, endgame, "never hand-edit `.specd/`", MCP-vs-CLI split from `ForbiddenTool`, handshake pinning instruction (depends: T1, T2) | `internal/core/embed_templates/AGENTS.md`, `internal/core/scaffold.go` | conformance test below |
| T4 | Conformance test: embedded AGENTS.md palette block == `RenderAgentPalette()` output; MCP-split section == `ForbiddenTool` (depends: T3) | `internal/integration/` | `go test ./internal/integration -count=1` |
| T5 | Enrich MCP tool descriptions with WhenToUse/Usage/Examples/AllowedPhases (depends: T1) | `internal/mcp/tools_core.go` | `go test ./internal/mcp -count=1` |
| T6 | Fix `docs/mcp-guide.md` refusal list; update `docs/command-reference.md` **and** `docs/CHEATSHEET.md` for schema v2 + full approve-gate set | `docs/` | `./scripts/docs-lint.sh` |

### Wave 2 — `init-hosts` (P0/P1: scaffolding becomes real and fail-closed)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | Host registry: single table (claude, codex, pinky) driving `--agent` validation; unknown agent exits 2 listing known hosts; resolve the `new --agent` dead flag (implement or remove, record decision) | `internal/core/agents.go`, `internal/cmd/registry.go`, `internal/cmd/lifecycle.go` | `go test ./internal/cmd -count=1` |
| T2 | Claude scaffold: YAML frontmatter on `.claude/agents/pinky-*.md` with role-scoped `tools:`; managed CLAUDE.md block importing AGENTS.md (depends: T1) | `internal/core/scaffold.go` | `go test ./internal/core -run Scaffold -count=1` |
| T3 | MCP hosts: add `codex` snippet to `MCPHosts()`; `init --agent=claude` merges managed `.mcp.json` entry; `init --agent=codex` merges `mcp_servers` block; validate current host schemas upstream, incl. the existing `[agents.*]` Codex block (depends: T1) | `internal/core/mcpconfig.go`, `internal/core/scaffold.go` | `go test ./internal/core ./internal/cmd -count=1` |
| T4 | Extend `DiscoverAgents` to all hosts + new artifacts; `specd agents` reports missing/invalid per host (depends: T2, T3) | `internal/core/agents.go` | `go test ./internal/core -run Discover -count=1` |
| T5 | Init success output: scaffold manifest + next-step line; `--dry-run` covers new artifacts | `internal/cmd/registry.go` | `go test ./internal/cmd -run Init -count=1` |
| T6 | Docs: user-guide + agent-integration sections for host scaffolding; command-reference + CHEATSHEET together | `docs/` | `./scripts/docs-lint.sh` |

### Wave 3 — `workflow-affordances` (P1/P2 polish)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | `status` `next_action`: pure lookup over status × gates × frontier; JSON field + text "Next:" line | `internal/cmd/registry.go`, `internal/core/` | `go test ./internal/cmd -run Status -count=1` |
| T2 | Handler usage errors derive from palette `Usage`; parity test | `internal/cmd/*.go` | `go test ./internal/cmd -count=1` |
| T3 | Steering `workflow.md` gets the phase→command table (shared source with AGENTS.md) | `internal/core/embed_templates/steering/workflow.md` | conformance test |
| T4 | Handshake digest embedded in generated AGENTS.md; pinky agent prompts instruct digest pinning | `internal/core/scaffold.go` | `go test ./internal/integration -count=1` |

### Sequencing & invariants to preserve

- Ship Wave 1 before Wave 2: the CLAUDE.md/AGENTS.md content that init installs should
  already be the generated, non-drifting version.
- Every wave: `gofmt -l .` empty, `go vet ./...`, `go test ./... -race -count=1`,
  `./scripts/test-lint.sh`, `./scripts/docs-lint.sh`; zero new dependencies (`go mod tidy`
  clean); all new scaffold writes go through `core.AtomicWrite` + managed-region markers so
  `--repair/--refresh/--dry-run` keep working; no LLM in any new path (`next_action` and
  the palette renderer are pure functions of on-disk state/metadata).
- `HelpSchemaVersion` bump (Wave 1 T1) is the only consumer-visible contract change;
  document it in CHANGELOG under the versioning policy.
