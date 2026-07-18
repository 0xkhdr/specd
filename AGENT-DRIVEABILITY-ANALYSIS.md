# specd Agent-Driveability Analysis & Remediation Plan

Analysis of the failures recorded in `SPECD-FIELD-NOTES.md` and
`AGENTS_specd_investigation_plan.md`, traced to their causes in the specd
codebase, with concrete recommendations so a coding agent (Claude Code, codex,
or any harness) can drive specd accurately in both **base** and **orchestrated**
mode.

Every claim below is anchored to source. File references are to this repository
at `2ccd2a6`.

---

## 1. Verified root causes, issue by issue

### 1.1 `QUALITY_DECLARATION_INVALID` surfaces only at `complete-task` (field notes §3.1)

**Confirmed.** The `evidence` column of tasks.md is parsed by
`core.ParseQualityContract` (`internal/core/quality_contract.go:20`), which
requires `class/check-id` and accepts exactly four classes:
`test`, `output_eval`, `trajectory_eval`, `review`
(`internal/core/quality_contract.go:30`).

The **only** production caller is the complete-task handler
(`internal/cmd/lifecycle.go:249`). No gate in the check registry parses the
`evidence` column:

- The quality gate (`internal/core/gates/quality.go`) validates
  `ctx.QualityPolicies` — policy *files*, not the tasks.md `evidence` column —
  and is opt-in.
- `specd check`, tasks-phase approval, and `specd verify` never call
  `ParseQualityContract`.

So a malformed cell (`evidence: tests`) survives authoring, `check`, approval,
and `verify`, and detonates only at the last step. The errors
(`QUALITY_DECLARATION_INVALID`, `QUALITY_CLASS_UNKNOWN`) also never list the
valid class enum, so the agent must reverse-engineer it from binary strings.

### 1.2 `verify` can never satisfy a declared quality check (field notes §3.2)

**Confirmed, and worse than the field notes concluded.** Two disjoint evidence
stores exist:

| Store | Record type | Writers | Read by |
|---|---|---|---|
| `evidence.jsonl` | `EvidenceRecord` (`internal/core/evidence.go:41`) — **no `evidence_class` field** | `specd verify` (`internal/cmd/registry.go:993`), `complete-task` telemetry annotation | The no-bypass verify gate (`verifyEvidenceReady`, `internal/core/task_complete.go:22`) |
| eval store (`EvalStorePath`) | `EvidenceEnvelopeV1` (`internal/core/eval.go:29`) — carries `evidence_class`/`check_id`/`verdict` | **only `specd eval import`** (`internal/cmd/eval.go:43`) | The quality contract gate (`CompleteTaskWithQuality`, `internal/core/task_complete.go:50` → `EvaluateQuality`) |

The field notes guessed that `verify --criterion` or `brain report` produce
class-tagged evidence. **Neither does:**

- `verify --criterion` (`internal/cmd/registry.go:1263`) writes a
  `CriterionRecord` to `criteria.jsonl` — a third ledger, consumed by the
  *criteria* gate at the completion-approval transition
  (`internal/core/gates/approval.go:75`), not by the per-task quality contract.
- `brain report` records worker completion with a `VerifyRef` pointing back at
  `evidence.jsonl` (`internal/cmd/brain_worker.go`) — same class-less store.

The only path that satisfies a declared `class/check-id` is
`specd eval import` of an `EvidenceEnvelopeV1` JSONL, and that envelope demands
`producer`, `producer_version`, `config_digest`, `subject_revision`,
`artifact_ref`, `artifact_digest`, etc. — it is designed for an external eval
harness, not for hand authoring.

**Net effect:** any tasks.md that declares an `evidence` cell makes the
documented `verify → complete-task` loop structurally uncompletable unless an
eval-import pipeline exists. Nothing in help, errors, or the verify success
message says so; `verify` even prints
`"…run \`specd complete-task <slug> <task>\`"` (`internal/cmd/registry.go:1016`),
actively steering the agent into the wall.

### 1.3 Brain wait message conflates two states (field notes §3.3)

**Confirmed.** `internal/orchestration/decide.go:57`:

```go
return Decision{Action: ActionWait, Reason: "no dispatch authority or no frontier"}
```

One string for two unrelated conditions. Dispatch authority is simply the
`--authority` flag on `brain run`/`brain start`
(`internal/cmd/brain_run.go:148`):

```go
authority := orchestration.Authority{Enabled: flagEnabled(flags, "authority")}
```

It *is* documented (`docs/command-reference.md:749`,
`specd brain start payments --authority`), but the error never points there,
and the help system can't be asked (see 1.4). The chicken-and-egg the notes hit
(`claim` needs a mission-id, missions only exist after authorized dispatch) is
real and by design — the missing piece was purely discoverability of
`--authority`.

### 1.4 `brain --help` / `agents inspect` fail with "unknown operation" (field notes §3.6)

**Confirmed.** `core.ResolveOperation` (`internal/core/commands.go:809`)
resolves multi-op verbs by their first positional:

- `brain` with no subcommand (which is what `brain --help` parses to) returns
  `Operation{}, false` (`internal/core/commands.go:825-828`), so dispatch fails
  closed with `unknown operation for command "brain"`
  (`internal/cmd/dispatch.go:39`). `--help` is never intercepted for
  multi-op commands.
- `agents` accepts only `""`, `doctor`, `guide` as subcommands
  (`internal/core/commands.go:816-824`). The MCP/palette id is
  `agents.inspect`, but the CLI spelling is bare `specd agents` — typing
  `specd agents inspect` (the obvious transliteration of the tool id) fails
  closed. Tool-contract name and CLI surface disagree.

Rich per-operation metadata (usage, flags, examples) already exists in the
palette (`internal/core/commands.go:515` declares `brain.*` operations) — it's
just unreachable through the natural `--help` spelling.

### 1.5 Coverage gate reads only the `refs` column (field notes §3.5)

**Confirmed.** `core.AnalyzeCoverage` (`internal/core/coverage.go:22`) builds
its task-side map exclusively from `task.Refs`, which the parser fills from a
column literally headed `refs` (`internal/core/tasksparser.go:78`):

```go
Refs: splitTaskList(cell(cells, headerIndex(header, "refs"))),
```

Criterion IDs listed in the `acceptance` column are **ignored** for coverage.
A tasks.md whose criterion linkage lives in `acceptance` (as the home-page spec
did) is therefore 0%-covered in the gate's eyes, and the refusal message
(`internal/core/gates/approval.go:107`) never names the column it reads.

Timing makes it worse: `coverageGate` only arms when
`ApproveTarget == executing` (`internal/core/gates/approval.go:89`), so the
defect is invisible during authoring and the entire `tasks` phase — exactly the
"latent blocker" pattern the field notes describe (three of them accumulated:
bad evidence cells, unsatisfiable quality checks, empty `refs`).

### 1.6 Pinky workers scoped to a harness that may not be wired (field notes §3.4)

**Partially environmental, partially design.** `specd init` scaffolds workers
for **both** harnesses — `.claude/agents/pinky-*.md` and
`.codex/agents/pinky-*.toml` (`internal/core/scaffold.go:93-107`) — and
`specd agents` doctor checks the codex-side files
(`internal/core/agents.go:102-133`). The field-notes project was missing them,
so either init predated worker scaffolding or the files were deleted;
`specd init --repair` regenerates them. Two genuine product gaps remain:

- The handshake reports a single `agent` value and nothing verifies at
  runtime that the *executing* harness matches the harness the Brain would
  dispatch into. `specd doctor` does not flag "orchestrated spec + no worker
  definitions for the active harness".
- Nothing in the brain `wait` path says "no registered worker" as a distinct
  condition.

(Note: in a correctly scaffolded Claude Code project, the pinky agents *are*
registered subagents — this repository's own harness lists
`pinky-craftsman/validator/scout/auditor`. The failure mode is stale/partial
scaffolding, not a fundamental absence.)

### 1.7 MCP positional/flag mismatch (field notes §3.7)

**Confirmed as designed but trippy.** The MCP layer reserves an `args` array
for ordered positionals and maps every other JSON key to a `--flag`
(`internal/mcp/server.go:178-194`). Passing `"--guide"` inside `args` makes it
a positional, which `status` rejects. The tool schema documents this
(`internal/mcp/tools_core.go:46-86`), but a literal `--flag` string inside
`args` is silently forwarded instead of being caught and corrected — a
one-line normalization would remove the entire failure class.

### 1.8 The investigation plan itself contains drift

`AGENTS_specd_investigation_plan.md` cites paths and role names that don't
match the code: gates live under `internal/core/gates/` (not
`internal/core/ears.go`), evidence checking is not in `internal/cmd/check.go`,
and roles are `scout/craftsman/validator/auditor` (per CLAUDE.md and
`internal/core/roles.go`), not `investigator/builder/reviewer/verifier`. An
agent following that document literally would hallucinate structure. Any
steering doc handed to an agent must be generated from, or linted against, the
palette (`specd help --json`) rather than written from memory.

---

## 2. The unifying diagnosis

All seven issues are one disease: **specd validates late and explains
narrowly.** The harness is genuinely deterministic and fail-closed (good), but:

1. **Constraint knowledge lives only at the enforcement point.** The evidence
   class enum, the `refs`-column contract, and the eval-store requirement are
   each encoded in exactly one function that runs at the *last* step of the
   pipeline. An agent — which discovers rules by reading errors — pays a full
   implement→verify→complete round-trip per rule.
2. **Sibling commands look paired but aren't.** `verify`/`complete-task`,
   `verify --criterion`/quality contracts, `agents.inspect`/`specd agents`.
   Agents generalize from surface symmetry; every asymmetry needs either
   unification or an explicit signpost in the error/help text.
3. **Fail-closed without a next command.** Fail-closed is correct for an
   evidence harness, but each refusal must carry the exact command that
   unblocks it (as `EVIDENCE_STALE` already does: "re-run `specd verify …`" —
   that's the model to copy).

An agent-driven CLI is used by a reader that never skims a manual but always
reads the error it just received. **The error channel is the documentation
channel.**

---

## 3. Recommended specd changes (prioritized)

### P0 — validate declarations early, at `check` and tasks-phase approval

Add a `quality-declaration` gate to the registry
(`internal/core/gates/registry.go`) that runs `ParseQualityContract` over every
task row. Pure function of tasks.md → fits the determinism invariant. Also arm
`coverageGate` (advisory/warning severity) at `ApproveTarget == tasks`, not
only `executing`, so a spec cannot enter the tasks phase carrying gaps.
This single change eliminates the "three latent blockers" pattern.

Include the enum in the parse errors:

```
QUALITY_CLASS_UNKNOWN: "tests" — valid classes: test, output_eval, trajectory_eval, review; format class/check-id, e.g. test/db
```

### P0 — close (or signpost) the verify↔quality-contract gap

Preferred: when a task declares `test/<check-id>` and its verify command
passes, have `runVerify` also append an `EvidenceEnvelopeV1` to the eval store
(`evidence_class: test`, `check_id` from the declaration,
`subject_revision`/`GitHead` from the same head pin, `producer: specd-verify`).
Deterministic, exit-code-backed, HEAD-pinned — no invariant weakened; it is the
same fact recorded in the schema the completion gate reads. Non-`test` classes
(review, evals) legitimately still require `eval import`.

Minimum fallback: rewrite `EVIDENCE_MISSING` to state the mechanism:

```
EVIDENCE_MISSING: task T1 declares test/db, but `specd verify` records carry no
evidence class. Produce it with `specd eval import <slug> <file> --task T1 --check db`
(see docs/command-reference.md §eval), or remove the declaration if unintended.
```

And stop `verify`'s success line from unconditionally recommending
`complete-task` when a contract it cannot satisfy exists on the task.

### P1 — split the brain wait reason and name the unblock command

`internal/orchestration/decide.go:57` → two decisions:

- `wait: dispatch authority not granted — operator: re-run \`specd brain run <slug> --authority\``
- `wait: frontier empty — no eligible task (all blocked, leased, or done); \`specd status <slug> --guide\``

Add a third distinct reason when no worker definition exists for the active
harness.

### P1 — make `--help` reach the palette for multi-op verbs

In dispatch, intercept `--help` (and empty subcommand) for `brain`, `eval`,
`exception`, `agents`, and print the operations already declared in
`internal/core/commands.go` (usage, flags, examples). Accept `agents inspect`
as an alias so the CLI spelling matches the palette/MCP id. Extend `Flag`
metadata with enum/shape (`evidence` classes, `--status pass|fail`,
mission-id provenance) so `help --json` — the machine surface agents actually
consume — carries the value shapes, not just flag names.

### P1 — name the field every gate reads

Coverage refusal → `coverage: requirements without an implementing task (matched
against the tasks.md \`refs\` column): R1, R2 … Add criterion IDs (R1.1, …) to
\`refs\`, or mark the task \`kind: deferred\`.` Apply the same rule everywhere: a
gate that reads a specific column/file must say so in its refusal.

### P2 — doctor checks harness alignment

`specd doctor` should verify, for an orchestration-enabled config: worker
definitions on disk for the detected harness (`.claude/agents/pinky-*.md`
and/or `.codex/agents/pinky-*.toml`), handshake `agent` value consistent with
what is present, and emit `specd init --repair` as the remedy. This converts
field-notes §3.4 from a dead-end into a one-command fix.

### P2 — MCP flag normalization

In `splitArguments` (`internal/mcp/server.go:181`), treat an `args` element
with a `--` prefix as an error with a corrective message ("pass `guide: true`
as a property, not inside `args`") — or normalize it into the flags map. Either
removes the CLI/MCP asymmetry trap.

### P2 — verify-command lint

At tasks-phase check, flag verify commands relying on interactive job control
(`kill %1`, bare trailing `&` without `$!` capture) — deterministic string
lint, warning severity. This catches the self-killing-server hang (field notes
§5 agent rule 6) before a 120 s timeout does.

---

## 4. Best practices until the code changes land

### 4.1 Spec authoring rules (agent or human, before approving into `tasks`)

1. **`refs` column is the coverage contract.** Every requirement *and* every
   acceptance criterion ID must appear in some task's `refs` cell (or the
   requirement's task marked `kind: deferred`). `acceptance` does not count.
2. **`evidence` column: leave it empty unless you operate an eval pipeline.**
   If set, it must be `class/check-id` with class ∈
   `test | output_eval | trajectory_eval | review`, and you must plan to run
   `specd eval import` to produce the matching envelope — plain `verify` will
   never close the task. In single-actor/base mode, prefer no declaration and
   a strong `verify` command.
3. **Verify commands must be non-interactive and self-contained.** Pattern for
   server smoke tests:
   `go run . & P=$!; sleep 1; curl -sf http://localhost:8080/; S=$?; kill $P; exit $S`.
   Never `kill %1`.
4. **Dry-run the exit gates before approval:** run `specd check <slug> --json`
   *and* `specd approve <slug>` expecting refusal, and read every blocker. A
   refusal at this point costs one command; the same refusal after
   implementation costs the whole loop.

### 4.2 Agent operating protocol — base mode

Per turn, in order:

1. `specd handshake bootstrap <slug> --json` once per session; pin the palette
   digest.
2. `specd status <slug> --guide --json` — act only within the legal-commands
   list.
3. `specd context <slug> <task> --json` — the context manifest is the task
   contract (files, acceptance, role, budget). If context lists quality
   class/check IDs, resolve *how each will be produced* before writing code;
   if the answer is "verify", stop and fix the declaration or ask the operator.
4. Probe gates early: run `specd complete-task` immediately after claiming a
   task, expecting failure, to surface evidence/coverage demands up front.
5. `specd verify <slug> <task>` before any completion attempt; on failure,
   fix and re-verify — after repeated failures the escalation ratchet requires
   a human `--override`, which is a stop-and-report condition, not something to
   route around.
6. Never hand-edit `state.json`, `evidence.jsonl`, `runs.jsonl`, or checkbox
   markers. `tasks.md` metadata cells *are* editable craftsman work — record
   the deviation (`specd decision` / spike) when fixing authoring defects.
7. Blocked-means-stop: one retry, then report the exact refusal text and the
   ledger state. Do not fabricate authority, evidence, or envelope files.

### 4.3 Agent + operator protocol — orchestrated mode

Preflight (operator):

1. `specd doctor --json` and `specd agents --json`; if pinky definitions are
   missing for the active harness, `specd init --repair`.
2. Confirm harness match: handshake `agent` vs the harness actually running.
   Claude Code needs `.claude/agents/pinky-*.md` registered as subagents;
   codex needs `.codex/agents/pinky-*.toml` + config block.
3. Grant authority explicitly: `specd brain run <slug> --authority` (or
   `brain start … --authority`). Without it every step is `wait` — this flag is
   human-granted by design; an agent must request it, never simulate it.
4. If no eval pipeline exists, either strip `evidence` declarations from
   tasks.md or stand up the `eval import` producer *before* dispatch.

Loop (Brain is deterministic — no LLM in the decision path):

- Brain: `brain run <slug> --authority` → dispatches missions off the frontier.
- Worker (pinky subagent): `brain claim <slug> <mission-id> <worker-id> <role>`
  → edit only declared files → `specd verify` → `brain report` → release.
  Mission IDs come only from dispatched missions (`brain status <slug> --json`
  shows them); if none exist, the fix is authority/frontier, not `claim`
  retries.
- On `wait`: check the three causes in order — authority granted? frontier
  non-empty (`specd next <slug> --json`)? worker defined for this harness?
- Escalations, gate clears, and completion approval remain human. Cost brakes
  (`hostReportedCostLimitUSD`) are the operator's dial.

### 4.4 Mode selection rule of thumb

- **Base mode** when: single agent session, no eval-import pipeline, tasks
  ≤ ~10, human available to approve phases. Sanctioned close path:
  `verify → complete-task`, with *no* `evidence` declarations.
- **Orchestrated** when: parallel frontier waves matter, worker definitions are
  scaffolded for the running harness, an operator is present to grant
  `--authority` and clear escalations. Verify readiness with
  `specd mode <slug> --recommend --json` + `specd doctor --json` before
  switching.

---

## 5. Summary table

| Field-notes issue | Root cause (code) | Fix owner | Recommendation |
|---|---|---|---|
| §3.1 late `QUALITY_DECLARATION_INVALID` | `ParseQualityContract` called only from complete-task (`internal/cmd/lifecycle.go:249`) | specd | P0: parse in `check` + approval; enum in error |
| §3.2 `verify` never satisfies quality checks | quality gate reads eval store; only writer is `eval import` (`internal/cmd/eval.go:43`); `EvidenceRecord` has no class field | specd | P0: stamp `test/*` envelopes from passing verify, or signpost `eval import` in `EVIDENCE_MISSING` |
| §3.3 brain `wait` opaque | single conflated reason (`internal/orchestration/decide.go:57`); authority = undocumented-in-error `--authority` flag | specd + operator | P1: split reasons, name the command |
| §3.4 pinky workers absent | scaffolding stale/partial; doctor doesn't check harness alignment | operator + specd | `specd init --repair`; P2 doctor check |
| §3.5 coverage refusal | gate reads only `refs` column (`internal/core/coverage.go:22`, `internal/core/tasksparser.go:78`); armed only at approve→executing | spec author + specd | put criterion IDs in `refs`; P1 name the column, arm earlier |
| §3.6 broken `--help` | `ResolveOperation` fails closed on empty subcommand (`internal/core/commands.go:825`); `agents inspect` not a CLI spelling | specd | P1: route `--help` to palette; alias `agents inspect` |
| §3.7 MCP `--guide` in `args` | flags-as-properties contract (`internal/mcp/server.go:181`); literal `--x` positionals forwarded silently | specd + agent | P2 normalize/reject; agent: flags are JSON properties |

The through-line for every fix: **move each rule's first point of contact from
the completion gate to the authoring gate, and make every refusal name the
field it read and the command that unblocks it.** The harness already enforces
correctly; it needs to *teach* at the same points where it enforces.
