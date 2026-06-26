# specd — Pending Work

Tracks planned improvements and production-readiness gaps.
Items are staged: each stage must close before the next opens.

---

## Stage 1 — Host Integration Parity

### 1.1 Replace Gemini Adapter with Antigravity CLI Adapter

**What:** Delete `internal/integration/gemini.go` and its test. Add
`internal/integration/antigravity.go` — a `HostAdapter` for the Antigravity
CLI (`agy`). Gemini CLI was sunset June 18, 2026; removal is not optional.

**Why:** Gemini CLI is dead. `DefaultRegistry()` in `registry.go:59` still
registers `NewGeminiAdapter()` — the adapter detects nothing on any machine
that ran the migration, making `specd init` silently fail for all Gemini
ex-users who are now on Antigravity.

**Antigravity CLI — resolved facts (researched 2026-06-26):**

| Property | Value |
|----------|-------|
| Binary | `agy` |
| Project config path | `.agents/mcp_config.json` |
| Global config path | `~/.gemini/config/mcp_config.json` |
| Registration method | **Direct JSON write** — no `agy mcp add` CLI command exists |
| Action kind | `"project-json"` (same as Cursor adapter, NOT `"native-cli"`) |
| Config schema key | `"mcpServers"` (same object shape as other adapters) |

**Resulting `.agents/mcp_config.json` entry specd must write:**
```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "<project-root>"]
    }
  }
}
```

**Implementation delta vs `gemini.go`:**
- `Detect`: executable `"agy"`, project config `".agents/mcp_config.json"`
- `Plan`: action kind `"project-json"` (not `"native-cli"`); no `Command`/`Args`
  fields needed — `installProjectJSON` writes the file directly
- `Install`: call `installProjectJSON` (same helper Cursor uses, in
  `internal/integration/hostutil.go`) targeting `.agents/mcp_config.json`
- `Inspect` / `Verify`: inspect `inspectJSONServer` against same path
- Note: `.agents/` should be committed to VCS — document this in `README.md`

**Scope:**
- DELETE `internal/integration/gemini.go` + `gemini_test.go`
- ADD `internal/integration/antigravity.go`
- ADD `internal/integration/antigravity_test.go` — idempotency + preserve
  existing keys test (mirror ex-`gemini_test.go` pattern)
- `internal/integration/registry.go:60` — swap `NewGeminiAdapter()` →
  `NewAntigravityAdapter()`
- `internal/integration/conformance_test.go` — update any gemini references
- `README.md` — update host matrix; add note that `.agents/` is VCS-tracked
- `AGENTS.md` — update any gemini references

**No deprecation shim needed** — Gemini CLI is dead; no users remain on it.

---

## Stage 2 — Role System Redesign

### 2.1 Audit Current Role Surface

**Current state:**
- `internal/spec/role.go` — `ReadonlyRoles = ["investigator", "reviewer"]`
- `internal/mcp/prompts.go` — two role prompts: `role/builder`,
  `role/investigator`
- `reviewer` appears in `ReadonlyRoles` but has **no registered prompt** (dead ref)
- No tool-gate per role — readonly distinction is the only enforcement

**Gaps:**
- Role names leak implementation shape (`builder` = "I write code") instead
  of expressing behavioral contracts
- No depth distinction in investigation — a quick symbol lookup and a
  full subsystem trace are both forced through `investigator`
- Spec integrity verification is inline in `cmd/check.go` / `core/RunGates` —
  no dedicated agent role owns it; any session can call it ad-hoc
- No `reviewer`, `tester`, `documenter`, `architect` prompts despite agents
  needing them regularly

### 2.2 Define Role Contracts

**What:** Redesign roles with explicit contracts. Each role declares: R/W
permission, context budget tier, phase affinity, allowed tool subset, and
file scope policy.

**Proposed role set** (expand from 2 → 8):

| Role | R/W | Budget | Phase affinity | Invocation trigger |
|------|-----|--------|----------------|--------------------|
| `scout` | readonly | minimal | any | Fast single-target lookup: "where is X", "find function Y", "what file defines Z" |
| `researcher` | readonly | large | any | Deep multi-file traversal: "how does Z work", "trace this call chain", "map subsystem" |
| `reviewer` | readonly | medium | any | Diff/PR review: "review this change", "audit this file for issues" |
| `architect` | readonly | medium | requirements / design | System design: "propose approach for X", "design component Y" |
| `builder` | read-write | focused | execute only | Implement exactly one task within its `files:` scope |
| `tester` | read-write | focused | execute / verifying | Write or run tests: "write tests for X", "verify task N passes" |
| `documenter` | read-write | focused | any | Write docs/changelog: "update README", "write entry for X" |
| `verifier` | read-write (specd state only) | medium | execute / verifying | Spec integrity: "check spec gates", "validate state.json", "run specd check" |

**Contract per role must declare:**

```
name:         stable kebab-case slug
rw:           readonly | readwrite
budget_tier:  minimal | focused | medium | large
phase:        [list of valid phases] | any
tools:        [allowed MCP tool subset]
file_policy:  "no writes" | "task scope only" | "specd state only" | "unrestricted within files:"
prompt_class: "gate-only" | "card" | "contract"
```

**Prompt strategy — two classes, not one:**

Role prompts and phase prompts solve different problems. Phase prompts are
verbose because they describe a multi-step workflow with conditional gates —
the agent needs that text to know what to do next. Roles are point-in-time
constraints — behavior enforced by *which tools are present*, not by text.

Injecting a long role prompt on every role switch wastes context. Design:

**Class A — `gate-only` (readonly roles: scout, researcher, reviewer, architect):**
No prompt body needed. Tool-gate already enforces "no writes" by omitting
all mutating tools from `tools/list`. `prompts/get` for these roles returns
a single-line identity card (~15 tokens):
```
Role: scout. Locate and return file:line references. Never modify files.
```
This is enough for the agent to confirm identity. No workflow instruction
needed because there is no workflow — just find and report.

**Class B — `card` (write-scoped roles: builder, tester, documenter):**
Tool-gate enforces which files are writable in principle, but cannot express
"write only inside the task's declared `files:` scope" — that's a semantic
constraint, not a tool constraint. 2-3 sentence card:
```
Role: builder. Implement exactly one task within its declared files: scope.
Run the task's verify command. Report evidence or blocker — never guess.
```
~30 tokens. No phase workflow text. No preamble.

**Class C — `contract` (verifier only):**
Verifier writes specd state only, never source. That distinction cannot be
expressed by tool-gate alone (both categories are "files on disk"). Needs
an explicit boundary statement. 3-4 sentences, ~45 tokens max:
```
Role: verifier. Run all gates for the active spec via spec_check. Report
each violation with gate, location, and remediation. You may repair specd
bookkeeping (stale locks, blocked task reset) — never modify source files
or spec artifacts.
```

**Token cost comparison:**

| Class | Tokens (approx) | Current investigator prompt | Saving |
|-------|-----------------|----------------------------|--------|
| gate-only | ~15 | ~70 | ~80% |
| card | ~30 | ~70 | ~57% |
| contract | ~45 | — | new |

**Rule:** if the tool-gate already prevents the behavior, don't repeat it
in text. Text only for constraints the gate cannot express.

**Scout vs Researcher distinction (key design decision):**
- `scout`: O(1) reads — returns file:line + one-line summary. Returns
  immediately. Context budget minimal; appropriate for tight agent loops.
- `researcher`: multi-file traversal, reads full files, traces call chains,
  understands package structure across boundaries. Large context budget.
  Used when scout answer is "it's complicated."

**Verifier role — elevated from inline check:**
Currently `specd check <slug>` (in `cmd/check.go`) runs `core.RunGates()` as
a CLI command. No agent role owns this responsibility — any session calls it
ad-hoc.

Elevating to dedicated `verifier` role gives the agent clear boundaries:
- **Can read:** all spec artifacts (`requirements.md`, `design.md`,
  `tasks.md`, `state.json`), `.specd/` directory, MCP registration files
- **Can write:** only specd state metadata (e.g. repair a stale `.lock`
  file, reset a blocked task) — never source files
- **Tools exposed:** `spec_check` (wraps `RunGates`), `spec_status`,
  `spec_state_read`, `spec_doctor` (wraps `RunDoctor`)
- **Prompt contract:** "Run all gates for the active spec. Report each
  violation with: gate name, location, message, and remediation step.
  Validate state.json task statuses against tasks.md checkboxes. Check
  MCP server registration health. Never modify source files or spec
  artifacts — only repair specd bookkeeping."

### 2.3 Implementation Path

**Step 1 — `internal/spec/role.go`:**
Replace flat `ReadonlyRoles []string` with a `RoleDef` struct carrying all
contract fields. Export `Roles []RoleDef` as the single registry. Keep
`IsReadonlyRole()` for backward compat — derive from `RoleDef.RW`.

**Step 2 — `internal/mcp/prompts.go`:**
Add 6 new `rolePrompt` entries: `role/scout`, `role/researcher`,
`role/reviewer`, `role/architect`, `role/tester`, `role/documenter`,
`role/verifier`. Each gets an embedded constant body — same pattern as
`builderPrompt`/`investigatorPrompt`. Existing `role/builder` and
`role/investigator` stay for backward compat until a deprecation cycle.

**Step 3 — `internal/mcp/tools.go`:**
Add tool-gate by active role. Extend the phase-tool filter logic so exposed
tools = phase-allowed ∩ role-allowed. `verifier` gets its own tool subset
(`spec_check`, `spec_status`, `spec_state_read`, `spec_doctor`) not exposed
to other roles by default.

**Step 4 — `internal/context/manifest_types.go`:**
Add `role` field to context manifest so the manifest filter (C1) can scope
context delivery by role as well as phase.

### 2.4 Wire Role Into Phase Watcher

Phase watcher currently pushes tool subsets keyed on phase alone.
After role redesign it must key on phase × active role, pushing the
intersection of phase-allowed tools ∩ role-allowed tools.

**File:** `internal/mcp/watcher.go` — extend `buildPhaseTools()` to accept
current role; update `phaseWatcher` to track active role alongside status.

---

## Stage 3 — Multi-Session Concurrency

### 3.1 Understand the Problem

**Scenario:** User installs specd on project. Opens two Claude Code sessions:
- Session A works on spec `feature-auth`
- Session B works on spec `feature-payments`

Each session spawns its own `specd mcp --root /project` process. Both MCP
processes share the same `.specd/` directory.

**What already works:**
- `internal/core/lock.go` — cross-process advisory lock per spec slug
  (`O_EXCL` file lock). Session A writing `feature-auth/state.json` and
  Session B writing `feature-payments/state.json` are already serialized
  independently. State writes cannot corrupt each other.

**What breaks:**
- `internal/mcp/watcher.go:activeSpec()` picks ONE "most active" spec across
  ALL specs. It uses `statusRank` tie-broken by slug order. If both specs are
  in `executing` phase, `activeSpec()` returns whichever slug sorts first
  alphabetically — the other session gets the wrong spec's phase context.
- Phase watcher pushes tool list updates keyed on that wrong spec's status.
  Session B might receive tool list built from Session A's spec phase.
- `specd mcp --root` has no session identity — no way to pin which spec a
  given MCP process is serving.

### 3.2 Add Session-Spec Affinity

**What:** Allow (and optionally require) each `specd mcp` process to declare
which spec slug it is serving.

**Implementation:**
- Add `--spec <slug>` optional flag to `specd mcp` sub-command
  (`internal/cli/args.go`)
- Pass slug down to `phaseWatcher` and `activeSpec()` via config or context
- When `--spec` is set: `activeSpec()` returns only that slug's state
  (ignores all others)
- When `--spec` is absent: keep current `statusRank` behavior (backward compat)
- `internal/mcp/server.go` — thread the slug through `startPhaseWatcher()`

**Files to change:**
- `internal/cli/args.go` — `--spec` flag on mcp subcommand
- `internal/mcp/watcher.go` — `phaseWatcher.slug` field; filter in
  `activeSpec()` when slug is set
- `internal/mcp/server.go` — pass slug from config/args into watcher setup
- Tests: multi-session golden test with two overlapping watcher instances
  driven by `FakeClock`

### 3.3 Session Isolation for Same-Spec Concurrent Access

**Scenario:** Two sessions both happen to work on the same spec slug (user
opened two windows on the same feature). Goal: second session should detect
the lock, block until first releases, and warn the user — not silently corrupt.

**What already works:** `WithSpecLock` in `lock.go` already blocks and
returns `GateError` on timeout with a human-readable message. This is correct.

**Gap:** MCP tools that call `WithSpecLock` propagate the error as a tool
result. Agents receive the error text but may not surface it clearly. Add
a structured `"locked"` status field to tool results so hosts can present
a blocking-session warning instead of raw error text.

### 3.4 Different Verifiers Per Session

**Scenario:** Session A spec uses `go test ./...` as verify command.
Session B spec uses `npm test`. Both run verify concurrently. Workers
(`internal/worker/`) spawn subprocesses.

**What already works:** Workers are stateless per invocation; each verify
call gets its own subprocess. Concurrent runs are independent OS processes
and do not share in-process state.

**Gap (minor):** `SPECD_WATCH_INTERVAL_MS` is global env — both watchers
poll on the same interval. Not a correctness issue, just a resource note.
No action required unless profiling shows contention.

---

## Stage 4 — Production Readiness

### 4.1 Observability

- Structured log events for: session start, spec lock acquired/released,
  phase transition, tool call with duration, install result
- `internal/obs/log.go` — currently minimal; add fields for slug, session-id,
  phase, role
- Export format: JSON lines (`--log-format json`) gated behind env flag

### 4.2 Error Budget and Graceful Degradation

- MCP server must respond to all JSON-RPC requests even when the backing
  `.specd/` directory is missing or corrupt — return structured error, not
  panic or silent drop
- `internal/mcp/server.go` — audit for unhandled nil-deref paths when no
  spec root found

### 4.3 Install Idempotency Regression Guard

- Add a conformance test that runs `specd init` twice on each adapter and
  asserts `Changed=false` on second run — currently only Gemini and Claude
  adapters have this (see `gemini_test.go:TestGeminiProjectAdapterPreservesSettingsAndIsIdempotent`)
- Extend `conformance_test.go` to enforce this for every adapter in the
  registry automatically

### 4.4 Context Manifest Spec-Scoping

- `internal/mcp/watcher.go:activeSpec()` reads ONE spec at a time. The
  context manifest filter (`C1` in server comments) is also spec-global.
- After Stage 3.2 (session-spec affinity), context manifest should be
  filtered to the pinned spec only — reduces noise delivered to agent context

### 4.5 Release Artifact Completeness

- `.goreleaser.yml` — verify checksums, sbom, and provenance attestation
  flags are set for production release
- `SECURITY.md` — already exists; ensure vulnerability disclosure contact
  is current
- Add `specd version --json` output for machine-readable version info in
  CI pipelines

---

## Notes

- Stages 1 and 2 are independent and can be developed in parallel.
- Stage 3 depends on Stage 2 being stable (role context used in watcher).
- Stage 4 items are independent of each other and can be parallelized.
- Add tests before code in each item where golden tests already exist —
  follow the pattern in `internal/testharness/`.
