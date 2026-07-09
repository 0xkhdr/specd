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

---
---

# Part II — Deep dive: remaining domains

A second pass over the domains Part I did not cover: dispatch/CLI parsing, evidence & verify
integrity, orchestration (Brain/Pinky), the context manifest, configuration, and the
concurrent-execution model. Same conventions: every finding cites code; **P0** = breaks a
stated invariant or renders a shipped feature unusable, **P1** = real capability gap,
**P2** = consistency/polish.

**What is healthy and needs no work:** CI is thorough (race + count=2 matrix, coverage floor
ratchet, staticcheck, govulncheck, shellcheck, perf gate, five concurrency-stress jobs);
atomic-write/fsync discipline in `io.go` is correct; the ACP ledger's write-ahead
checkpoint/resume protocol (`brain_run.go:134-172`, `brainResumeLocked`) is genuinely careful;
the scale envelope is measured and documented with regression pins. The gaps below sit next
to that quality, which is what makes them worth closing.

---

## Domain 3 — Dispatch & CLI parsing (the enforcement choke point)

### Gap 3.1 — Unknown flags are accepted silently (P0)

`checkFlagEnums` (`internal/cmd/dispatch.go:41-52`) validates only flags that exist in
metadata *and* declare an enum; a flag absent from metadata is "left to the handler" — and
handlers read specific keys, so anything else vanishes. `specd status payments --jsn` (typo)
or `specd verify payments T3 --revert-on-fial` succeeds as if the flag were never passed.
For an agent this is the worst failure shape: it believes it requested a behavior and gets
no signal it didn't. The palette declares every legal flag per verb — the dispatcher has
everything it needs to fail closed (exit 2, "unknown flag --jsn; command flags: …") and
chooses not to.

**Action:** in `Run`, reject any flag not declared in `meta.Flags` (with an allowance for
the reserved MCP `args` key). This is a strict tightening consistent with "unknown verbs
fail closed."

### Gap 3.2 — Boolean flag followed by a positional swallows the positional (P0)

`ParseArgs` (`internal/cli/args.go:29-36`) treats the next token as the flag's value
whenever a `--flag` isn't `--flag=value` and the next token doesn't start with `-`. So
`specd next --json payments` parses as `json=payments` with **no positional** — the agent
gets a usage error (or worse, `flagEnabled` silently treats `json` as false). The parser is
metadata-blind even though every flag declares `TakesValue`.

**Action:** thread `core.CommandByName(command).Flags` into parsing (parse the verb first,
then flags): a declared boolean flag never consumes a following token; a declared value
flag always does. Flag-order robustness is table stakes for agent callers.

### Gap 3.3 — In-metadata invalid examples (P2)

`brain`'s example `specd brain start payments --authority` (`commands.go:317`) attaches
`--authority` to `start`, but authority only affects `step`/`run` (`brain_run.go:87-88`).
Examples are part of the machine-readable contract agents will imitate — they must be
canonical. Audit all `Examples` against actual handler behavior once 3.1 lands (an unknown
or inert flag in an example should fail a test).

---

## Domain 4 — Evidence & verify integrity

### Gap 4.1 — The evidence gate and `CompleteTask` disagree on what "passing" means (P0)

`CompleteTask` requires `HeadPinned` (rejects `git_head: "unknown"`,
`task_complete.go:21`), but the **evidence gate** and the Brain's worker-report check use
`HasPassingEvidence` (`evidence.go:110-113`), which only requires `GitHead != ""` — the
`"unknown"` sentinel passes. Consequence: a task hand-marked ✅ in `tasks.md` backed by an
unpinned record sails through `specd check`'s evidence gate (`gates/core.go:157`) even
though `task complete` would have refused it. The gate is the audit backstop for exactly
this hand-editing scenario, so it must be at least as strict as the transition it audits.

**Action:** fold `HeadPinned` into `HasPassingEvidence` (one definition of "passing",
used by gate, completion, and `brain_worker.go:18` alike).

### Gap 4.2 — Verify commands have no timeout (P1)

`runVerify` passes `context.Background()` (`registry.go:594`) and `verify.Run` applies no
deadline. A verify line that hangs (test deadlock, network wait) hangs `specd verify`
forever — and under Brain, wedges the mission until the 15-minute lease TTL, with nothing
killing the child process. `submit` already has `SubmitConfig.TimeoutSecs` (default 120s,
`config_loader.go:59-66`); task verify — the far hotter path — has nothing.

**Action:** add `verify.timeout_seconds` config (generous default, e.g. 600s; 0 = none),
plumb `context.WithTimeout` into `verifyexec.Run`, and record a timeout as a failing
evidence record (distinct exit-code note) so the escalation ratchet counts it.

### Gap 4.3 — Evidence pins HEAD, not the tree that actually passed (P1)

Evidence claims "exit 0 pinned to a real git HEAD," but verify runs against the **working
tree**, which is normally dirty at verify time (the task's edits aren't committed yet). The
record pins the *parent* commit; the code that passed may differ from anything ever
committed. This weakens the audit-trail claim more than the docs acknowledge.

**Action:** record `git_dirty: true/false` (one `git status --porcelain` call) in
`EvidenceRecord` — cheap, additive, honest. Optionally follow with a stricter opt-in
(`verify.require_clean: true`) for teams that want evidence == committed tree, and surface
dirty-evidence counts in `report`.

### Gap 4.4 — Sandbox cannot be required by policy (P1)

`SECURITY.md`'s threat model names hostile verify-line content, and `--sandbox` exists —
but only as a per-invocation flag. Nothing lets a project declare "verify always runs
sandboxed"; one forgotten flag (or an agent that doesn't know to pass it) runs hostile
content unsandboxed. `SecurityConfig` (`config_loader.go:29-33`) governs scanners only.

**Action:** add `verify.sandbox: off|preferred|required` config. `required` fails closed
(exit 2) when `bwrap` is absent — same fail-closed shape the flag already has.

### Gap 4.5 — Evidence-log reader inconsistencies (P2)

`LoadEvidence` uses a default `bufio.Scanner` (64KB line cap, `evidence.go:65`) while
`LoadEvidenceRecords` deliberately raises it to 1MB (`evidence.go:96`) — a record over 64KB
(large telemetry/command) breaks completion but not history replay. Evidence appends in
`runVerify` also happen outside `WithSpecLock` (`registry.go:612`), so two concurrent
verifies interleave appends with only O_APPEND atomicity protecting line integrity.

**Action:** share one reader with the 1MB buffer (or read whole-file like `ReadACP`,
which fixed this exact bug — see its comment at `acp.go:146-149`); take the spec lock
around the append.

---

## Domain 5 — Orchestration (Brain/Pinky): shipped but not reachable

### Gap 5.1 — No command can put a spec in `orchestrated` mode (P0)

`brain start` hard-requires `state.Mode == "orchestrated"` (`brain_run.go:120`), but the
`Mode` type declares only `default` and `agent` (`state.go:60-65`), `InitialState` writes
`default`, and **no production code path ever writes `orchestrated`** — only test helpers
do. The only way a user can satisfy the precondition is to hand-edit `state.json`, which
the project's own guardrails forbid. The entire Brain feature — leases, ACP ledger,
checkpoint/resume, five CI stress jobs — is unreachable through the CLI.

**Action:** add the mode transition as a first-class, auditable verb (e.g.
`specd brain enable <spec>`, or `new --mode=orchestrated`): CAS on state, records an
`approval`-style ledger entry, declares `Orchestrated` as a real `Mode` constant. Document
it in the palette.

### Gap 5.2 — Workers have no verb to claim or report missions (P0)

The ACP ledger defines `claim` and `report` event kinds with worker-rigor fields
(attempt, git_head, changed_files, verify_ref — `acp.go:17-44`), and the scaffolded Pinky
prompts say "claims one dispatched mission… reports evidence." But `AppendClaim` is called
from **nowhere** in production code, and no CLI/MCP verb exposes claim or report. A Pinky
worker literally cannot execute its own instructions through the harness; the
dispatch → claim → work → report loop is closed on paper and open in the binary.
(`acceptWorkerReport` in `brain_worker.go` is likewise dead code no verb reaches.)

**Action:** add `specd mission claim <spec> <mission-id>` and
`specd mission report <spec> <mission-id> [--verify-ref …]` (names illustrative): claim
appends the ACP claim event and re-leases to the worker id; report validates passing
verify evidence via the shared `requirePassingVerify`, appends the report event, and
releases the lease. Update the Pinky role/agent templates to name the real verbs.

### Gap 5.3 — `brain run` is `brain step` with a different name (P1)

Both subcommands fall through to the same single-decision body
(`brain_run.go:49-101`); docs promise "run to a stopping point"
(`docs/agent-integration.md:74`). One decision per invocation may even be the *right*
design (checkpointed, resumable), but then `run` should not exist as a separate verb, or
it should loop until `ActionWait`/`ActionHalt` with per-step checkpoints.

**Action:** either implement the loop (bounded by brakes/deadline) or delete `run` from
the palette and docs. Don't ship two names for one behavior.

### Gap 5.4 — Leases are never released on completion (P2)

A lease is released by cancel, resume-reclaim, or 15-minute expiry (`brainLeaseTTL`,
`brain_run.go:14`) — never by the task actually completing. A fast task's lease lingers,
and `Decide`'s escalation input (`snapshot.Leases`) reasons over stale holds. Wire the
mission-report verb (5.2) to drop the lease; until then completion and leasing are
disconnected bookkeeping.

---

## Domain 6 — Context manifest: doesn't deliver its own promise

### Gap 6.1 — The task's declared `files:` never enter the manifest (P0)

The pitch (docs/agent-integration.md: "only the files that task needs") and the task
schema (`files:` is a required column) both say the manifest scopes the agent to the
task's files. `BuildManifest` (`internal/context/manifest.go:39-70`) assembles: spec
requirements, tasks.md, the task row, the role file, steering, and memory — **`task.Files`
is never read**. The one thing the craftsman actually needs — the code files it may
touch — is absent, so a worker driving from the manifest alone cannot even locate its
scope, and the context-budget gate never accounts for the real payload.

**Action:** add `Kind: "file"` items from the task's parsed `files:` list (existing files
sized via the same `tokensFromBytes`; missing ones noted, since a task may create files).
These are core items — never dropped by the budget; the budget then finally measures what
an agent would truly load. Also consider including `design.md` (the craftsman's contract
authority) at least as `reference-if-needed`.

### Gap 6.2 — Core-item token estimates measure the path string, not the file (P1)

Core items get `EstimateText(kind + path + taskID)` (`manifest.go:51-53`) — requirements.md
"costs" ~10 tokens whether it's 1KB or 400KB, while steering items are sized from real
file bytes (`manifest.go:91`). The context-budget gate is therefore blind to most of the
actual context. Size core items from disk like steering items.

### Gap 6.3 — Manifest paths are inconsistent and partly wrong (P1)

Spec items emit `specs/<slug>/requirements.md` — missing the `.specd/` prefix
(`manifest.go:46-47`) — while steering/memory items emit `.specd/...` correctly. The plain
`specd context` output prints these paths for the agent to open; the spec paths resolve to
the wrong location (top-level `specs/` — which in *this* repo is a different, real
directory, the exact confusion `regress-lint` smell "A" exists to catch). Normalize all
items to root-relative `.specd/...` paths.

### Gap 6.4 — `ModeForTask` knows a ghost role and forgets a real one (P2)

`ModeForTask` (`manifest.go:157-168`) maps `scribe` — a role that exists nowhere else in
the system (roles are scout/craftsman/validator/auditor) — and lets **auditor** fall
through to `craftsman` mode, giving the read-only auditor a write-mode manifest. Fix the
switch; derive it from the same role set the `roles` gate validates so it can't drift.

---

## Domain 7 — Configuration: silently fail-open

### Gap 7.1 — Config diagnostics are dropped at nearly every call site (P0)

`LoadConfig` returns diagnostics; only `brain start` checks them
(`brain_run.go:107-112`). Everywhere else — `contextBudget` (`registry.go:540-543`),
`check --security`, `handshake`, submit/escalation/criteria config reads — the pattern is
`cfg, _ := core.LoadConfig(...)`: a malformed `project.yml` **silently degrades to
defaults**. A team that sets `review.required: true` with a one-space indent error gets no
review gate and no warning — the opposite of fail-closed, on the file that arms the
gates.

**Action:** load config once per invocation at the dispatch boundary; error-severity
diagnostics fail closed (exit 2) for every verb except `help`/`version`. This is a
one-place fix given `Run` is already the choke point.

### Gap 7.2 — No verb shows the effective config (P1)

Config semantics are spread across defaults, `project.yml`, and env overrides, with real
behavioral toggles (criteria/review/escalation/security/orchestration), yet there is no
`specd config` to print the merged result and its diagnostics. `handshake` proves a
digest of it — you can verify config *changed* but not see *what it is*.

**Action:** add `specd config [--json]`: effective config, source of each value
(default/global/project/env), and all diagnostics. Cheap, pure, and it completes the
handshake story.

### Gap 7.3 — `project.yml` at the repo root is a collision-prone, underdocumented location (P2)

The config file is a generically-named root file (`registry.go:107` et al.), not
`.specd/config.yml` beside everything else specd owns; `ConfigPaths.Global` exists but no
caller ever populates it (dead field). AGENTS.md never mentions the file at all — an agent
can't discover the knobs. Decide: either move to `.specd/config.yml` (with a deprecation
read of the old path) or document `project.yml` prominently; delete or implement the
global path.

---

## Domain 8 — Concurrency model: waves promised, one bench provided

### Gap 8.1 — Parallel wave execution has no working-tree isolation story (P1)

The philosophy sells concurrent waves ("Waves, Not Lines"), `next --waves` exposes them,
and Brain dispatches missions — but all workers share **one working tree and one repo
root**: `verify` runs at root (`registry.go:595-599`), `--revert-on-fail` snapshots and
reverts the **entire** tree (`registry.go:769-783`), so a failing task's revert destroys
a parallel worker's in-flight edits; evidence for both pins the same HEAD while testing a
tree containing the union of their changes. Nothing enforces or even documents that
execution is serial-per-repo.

**Action:** state the contract now, build the isolation later. Short term: document
(AGENTS.md + steering/workflow) that within one repo clone, tasks execute serially —
waves are *scheduling* concurrency (safe to hand to N workers only across N clones/
worktrees). Medium term: `verify --worktree` (or Brain-managed `git worktree` per
mission) to make single-clone parallelism real. `--revert-on-fail` should also refuse or
scope itself when files outside the task's `files:` are dirty.

### Gap 8.2 — The lock is per-root, not per-spec, contrary to its own docs (P2)

`WithSpecLock` takes the **project root** and uses a single `.specd/specd.lock`
(`lock.go:16,31,158`); CLAUDE.md and contributor docs say "per-spec work is serialized by
a reentrant per-spec lock." Two specs' state writes serialize against each other, and
`status --program` style workflows funnel through one file lock. Correct the docs (fine
for current scale) or key the lock by `root+slug` (the lock map already supports it) —
the misdocumented invariant is the bug either way.

---

## Consolidated priority table (Part II)

| Finding | Domain | Priority |
|---|---|---|
| 5.1 Brain unreachable: nothing sets `orchestrated` mode | Orchestration | **P0** |
| 5.2 No claim/report verbs — Pinky loop can't round-trip | Orchestration | **P0** |
| 6.1 Task `files:` absent from context manifest | Context | **P0** |
| 3.1 Unknown flags silently accepted | Dispatch | **P0** |
| 3.2 Bool-flag/positional parsing bug | CLI | **P0** |
| 4.1 Evidence gate weaker than `CompleteTask` (`HeadPinned`) | Evidence | **P0** |
| 7.1 Config parse errors silently fall back to defaults | Config | **P0** |
| 4.2 No verify timeout | Verify | P1 |
| 4.3 Evidence ignores dirty tree | Evidence | P1 |
| 4.4 Sandbox not requirable by policy | Security | P1 |
| 5.3 `brain run` ≡ `brain step` | Orchestration | P1 |
| 6.2 Budget measures path strings, not files | Context | P1 |
| 6.3 Manifest paths missing `.specd/` prefix | Context | P1 |
| 7.2 No `specd config` verb | Config | P1 |
| 8.1 No parallel-execution isolation contract | Concurrency | P1 |
| 3.3 Invalid palette examples | Dispatch | P2 |
| 4.5 Evidence reader buffer/lock inconsistencies | Evidence | P2 |
| 5.4 Leases never released on completion | Orchestration | P2 |
| 6.4 `ModeForTask` ghost role / auditor fallthrough | Context | P2 |
| 7.3 `project.yml` location + dead global path | Config | P2 |
| 8.2 Lock is per-root, docs say per-spec | Concurrency | P2 |

## Action plan — additional waves

Continue the wave numbering from Part I (Waves 1–3). Wave 4 is pure correctness and can
ship independently of Part I; Waves 5–6 build on it.

### Wave 4 — `enforcement-integrity` (P0 correctness at the choke points)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | Reject undeclared flags at dispatch (reserve MCP `args`); audit palette examples against handlers | `internal/cmd/dispatch.go` | `go test ./internal/cmd -count=1` |
| T2 | Metadata-aware flag parsing: booleans never consume the next token (depends: T1) | `internal/cli/args.go` | `go test ./internal/cli -count=1` |
| T3 | Unify "passing evidence": `HasPassingEvidence` requires `HeadPinned`; gate/worker/complete share it | `internal/core/evidence.go`, `internal/core/gates/core.go` | `go test ./internal/core/... -count=1` |
| T4 | Config fail-closed: load once in `Run`, error diagnostics exit 2 (except help/version) | `internal/cmd/dispatch.go`, `internal/core/config_loader.go` | `go test ./internal/cmd -count=1` |
| T5 | Evidence reader: shared 1MB-safe loader; append under spec lock | `internal/core/evidence.go`, `internal/cmd/registry.go` | `go test ./internal/core -run Evidence -count=1` |

### Wave 5 — `orchestration-reachable` (make Brain/Pinky a real loop)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | `Orchestrated` mode constant + auditable enable verb (CAS + ledger record) | `internal/core/state.go`, `internal/cmd/brain_run.go`, `internal/core/commands.go` | `go test ./internal/cmd -run Brain -count=1` |
| T2 | `mission claim` / `mission report` verbs: ACP claim/report events, lease transfer + release, `requirePassingVerify` on report (depends: T1) | `internal/cmd/` (new), `internal/orchestration/acp.go` | `go test ./internal/cmd ./internal/orchestration -count=1` |
| T3 | Resolve `brain run`: loop-until-brake with per-step checkpoints, or remove the alias (record decision) | `internal/cmd/brain_run.go`, docs | `go test ./internal/cmd -run Brain -count=1` |
| T4 | Update Pinky role/agent templates + agent-integration docs to the real verbs; fix `brain start --authority` example (depends: T2) | `embed_templates/`, `docs/`, `internal/core/commands.go` | `go test ./internal/integration -count=1`; `./scripts/docs-lint.sh` |
| T5 | Release lease on task completion; stress job asserts no stale live leases after a full orchestrated run (depends: T2) | `internal/cmd/lifecycle.go`, `scripts/stress-orchestration.sh` | `./scripts/stress-orchestration.sh` |

### Wave 6 — `context-and-execution-truth` (manifest delivers; contracts stated)

| id | task | files (primary) | verify |
|---|---|---|---|
| T1 | Manifest includes task `files:` as undroppable core items; missing files noted; add `design.md` reference | `internal/context/manifest.go` | `go test ./internal/context -count=1` |
| T2 | Size all items from disk; normalize every path to `.specd/...` (depends: T1) | `internal/context/manifest.go` | `go test ./internal/context ./internal/core/gates -count=1` |
| T3 | Fix `ModeForTask`: drop `scribe`, map auditor read-only; derive from the canonical role set | `internal/context/manifest.go` | `go test ./internal/context -count=1` |
| T4 | `verify.timeout_seconds` + `verify.sandbox: off\|preferred\|required` config; timeout recorded as failing evidence | `internal/core/config_loader.go`, `internal/core/verify/exec.go`, `internal/cmd/registry.go` | `go test ./internal/core/verify ./internal/cmd -count=1` |
| T5 | Evidence honesty: record `git_dirty`; surface dirty-evidence counts in `report`; opt-in `verify.require_clean` | `internal/core/evidence.go`, `internal/cmd/registry.go`, `internal/cmd/report.go` | `go test ./... -count=1` |
| T6 | `specd config [--json]` verb (effective config + per-value source + diagnostics); decide `project.yml` vs `.specd/config.yml`, delete or implement the global path (record decision) | `internal/cmd/`, `internal/core/config_loader.go`, docs | `./scripts/docs-lint.sh` |
| T7 | Concurrency contract: document serial-per-clone execution (AGENTS.md + steering/workflow); scope or guard `--revert-on-fail` against out-of-scope dirty files; correct or implement the per-spec lock claim | `embed_templates/`, `internal/cmd/registry.go`, `internal/core/lock.go`, `CLAUDE.md`, docs | `go test ./... -race -count=1` |

Same standing invariants as Part I apply to every wave: zero new dependencies, atomic
writes, CAS on state, no LLM in any gate/decision path, docs-lint green, and any palette
change updates `docs/command-reference.md` + `docs/CHEATSHEET.md` together with a
`HelpSchemaVersion`/`StateSchemaVersion` bump where the machine contract moves.
