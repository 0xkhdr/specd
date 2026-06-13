# specd — Gap Analysis & Roadmap

Where the implementation does not yet pay off the stated philosophy, and concrete plans to
close each gap. Read alongside [`specd-core-philosophy.md`](specd-core-philosophy.md) (the *why*),
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) (the *how*), and [`CLAUDE.md`](CLAUDE.md) (file map).

The harness is small (~2300 LOC), zero-dep, well-tested, philosophy-clean. These gaps are not
bugs — they are places where the promise ("agent reasons, harness enforces") is documented more
strongly than it is enforced in code.

| ID | Gap | Status | Priority |
|----|-----|--------|----------|
| G3 | `context` is a static phase→files map, ignores live state | ✅ **done** | — |
| G1 | Evidence is unverified free text | ✅ **done** | — |
| G2 | `delegate` mode emits no dispatch packet | ✅ **done** | — |
| G4 | No multi-spec / program view | ✅ **done** | — |
| G5 | Traceability warn-only; VERIFY has no per-criterion proof | ✅ **done** | — |

---

## G1 — Evidence is unverified text ✅ DONE

Shipped. `specd verify <slug> <id>` (`src/commands/verify.ts`) `spawnSync`s the task's `verify:`
line under the per-spec lock and writes a structured `VerificationRecord`
(`{ command, exitCode, verified, timedOut, stdoutTail, stderrTail, durationMs, ranAt, gitHead }`)
into `state.json` (schema v3 + migrate branch). `specd task <id> --status complete` now requires a
matching exit-0 record whose `command` equals the task's current `verify:` line — a changed verify
line is rejected as stale. The manual escape hatch is `--unverified --evidence "..."` for read-only
roles (`verify: N/A`). `check.ts` gate 5 upgraded: a complete **build-role** task with no verified
record is a violation (read-only roles exempt). Timeout via `SPECD_VERIFY_TIMEOUT_MS` (default 600s),
recorded as `timedOut` + exit 124. Tests: `test/verify.test.ts` (new, 5 cases) + gate updates across
`task`/`check`/`context`/`concurrency`/`hardening`/`full-spec`. 75 tests pass.

<details><summary>Original plan</summary>

### Problem
Philosophy: *"the agent cannot mark its own homework; trust is recorded, not assumed."*
Reality: `check.ts` gate 5 and `task.ts` only check that `--evidence` is **non-empty**. The string
`"npm test PASS"` is accepted on faith. specd never runs the `verify:` command, never confirms the
commit SHA exists, never sees an exit code. A drifting or adversarial agent fabricates evidence and
passes the gate. This is the single biggest gap between the stated philosophy and the code.

### Approach
Add a `specd verify <slug> <id>` command that *runs* the task's `verify:` line deterministically
(specd executes a shell command, makes zero judgments), captures `{ command, exitCode, stdout/stderr
tail, durationMs, ranAt, gitHead }`, and writes a structured **evidence record** into `state.json`.
Then make `specd task <id> --status complete` require a matching verified record rather than a free
string.

- **Schema:** bump `SCHEMA_VERSION` to 3; add `TaskState.verification?: VerificationRecord` (add a
  `migrate()` branch — default `undefined`). Files: `src/core/state.ts`.
- **Execution:** `src/commands/verify.ts` (new) — `child_process.spawnSync` the `verify:` line under
  the per-spec lock; truncate captured output; record exit code + `git rev-parse HEAD` if a repo.
  Register in `src/cli.ts` dispatch + USAGE.
- **Gate:** `task.ts` — `--status complete` requires either (a) a `verification` record with exit 0
  whose `command` matches the task's current `verify:`, or (b) explicit `--evidence` **plus**
  `--unverified` flag for manual/read-only proofs, recorded as `verified: false`. `check.ts` gate 5
  upgraded: complete tasks with a builder role and no verified record → violation (read-only roles
  exempt, as today).
- **Determinism preserved:** specd still makes no judgment — it runs the command the *task author*
  wrote and records the OS exit code. Zero LLM. Zero new runtime deps (`child_process` is stdlib).

### Risks / decisions
- Running arbitrary shell from a task is a capability bump — gate behind nothing new (the agent
  already runs shell), but document it. `verify:` lines are human-approved at the tasks gate.
- Long/hanging verify commands — add a timeout (`SPECD_VERIFY_TIMEOUT_MS`, default 600s) and record
  a timeout as a non-zero result.
- Keep `--evidence` working for the read-only roles (investigator/reviewer) that legitimately have
  `verify: N/A`.

### Files
`src/commands/verify.ts` (new) · `src/core/state.ts` (schema v3 + migrate) ·
`src/commands/task.ts` (gate) · `src/commands/check.ts` (gate 5) · `src/cli.ts` (register) ·
`test/verify.test.ts` (new) · `test/task.test.ts`, `test/check.test.ts` (gate updates).

### Verification
`npm run build && npm test`; new tests: verify-pass writes exit-0 record and unlocks complete;
verify-fail (exit≠0) blocks complete; stale record (verify line changed since) rejected;
read-only role still completes with `--evidence --unverified`.

</details>

---

## G2 — `delegate` mode is hollow for orchestrators ✅ DONE

Shipped. `specd dispatch <slug> [--json]` (`src/commands/dispatch.ts`) emits a ready-to-run packet per
runnable-frontier task — `{ id, wave, role, rolePrompt, title, why, contract, files, acceptance,
verify, depends, requirements, completion }` — with `rolePrompt` resolved from the actual
`.specd/roles/<role>.md` body (distinct roles loaded once each, via new `readRole()` in
`src/core/specFiles.ts`) and `completion` the exact `specd task … --status complete` command. Reuses
`runnableFrontier()` + `findTask()`; an empty frontier is classified (`all-complete` / `all-blocked` /
`waiting`) like `next`. Honors the `awaiting-approval` gate (`--force` overrides). Text mode prints a
compact summary; the full payload lives in `--json`. `next --all` is left untouched. Registered in
`src/cli.ts` dispatch + USAGE. Tests: `test/dispatch.test.ts` (new, 4 cases). 79 tests pass.

<details><summary>Original plan</summary>

### Problem
`config.json.roles.subagentMode: "delegate"` is documented as the parallel-dispatch path
([agent-integration.md §inline-vs-delegated]), but specd emits no machine-consumable **dispatch
packet**. An orchestrator fanning out builders must hand-assemble role-prompt + contract + files +
verify from `next --all --json`. This is the highest-leverage missing feature for *agentic* tools
specifically — the whole "fan the frontier out to parallel subagents" story has no payload.

### Approach
Make `next --all --json` (and/or a new `specd dispatch <slug> [--json]`) emit ready-to-run task
packets — each packet is everything a fresh subagent needs with zero assembly:

```jsonc
{
  "kind": "frontier", "count": 2,
  "packets": [
    {
      "id": "T2", "wave": 2, "role": "builder",
      "rolePrompt": "<full body of .specd/roles/builder.md>",
      "contract": "...", "files": "...", "acceptance": "...",
      "verify": "npm test", "requirements": [1],
      "completion": "specd task my-feat T2 --status complete --evidence \"<proof>\""
    }
  ]
}
```

- Reuse `runnableFrontier()` (`src/core/dag.ts`) + `findTask()` (`src/core/tasksParser.ts`) — both
  already power `next --all`.
- Load the role body once per distinct role from `.specd/roles/<role>.md` via `templatesDir`-style
  read (here a repo-local read, roles live in user repo).
- Text mode unchanged (human frontier list); the packet lives in `--json` only.

### Files
`src/commands/next.ts` (extend `--all --json`) **or** `src/commands/dispatch.ts` (new) + `cli.ts` ·
`src/core/specFiles.ts` (small `readRole(root, role)` helper) · `test/next.test.ts` or
`test/dispatch.test.ts` (new).

### Verification
`next --all --json` packet contains the resolved role body + contract + completion command for each
runnable task; empty frontier still classified (all-complete / all-blocked / waiting) as today.

</details>

---

## G3 — Curated `specd context` ✅ DONE

Folded live state into the phase briefing: blockers (every phase), latest midreq (when gated,
with impact + verbatim input), uncovered requirements (at VERIFY). SIGNALS section omitted when
nothing live. JSON gains `signals: { blockers, latestMidreq, uncoveredRequirements }`.
Shipped in `src/core/render.ts` + `src/commands/context.ts`; `test/context.test.ts` added
(command previously untested). 69 tests pass.

---

## G4 — No multi-spec / program view ✅ DONE

Shipped. Cross-spec edges live in a central `.specd/program.json` manifest
(`{ version, dependsOn: { slug: [deps] } }`) — the open question resolved in favor of one manifest
over a per-spec field, so the program is reasoned about in one place and each spec stays
self-contained. `src/core/program.ts` (new) projects every spec as a `DagTask` (id=slug; spec status
→ task status: `complete`→complete / `blocked`→blocked / else pending; waves derived as longest
dependency-chain length) and reuses the `dag.ts` primitives unchanged — `runnableFrontier`,
`nextRunnable`, `groupWaves`, `criticalPath`, `detectCycle`. A spec is runnable when all the specs it
depends on are `complete`. `specd program [status] [--json]` renders the spec-level DAG + the
program-wide runnable frontier; `specd program link <spec> --on <dep>` / `unlink` edit the manifest
(atomic write, de-duped, sorted). `link` validates both specs exist (exit 3), rejects self-deps
(exit 2), and refuses any edge that would create a cross-spec cycle (exit 1). The view flags edges to
deleted specs as orphans (warn, exit 0) and a cycle as a hard error (exit 1). Registered in
`src/cli.ts` dispatch + USAGE. No `state.json` schema bump needed — edges live entirely in the new
manifest. Tests: `test/program.test.ts` (new, 7 cases). 86 tests pass.

<details><summary>Original plan</summary>

### Problem
`specd status` (no slug) lists a board, but there are no **cross-spec dependencies** and no
program-level orchestration. Real agentic workflows run several specs; specd has no story for
ordering or blocking across them. An orchestrator can't ask "what's runnable across the whole
program right now."

### Approach (sketch — needs its own design pass)
- Optional `dependsOnSpecs: string[]` in a spec's `state.json` (schema bump) or a top-level
  `.specd/program.json` declaring inter-spec edges.
- `specd status --program` (or a `program` command) renders a spec-level DAG, reusing the existing
  `dag.ts` primitives (`groupWaves`, `criticalPath`, `nextRunnable`) at spec granularity — a spec is
  "complete" when its status is `complete`, "runnable" when all its spec-deps are complete.
- A program-level runnable frontier: which specs may start now.

### Files (anticipated)
`src/core/program.ts` (new, wraps `dag.ts`) · `src/commands/status.ts` or new `program.ts` ·
`src/core/state.ts` (deps field) · `test/program.test.ts`.

### Open questions
Where do cross-spec edges live — per-spec field vs central manifest? Central manifest is simpler to
reason about and keeps each spec self-contained; lean that way. Defer until G1/G2 land.

</details>

---

## G5 — Traceability warn-only; VERIFY has no per-criterion proof ✅ DONE

Shipped. Two config knobs under a new `gates` block in `config.json`
(`src/core/specFiles.ts`, `loadConfig` now deep-merges nested objects so a partial `gates` keeps the
other defaults): `gates.traceability: "warn" | "error"` (default `warn`) and
`gates.acceptance: "off" | "required"` (default `off`) — both preserve legacy behavior.

- **Configurable traceability** (`check.ts` gate 7): forward-traceability (a requirement covered by
  no task) routes to violations when `traceability: error`, else stays a warning. Backward-traceability
  remains a hard violation.
- **Per-criterion VERIFY proof** (`verify.ts`): new mode `specd verify <slug> --criterion <r>.<n>
  --status pass|fail --evidence "..."` records a `CriterionRecord` into the spec-level
  `state.acceptance` ledger (schema v4 + migrate branch), keyed by `<requirement>.<criterion>`.
  The requirement must be defined in requirements.md (else exit 1); bad key format / missing evidence
  are usage errors (exit 2); a recorded `fail` exits 1.
- **Approve refusal** (`approve.ts`): when `acceptance: required`, `approve` of `verifying → complete`
  refuses while any defined requirement lacks a passing criterion or any criterion is a `fail`
  (`acceptanceGaps()` in `render.ts` is the shared evaluator). With `acceptance: off` the gate is
  inert and approve advances as before.
- **Reporting** (`report.ts`): a 🧪 Acceptance Criteria section renders the pass/fail ledger when
  non-empty (md + html).

Tests: `test/g5.test.ts` (new, 5 cases) + a report-section case in `test/report.test.ts`. 92 tests pass.

<details><summary>Original plan</summary>

### Problem
Gate 7 (traceability) only **warns** when a requirement has no covering task. At the spec-level
VERIFY gate, nothing tracks **which acceptance criteria actually passed** — `approve` is a single
human yes/no with no structured per-requirement evidence. G3 now *surfaces* uncovered requirements
in `context`, but nothing *enforces* coverage or records per-criterion outcomes.

### Approach (sketch)
- Promote forward-traceability from warn to a configurable gate: `config.json` →
  `gates.traceability: "warn" | "error"` (default `warn` to preserve current behavior).
- At VERIFY, optionally require a per-requirement acceptance map: `specd verify <slug> --criterion
  <req>.<n> --status pass|fail --evidence ...`, persisted into `state.json`, surfaced by `report`.
  Pairs naturally with G1's evidence records.
- `approve` of `verifying → complete` refuses while any criterion is unmet (when the map is enabled).

### Files (anticipated)
`src/commands/check.ts` (configurable gate 7) · `src/commands/verify.ts` (criterion map, shares G1) ·
`src/commands/approve.ts` (VERIFY refusal) · `src/core/report.ts` (render coverage) ·
`src/core/specFiles.ts` (Config) · tests.

### Dependency
Builds on G1 (shared evidence-record machinery). Do G1 first.

</details>

---

## Recommended sequence

1. ~~**G1** — makes the core promise true; unblocks G5.~~ ✅ shipped.
2. ~~**G2** — makes specd genuinely useful to agentic orchestrators.~~ ✅ shipped.
3. ~~**G5** — per-criterion proof, riding on G1's machinery.~~ ✅ shipped.
4. ~~**G4** — program-level orchestration; largest design surface, do last.~~ ✅ shipped.

All gaps (G1–G5) shipped. 🎉
