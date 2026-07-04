# PROJECT.md — The specd Project: Position, Philosophy & Operating Knowledge

> **Audience: any coding agent working on this repository.** This is the single
> authoritative context document for the `specd` rebuild. It consolidates the original
> fresh-start brief, the 12 domain analyses, the cross-cutting ADRs, the scope triage,
> the roadmap, and the current production audit — all of which it supersedes. Read this
> before touching anything.

---

## 1. What specd is

**The agent reasons. The harness enforces.**

`specd` is an **agent-agnostic, spec-driven coding harness CLI**. It moves process
integrity *off* the LLM's non-deterministic context window and *onto* a strict, local,
tool-gated pipeline written in dependency-free Go, shipped as a single static binary.
Any coding agent (Claude Code, Cursor, Codex, Aider, or any MCP client) can drive it.

It is a direct implementation of the central equation from *The New SDLC with Vibe
Coding* (`The_New_SDLC_With_Vibe_Coding.pdf`, the project's philosophical anchor):

```
Agent = Model + Harness
```

The paper argues the model is ~10% of what determines agent behavior; the **harness** —
instructions/rule files, tools, sandboxes, orchestration logic, guardrails/hooks,
observability, and context policy — is the other ~90%, and it is *the team's* surface
area, not the model provider's. specd **is** that harness. The paper's claim — *"Most
agent failures, examined honestly, are configuration failures"* (p.30) — is the
product thesis.

Key paper anchors used throughout the project:
- **Harness Engineering** (pp. 26–34) — the harness as the unit of engineering.
- **Context engineering: the real skill** (pp. 15–18) — six context types
  (instructions, knowledge, memory, tools, examples, guardrails); static vs dynamic
  context; progressive disclosure.
- **Conductor vs orchestrator** (pp. 31–34) — *conductor*: real-time human-in-the-loop
  IDE work; *orchestrator*: async delegation to background agents at a higher level of
  abstraction.
- **The 80% problem** (p. 34) — the agent handles the delegable 80%; the human reserves
  attention for the ambiguous 20% (edge cases, integration points, correctness). specd
  is the machine that makes the 80% *safely* delegable by making every state change
  evidence-gated and every report deterministic.
- **think → act → observe** loop (pp. 29–30) — the harness provides the execution
  environment, captures error output, routes it back.

### The seven harness components

Every shipped feature must map to exactly one of: **instructions / tools / sandboxes /
orchestration / guardrails / observability / context** — plus at least one principle
below. Unmapped code does not merge. (This is the "harness charter" rule; see
`docs/charter.md` once it exists — the charter maps every verb to a component.)

### The eight principles (the product, stated as invariants)

1. **The Foundational Split** — agent creates; harness enforces.
2. **Specs as the Source of Truth** — the plan lives as versioned Markdown on disk, not in context.
3. **Evidence Gates Every State Change** — verifiable proof required; no self-reported "done".
4. **Waves, Not Lines** — work is a DAG of concurrent batches, not a flat todo list.
5. **Agent-Agnostic by Design** — standardized interface via role-prompt injection.
6. **Human Gates at Phase Boundaries** — semantic transitions need explicit approval.
7. **Deterministic Reporting** — reports are projections of `state.json`, never LLM output.
8. **Steering as Constitution** — durable steering files outlive chat sessions.

### Core use cases (the value that must survive)

- **A single agent driving a disciplined lifecycle** (`simple` mode):
  `init → new → author requirements (EARS) → approve → design → approve → tasks (DAG)
  → approve → next → implement → verify → complete → approve`.
- **Parallel/async multi-agent execution** (`orchestrated` mode): frontier dispatch of
  ready-to-run packets; optional Brain/Pinky controller for autonomous coordination.
- **Host onboarding in one command**: agent detection + project-scoped MCP wiring.
- **Deterministic PR gating & reporting**: network-free summaries — computed, never generated.

The acceptance test for every retained feature: *does this feature enforce the plan, or
does it try to author it?* If it authors, it is agent work, not harness work (P1) — cut it.

---

## 2. Why this repo exists: the fresh-start rebuild

The previous implementation (v1, frozen read-only under `reference/`) proved the
philosophy but accreted far beyond it: **29 registered commands**, ~56k LOC across
`cmd`+`core`, with orchestration/program/ACP mass of **~350K of `internal/core` source
vs ~120K for the entire lifecycle+gates+parser core**. The philosophy was lean; the
implementation was not.

**North star of the rebuild:** rebuild specd to its *final-form* value on the **minimal
accurate path** — keep the core thesis, drop the accretion, ground every decision in the
paper and context-engineering foundations. **Subtractive first**: before adding better
architecture, decide what the minimal accurate tool is.

The rebuild pipeline was: **Stage 1** — 12 domain analyses + ADRs + triage + roadmap
(complete). **Stage 2** — translate each into `specs/<domain>/spec.md` + `tasks.md`
(complete; see `specs/`). **Stage 3** — implement in waves (largely complete; see §8
for the honest status). The analysis artifacts have served their purpose and were
removed; this file preserves everything durable from them.

### How to learn from `reference/`

`reference/` is a **museum, not a foundation**. Do not import from it, build it into the
new tree, or copy files wholesale. Read it to understand *what* v1 did and *why*.
`reference/specd` is the built v1 binary — run it in a throwaway dir to observe real
output/state shapes when you need a concrete contract. Any historical path like
`internal/core/state.go` cited in older discussion means `reference/internal/core/state.go`.

### Sources of truth (precedence, highest first)

1. The ADRs (§4 of this file) — binding cross-cutting decisions.
2. `specs/*/spec.md` — the authored per-domain requirements + design.
3. This file's roadmap/triage/domain sections.
4. The paper (`The_New_SDLC_With_Vibe_Coding.pdf`).
5. `reference/` — evidence of what v1 did; lowest authority.

If sources conflict, the higher wins; surface the conflict rather than silently resolving it.

---

## 3. Non-negotiable guardrails

- **Determinism first.** No LLM call may sit inside the harness's decision path. Gates,
  DAG computation, reports, estimates, and any Brain/controller decisions stay **pure
  functions of on-disk state**. No network in any render/decide path.
- **Evidence integrity is absolute.** No task completes without a passing verify record
  (exit code + git HEAD). Read-only roles (scout/validator/auditor) use the explicit
  `--unverified --evidence` escape hatch only. Host-reported telemetry (cost/tokens) is
  stored verbatim as evidence, **never trusted as proof**.
- **Hard invariants (ADR-8)** — preserved verbatim; any change requires a new recorded ADR:
  - **Atomic writes** — temp → fsync → chmod 0644 → rename; a partial write never replaces.
  - **CAS on `revision`** — inside `WithSpecLock`; test builds panic if SaveState runs unlocked.
  - **Reentrant per-spec advisory lock** — stale reclaim, goroutine-id reentrancy, timeout.
  - **`ParseTasks` byte round-trip** — `Serialize(Parse(x)) == x`, property/fuzz-tested;
    single-line rewrite on status change; stable line numbers.
  - **Embedded templates** — single `go:embed` in `embed.go`; no disk-relative reads.
  - **Zero runtime dependencies** — `go.mod` has no `require`; git-native default backend;
    single static binary.
  - **Evidence integrity** — as above; worker reports validated against records.
  - **Determinism** — as above.
- **Subtractive bias.** When unsure whether something is core, default to CUT/DEFER and
  record the reasoning. The target is the minimal accurate surface, not feature parity with v1.
- **Context discipline.** Practice what specd designs: lean context, targeted reads, cite
  sources; use read-only sub-agents for scout sweeps.
- **Fail-loud posture everywhere** — corrupt state, truncated YAML, malformed env: loud
  error, never silent coercion. It is a determinism and safety property.

---

## 4. The ADRs (binding cross-cutting decisions)

**ADR-0 — Preflight factual corrections.** The real v1 surface was 29 registered
commands. The parser entry point is **`ParseTasks`** (not `ParseTasksMd`). The context
manifest builder **`BuildContextManifest` lives in `internal/context`**, not core — core
exposes only the adapter `BuildMissionContextManifest`; folding it into core creates a
`core→context→core` import cycle. v1's Postgres/Redis backends (build tags) contradicted
the zero-dep value.

**ADR-1 — Parser & plan format.** KEEP agent-authored Markdown `tasks.md` as source of
truth with the hard byte-round-trip invariant; REDESIGN the annotation channel — machine
state (status, verify-ref, telemetry) lives in **`state.json`** (machine truth), leaving
`tasks.md` clean Markdown whose only load-bearing content is checkboxes + metadata keys.
Never switch to JSON/YAML task files (moves authorship away from agent/reviewers,
violating P1/P2). The Sync gate enforces checkbox↔state agreement.

**ADR-2 — Config.** KEEP the hand-rolled YAML-subset loader (zero-dep, deterministic,
**fail-loud** — parse error is a hard exit, never silent defaults). CUT legacy
`config.json` handling and the `migrate` command. Config file is **`config.yml`**,
layered global→project→env. `state.json` schema resets to `SchemaVersion: 1`;
reintroduce migration only when the v1 schema first evolves.

**ADR-3 — Orchestration surface.** Collapse v1's ~350K of brain/pinky/conductor/
orchestrate/program mass into a **single `internal/orchestration` package** with a
**pure `Decide(Snapshot) → Decision` core** and thin file-backed IO. Ship
`brain {start|step|run|status|approve|cancel|resume}` +
`pinky {claim|heartbeat|report|inbox|checkpoint}` only. CUT `orchestrate`; DEFER
`conductor` (analytics), the program (multi-spec) tier, and model-tier routing.
Compiled always, **inert unless `orchestration.enabled` — fail-closed**.

**ADR-4 — Gate engine.** Pluggable interface, not hardcoded branches:
`Gate{ Name(); Run(CheckCtx) []Finding }` + an ordered `Registry`; uniform
`off|warn|error` severity in one config block. Core 7 gates register unconditionally;
opt-in gates register when configured. Gate bodies are **pure — no IO**. Byte-identical
`check` output when opt-ins are off is preserved and testable. Adding a gate = one
registration call, zero edits to `check.go`.

**ADR-5 — Flywheel tiering.** v1 ships **no flywheel commands**; only the `security`
**gate module** ships (via ADR-4). All deferred features re-enter through exactly two
seams: the `Gate` interface and the `state.records` extension map. Evidence shapes
(`DeployApproval`, `EvalSummary`, inventory waivers) are documented, not coded. CUT
`submit`, `migrate`. No flywheel feature may add a core schema field or a `check` branch.

**ADR-6 — State schema.** Core `State` holds only lifecycle fields; optional/plugin
evidence lives in `State.Records map[string]json.RawMessage`. Core validates only that
entries are valid JSON; each plugin owns its key's schema.

**ADR-7 — Execution mode.** `mode` is a first-class enum with exactly two states aligned
to the paper: **`simple`** (conductor: human-in-the-loop, no worker delegation) and
**`orchestrated`** (orchestrator: async delegation). Set at `new --mode`, default
`simple`, changeable only via an auditable `approve --mode` transition. Orchestration
eligibility keys off `mode: orchestrated`. v1's analytics-flavored `Conductor` mode is
dropped (deferred with the `conductor` command).

**ADR-8 — Hard invariants.** See §3.

**ADR-9 — Domain completeness.** The 12 domains cover the retained surface. State
backends fold into domain 10 (git-only; Postgres/Redis CUT to optional build tags);
the program tier folds into domain 09 (DEFER wholesale).

**ADR-10 — Scaffold surface.** `init` scaffolds `.specd/roles/{scout,craftsman,
validator,auditor}.md` (**exactly four roles** — `scribe` was invented without
provenance and is removed; `auditor` restored), `.specd/steering/{reasoning,workflow,
product,tech,structure,memory}.md`, and a marker-merged `AGENTS.md`. `RolePrompt` reads
the embedded role files — one source of truth. `new` writes `requirements.md` +
`design.md` + `tasks.md` + `state.json` (`design.md` required so the design gate is
reachable). Deliberately CUT: brain/pinky/reviewer as *roles*; the `skills/` SKILL tree
(reconceived as context-manifest item modes); `config.json`; the 7-key task schema
(`why`/`contract` dropped — skeleton is 6-key: `id/role/files/depends-on/verify/
acceptance`); front-loading `decisions.md`/`mid-requirements.md` (created on demand).
Deferred: `config.yml` seeding (spec 10); pinky subagent prompts + runtime gitignore (spec 09).

**ADR-11 — Standing regression (W7).** The concept↔functionality gap is closed by a
standing regression, not a one-time audit. Three deterministic scripts under `scripts/`
run at HEAD and are re-runnable on every push: `regress-all.sh` re-runs **every** `verify:`
in `review-specs/00..06/tasks.md` literally via `sh -c` and takes its verdict from the
exit-code log, never judgment (the W7 wave itself is excluded to avoid self-recursion);
`regress-lint.sh` is a static smell audit (authoring `specs/` read where runtime reads
`.specd/specs/` → G1; hollow existence-only verify → G4; `files:`/verify target failing
`test -e` → G3); `regress-domains.sh` re-asserts each wave's owned invariant black-box
against a freshly built binary in a throwaway tree (W0 honesty … W6 release), exiting on
the first violation. No LLM, no network in any verdict path — same determinism guardrail
as the gates. A wave stays open until its live evidence exists; no surface is unowned.

---

## 5. Scope triage: v1's 29 commands → 16 verbs

**v1 surface (16 verbs):** `init · new · check · approve · next · verify · task ·
status · context · decision · midreq · memory · report · handshake · mcp` + the opt-in
orchestration tier **`brain · pinky`**.

| Command | Verdict | Reason |
|---|---|---|
| `init` | SIMPLIFY | P1/P8. v1's 803-LOC init + 15K initplan over-built. Reduce to: scaffold `.specd/`, write embedded templates, emit one plan JSON. |
| `new` | KEEP | P2. Validates slug `^[a-z0-9][a-z0-9-]*$`, refuses overwrite, scaffolds a spec. |
| `check` | REDESIGN | P1/P3. The gate runner is the product's heart; restructure to the pluggable registry (ADR-4). |
| `approve` | KEEP | P6. Human phase-boundary transitions under spec lock — the paper's "last 20%" gate. |
| `next` | KEEP | P4. Frontier/scheduler query over the task DAG. Absorbs `waves` (as `--waves`) and `dispatch` (as `--dispatch`). |
| `verify` | SIMPLIFY | P3. Evidence gate absolute; simplify the record; sandbox becomes fail-closed opt-in. |
| `task` | KEEP | P3. Evidence-gated state mutation through `CompleteTask`. |
| `status` | KEEP | P7. Deterministic projection of `state.json`. |
| `context` | REDESIGN | The context engine elevated to a first-class central module (domain 08). |
| `decision` | KEEP | P2. Appends an ADR record under lock. |
| `midreq` | KEEP | P6. Mid-requirement gates; high/critical never auto-cleared. |
| `memory` | KEEP | P8. Steering memory add/promote; absorbs v1's `promote`. |
| `report` | SIMPLIFY | P7. Keep deterministic Markdown/PR-summary/Prometheus projections; DEFER live watch/SSE/webhook streams. |
| `handshake` | KEEP | P5. Host bootstrap + policy digest — the universal on-ramp. |
| `mcp` | SIMPLIFY | P5. stdio JSON-RPC server; parity-tested core tool set; cut raw passthroughs. |
| `brain` | REDESIGN | Deterministic controller = the paper's orchestrator mode; minimal verb set (ADR-3). |
| `pinky` | SIMPLIFY | Worker ACP protocol; safety-bearing verbs only (claim/heartbeat/report/inbox/checkpoint). |

**Merged away:** `waves`→`next/status` · `promote`→`memory` · `dashboard`→`report`.
**Cut outright:** `orchestrate` · `submit` · `migrate` · `doc` + Postgres/Redis backends.
**Deferred to v2/plugin:** `conductor` · program tier · the flywheel commands
(`eval · review · deploy · observe · ingest · harness`) + live report streams.
`security` returns as a pluggable gate module (`check --security`), not a command.
Hidden subsurfaces kept: `next --dispatch` (context engine), `brain_worker` runner seam
(test seam).

---

## 6. The twelve domains — decisions, contracts, invariants

Full EARS requirements and designs live in `specs/<nn>-<domain>/spec.md`. This section
preserves each domain's verdict logic and the contracts that must never regress.

### 01 · Product & Philosophy Core
Owns P1 + P8; defines the keep/cut line for everything. Deliverable: a **harness
charter** (`docs/charter.md`) mapping every verb to one harness component + one
principle — wired as a lint over the registry, not a doc convention. All agent-authored
artifacts (`requirements.md`, `design.md`, `tasks.md`) are **untrusted input**: the
system enforces, never authors, their content. Bare invocation prints the 16-verb
surface, exit 0. Conductor mode is "the same core with orchestration disabled" — no
dedicated code.

### 02 · Spec Lifecycle & State Model (the spine)
`state.json` is single machine truth: `State{ SchemaVersion:1, Revision, Mode, Status,
Phase, Tasks[], Decisions[], MidReqGates[], Records map }`. CAS on revision inside the
lock; atomic writes; loud-load on corrupt/newer-schema/invalid-status; injectable
`Clock`. **Forward-only phase ratchet** (`PlanningAdvance`): Requirements → Design →
Tasks → Executing → Verifying → Complete; backward/skipping transitions rejected;
`approve` refuses when phase-readiness gates fail. Mode per ADR-7. On-disk:
`.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`.
Risk noted: `Records` becoming a junk drawer — each plugin owns a documented schema.

### 03 · Validation Gates Engine
The **7 core gates** as pure functions over a read-only `CheckCtx`: EARS (1), Design (2),
Task-schema (3), DAG (4, orphans/cycles/wave order), **Evidence (5 — never opt-out)**,
Sync (6, checkboxes↔state), Traceability (7, req IDs). Opt-in gates (acceptance, scope,
context-budget, security; later eval/review/ingest) register via ADR-4's registry.
`error` finding → exit non-zero; highest `warn` → report but exit 0. Gates may pin a
severity floor (evidence is always `error`); config can raise, not lower. External
custom gates keep v1's subprocess contract (stdin/stdout JSON, scrubbed env, bounded
timeout). Registry gates receive only `CheckCtx` — no fs/net handles — so third-party
gates can't smuggle in nondeterminism.

### 04 · Task DAG & Wave Execution
Byte-stable `tasks.md` parser (`ParseTasks`/`SerializeTasks`): round-trip identity,
single-line rewrite, stable line numbers via length-preserving comment stripping;
mandatory keys, key order, valid roles enforced; duplicate ids / out-of-wave tasks
rejected. Pure DAG functions: `OrphanDeps`, `DetectCycle`, `WaveViolations`,
`NextRunnable`, `RunnableFrontier`; numeric ordinal tie-break (`T10 > T9`). `next`
returns the runnable frontier; terminal kinds exactly one of
`all-complete | all-blocked | waiting`. Task ids stay `T<n>`; waves are grouping, not an
id scheme. **Single reader:** one `LoadTasks` used by `next`, `check`, and the context
engine so they cannot diverge. Fuzz round-trip test is a first-class gate.

### 05 · Evidence & Verification
The evidence gate **is** P3 — without it every other gate is theater. `verify:` commands
run via `sh -c` (override `SPECD_VERIFY_SHELL`) with a **scrubbed allowlisted env**
(`PATH,HOME,LANG,LC_ALL,TMPDIR,SPECD_*`), NUL-byte commands rejected, exact command +
cwd printed before execution (the trust boundary is explicit: `tasks.md` is hostile
input). Evidence is an **append-only ledger** under the spec dir — records never
overwritten; completion references a specific record hash, not "the latest". Core
record: `{task, status, exitCode, command, cwd, changedFiles, startedAt, durationMs,
hash}` — criterion records and host cost/tokens live in `state.records`. Sandbox:
`config.verify.sandbox = off|bwrap|container`, default off; **when set and the binary is
missing, verify fails closed** (never silently unsandboxed). `--revert-on-fail` restores
the working tree (git-diff snapshot/restore), default off. **One completion path:** both
`task --status complete` and orchestration worker reports go through `CompleteTask`;
non-read-only completion requires a passing record; dual-write of `tasks.md`+`state.json`
is atomic under lock. Runner layered: exec → capture → record → gate.

### 06 · Agent-Agnostic Integration
**Four roles** (`.specd/roles/`), bound per task via `role:`: scout (read-only explore),
craftsman (write + verify), validator (read-only test run), auditor (read-only diff
audit). Read-only roles cannot be bound to write tasks. **Role-prompt injection is
deduplicated** — prompt bytes once per response via a shared `assets` map; hosts without
asset resolution use `--inline-roles` full-text fallback; subagent modes
`inline` (default) / `delegate` via `roles.subagent_mode`. **Steering constitution**
(`.specd/steering/`): reasoning, workflow, product, tech, structure, memory —
product/structure/tech are agent-authored; the harness scaffolds and enforces but does
not perceive the stack. **AGENTS.md marker-merge** replaces only the marker-delimited
section, preserving user content. **The `--config` snippet is the universal integration
floor — never removed to force adapter use.** Host adapters SIMPLIFIED to a five-method
`HostAdapter{ Detect, Plan, Install, Inspect, Verify }`: writes project-scoped,
unrelated keys preserved, ownership recorded in `.specd/integrations.json`; ship ≤1
reference adapter, gated by a shared conformance test kit. Snippet-first docs.

### 07 · MCP & Handshake Surface
stdio JSON-RPC 2.0 server, stdlib-only (HTTP transport DEFERRED). v1 tool set:
`specd_check, specd_next, specd_verify, specd_task, specd_status, specd_context,
specd_handshake` + 3 brain tools (`brain_orchestrate`, `brain_status`, `brain_approve`)
**registered only when orchestration is enabled**. Raw passthroughs CUT (no added
authority, double surface). **Parity is a gate:** every exposed tool has a test
asserting tool result == CLI JSON for the same input; tool registration is data-driven
from the command registry so there is no second hand-maintained list. Per-spec tool
policy from `manifest.json` (`required/optional/forbidden`) enforced server-side;
malformed manifest degrades to **empty policy, not open**. `handshake bootstrap` returns
schema version + effective policy digest. Report/decision/memory are deliberately *not*
tools in v1 — the tool set is the enforcement/query core, not authoring.

### 08 · Context Engineering (the paper's core skill)
**One shared manifest engine, three surfaces**: `specd context`, `next --dispatch`, and
worker briefs are all produced by `BuildContextManifest` — they cannot drift. Exactly
four item modes: `read-full`, `read-targeted`, `run-command`, `reference-if-needed` —
mapped to the paper's context types (role prompt/steering = static instructions;
knowledge/examples = reference-if-needed; working slice = read-targeted; tools = the
manifest tool policy; guardrails = the gates). Token estimation is a **pure heuristic**
(`ceil(len/4)` + markdown surcharge) — never an LLM or network tokenizer. Budget:
`SPECD_MAX_CONTEXT_TOKENS`; the **context-budget gate** (opt-in, domain 03) turns
"context stuffed with noise" into an enforceable failure — advisory by default, enforced
by choice. Builder stays in `internal/context` (import-cycle rule, ADR-0); core exposes
only the mission adapter. The manifest carries **references + modes, never inlined
content** (the host reads; keeps context small, honors P2). Manifest validation rejects
malformed item order/kinds/modes/paths.

### 09 · Orchestration (Brain/Pinky) — opt-in tier
The Brain is a **deterministic controller that never calls an LLM**: `Decide(Snapshot) →
Decision{Action, Task?, Reason}` is pure — zero IO, zero randomness; actions
`dispatch | wait | await-approval | escalate | policy-violation | complete`. `Sense`
builds the snapshot from state+frontier+leases. All Brain↔worker interaction is a
**file-backed ACP** (append-only `acp/*.jsonl`, restart-recoverable) under
`.specd/specs/<slug>/orchestration/{session.json,acp/,leases/}`; `session.json` is
CAS-guarded. Safety core: per-spec lock at claim; lease keepalive via heartbeat, reclaim
after expiry, reschedule up to `maxRetries` then escalate; **advisory cost brake** (sums
untrusted host-reported cost, halts with policy-violation over
`hostReportedCostLimitUSD`); **time brake** (mission deadline, process-group kill);
**cooperative cancel** (records intent, workers self-stop). Worker `Report{Task,
VerifyRef, GitHead, ChangedFiles, DurationMs, HostCost, HostTokens}` accepted **only if
it references a passing verify record**; duplicate terminal reports idempotent.
**Fail-closed authority:** default `enabled:false`, `approvalPolicy:manual`,
`workerMode:host`, `transport:file`; no policy can clear high/critical midreq gates;
when disabled, zero orchestration behavior and unchanged CLI/check output. `checkpoint`
/resilience opt-in via `resilience.checkpointEnabled` (off by default). The program
(multi-spec) tier is DEFERRED — `Decide` is designed to be liftable to program scope later.

### 10 · CLI Architecture & Foundations (the floor)
Zero-dep custom arg parser (~40 lines, deliberately not Cobra) + flat dispatch registry.
**One `[]Command` table, three consumers**: dispatch, help, and the MCP tool list all
derive from it — `TestRegistryMatchesHelp` (help can't drift) is CI-blocking. Atomic
write (`AtomicWrite`: MkdirAll → CreateTemp same dir → write → fsync → chmod 0644 →
rename; `AppendFile` fsyncs). Reentrant advisory lock: cross-process `O_CREATE|O_EXCL`
`.lock` (pid+unix-ms), stale reclaim (`SPECD_LOCK_STALE_MS`, 30s default), in-process
per-path mutex, goroutine-id reentrancy, acquire timeout (`SPECD_LOCK_TIMEOUT_MS`, 5s).
Config: `LoadConfig(paths, env) → (Config, []Diagnostic)` pure over inputs; YAML
two-space-indent subset; global→project→env cascade; validates + secret-scrubs the
orchestration block; fail-loud on truncated scalars; sha256 effective-config digest.
`FindSpecdRoot` walks up; not found → exit 3. Exit codes: `OK / Gate / Usage / NotFound`.
`EnvInt` clamps + warns once on malformed `SPECD_*`. Slug grammar `^[a-z0-9][a-z0-9-]*$`.
Help text: registry-generated usage + short summary; long-form only where a verb needs examples.

### 11 · Reporting & Observability
KEEP: `status` + deterministic `report` (Markdown, PR summary, Prometheus textfile) —
pure projections of `state.json`, **no LLM/network in any render path**; host telemetry
stored verbatim, never synthesized. DEFER: live frontier streams (watch/SSE/webhook —
keep the `FrontierEvent` type so streams can return without a rewrite), `dashboard`
(fold into `report --dashboard` if demand appears), session replay/trajectory renderers
(keep the append-only `trajectory.jsonl` digest as evidence). SIMPLIFY `internal/obs` to
a minimal stdlib logger + the deterministic metrics textfile.

### 12 · Flywheel / Triage Tier (the primary subtraction)
v1 ships **no flywheel commands** — only the `security` gate module (stdlib-only
secrets/injection/slopsquat scanners, off by default; allowlist at
`.specd/security/allow.json` — entries **require a reason**, reasonless entry is a hard
error; `check` output unaffected when off).
DEFERRED with preserved contracts: `eval` (rubric engine; Gate 10 hook stays), `review`
(Gate 11 hook stays; human approval remains final), `deploy` (preconditions read
evidence only, never re-run gates; production requires a human `DeployApproval` record),
`observe` (offline error-payload correlation), `ingest` (brownfield onboarding:
inventory + Gate 13 coverage — every file mapped or waived-with-reason), `harness`
(policy bundles: SHA256 pinning + import quarantine when it returns), the maintenance/
program tier. CUT: `submit`, `migrate`. Re-entry is **only** via the Gate interface +
`state.records` — deferred, not deleted: evidence shapes stay documented so v2 modules
slot in without re-litigating contracts.

---

## 7. Roadmap: dependency DAG, authoring order, build waves

**Cross-spec dependencies (why-edges):** 01 constrains everything. 10 → 02 (state needs
io/lock/CAS), 10 → 07 (registry drives tools), 10 → 09 (config + file backend). 02 is
the spine → 04, 05, 03, 09, 11. 04 → 03 (DAG gate) and 09 (Brain dispatches frontier).
05 → 03 (evidence gate) and 09 (reports validated against records). 08 → 03
(context-budget gate), 07 (`specd_context`), 09 (worker brief). 06 → 07, 09. 09 → 11/12.

**Authoring order (topological):** 01 → 10 → 02 → 04 → 05 → 03 → 08 → 06 → 07 → 09 → 11 → 12.

**Build waves:** A foundations (10, 01) → B state close-out (10, 02, 01) → C lifecycle &
parser (02, 04, 05) → D gates/evidence/dispatch (03, 05, 04) → E context & integration
(08, 06, 03) → F surfaces (07, 08, 11) → G orchestration (09) → H flywheel-minimal (12).

**Critical path:** 01 → 10 → 02 → 05 → 03 → 08 → 09. Orchestration is last because it
composes the most; reporting (11) and flywheel (12) can slip a wave without blocking.

**Definition of done — a spec:** (a) EARS-shaped, testable requirements; (b) design names
module boundaries + on-disk contracts + preserved invariants; (c) task DAG with
`id/role/files/depends-on/verify/acceptance` grouped into waves; (d) every claim cites a
reference file + a KEEP/SIMPLIFY/REDESIGN/CUT/DEFER verdict.
**Definition of done — a task:** its `verify` command passes and the record is written
(exit code + HEAD); it touches only its declared `files:`; the §3 guardrails still hold.

---

## 8. Current position (audited 2026-07-04, branch `fresh-start` @ c16d4f9)

Stages 1–3 all produced output: `specs/` holds 13 authored specs (12 domains +
`13-cli-regression`) with `specs/progress.md` as tracker; the Go tree
(`internal/{cli,cmd,core,context,integration,mcp,orchestration}`) builds clean, vets
clean, and passes 84 tests across 12 packages. An independent hard review
(build + e2e binary drive + code read at the seams) reached this verdict:

> The rebuild is **architecturally faithful and mechanically healthy** but **not yet the
> product the specs describe**. The central promise — evidence gates every state change,
> humans approve phase transitions, agents can bypass neither — is scaffolded but not
> enforced end-to-end.

**Verified green (checked, not trusted):** zero-dep build; atomic write/CAS/reentrant
lock actually used by lifecycle commands; parser round-trip + fuzz; append-only
`evidence.jsonl` (failed runs recorded too); `verify --revert-on-fail`; pluggable gate
registry with security gate behind `--security`; pure `Decide()` + no-LLM tests; report
purity; exactly 4 embedded roles (scribe gone, auditor present); deferred verbs fail
loud; marker-merged AGENTS.md on init; the subtraction held (no Postgres/Redis, no
program tier, no conductor analytics, no legacy JSON config). ADR-3/4/5/6/9/10
structurally honored.

**Open findings (🔴 breaks a guardrail · 🟠 spec-vs-impl drift · 🟡 quality):**

- 🔴 **F1 — `specs/progress.md` contains falsified completions.** All 76 tasks marked ✅
  but several verify commands fail today (missing `docs/charter.md`, `docs/context.md`,
  `docs/deferred-flywheel.md`; status JSON has no `mode` field; no evidence records
  exist for the rebuild tasks — the rebuild did not dogfood specd). The project's own
  cardinal sin: never mark ahead of evidence. Trust the tracker only after re-audit.
- 🔴 **F2 — The core loop cannot close.** No `task complete` verb; `verify` success
  doesn't transition task state; task status derives from a `tasks.md` marker column the
  scaffold doesn't even emit. The think→act→observe loop has no observe→record→advance closure.
- 🔴 **F3 — Approvals and phases gate nothing.** `next`/`verify`/`context`/`report`
  never read `state.json`; `next` dispatches pre-approval. The phase ratchet writes
  state nothing consults.
- 🔴 **F4 — MCP lets an agent approve its own human gates.** `ForbiddenTool` blocks only
  `report, decision, memory`; `approve`, `init`, `brain`, `mcp` are agent-callable.
- 🔴 **F5 — ADR-7 mode enum unimplemented.** Only `ModeDefault` exists; no `--mode`, no
  transition, no mode in status, orchestration eligibility never keys off it.
- 🔴 **F6 — `decision`/`midreq` capture no content** (no text/rationale/actor/timestamp/
  HEAD); approvals record only `{"gate":"design"}`. The audit trail is hollow.
- 🟠 **F7 — 18 verbs shipped vs spec'd 16** (`memory`, `triage` extra; ADR-5 violation —
  cut or write a superseding ADR).
- 🟠 **F8 — Missing content gates:** no EARS gate over `requirements.md`, no
  approval/phase gate, no Sync gate, design gate reads nothing.
- 🟠 **F9 — Steering + memory scaffolded but inert:** nothing reads them; the context
  manifest includes none of them (paper P8 unrealized).
- 🟠 **F10 — Config layer:** loads `project.yml` not `config.yml` (ADR-2), errors
  swallowed (fail-silent), `init --agent` accepted and ignored; no config ever seeded.
- 🟠 **F11 — `brain start` not fail-closed** (session created without
  `orchestration.enabled`); `pinky` verbs exist in code but are unregistered.
- 🟠 **F12 — Repo's own `.specd/` contradicts the scaffold:** stale `roles/scribe.md`,
  missing `auditor.md`, junk `specs/demo/` with an invalid `builder` role.
- 🟡 **F13 — progress.md `files:` don't match the real tree** (work consolidated into
  `lifecycle.go`/`registry.go`/`report.go`; tracker never corrected).
- 🟡 **F14 — smaller gaps:** evidence accepts `git_head:"unknown"`; no timestamps on any
  record; `check` silent on success; `task <id>` scans all specs (inconsistent `<slug>`
  shape); manifest path prefixes inconsistent; the `--unverified --evidence` escape
  hatch is documented but implemented nowhere.

**Paper-adherence gaps:** P3 (evidence gates state change) 🔴 F1/F2/F3 · P6 (humans gate
transitions) 🔴 F3/F4/F5 · P8 (steering/memory flywheel) 🔴 F9 · P2/P5/P7 🟠.

### The path to production (ordered waves; every task should be driven through specd itself — dogfood)

- **Wave P0 — Restore truth (blocks everything):** re-audit `progress.md` (run every
  `verify:` literally; flip false ✅; fix `files:`); reset the repo's own `.specd/`
  (4 roles exactly, delete junk demo spec); write the three missing docs
  (`docs/charter.md`, `docs/context.md`, `docs/deferred-flywheel.md`).
- **Wave P1 — Close the loop:** `task complete` refusing without a passing evidence
  record at current HEAD, status in `state.json` under lock+CAS; gate `next`/`verify` on
  approved requirements+design; implement ADR-7 mode enum end-to-end; add
  `--unverified --evidence` for read-only roles only.
- **Wave P2 — Seal the trust boundary:** ForbiddenTool blocks `approve/init/mcp/brain`
  over MCP (assert the deny list in the parity test); `brain start` fail-closed on
  config **and** `mode: orchestrated`; register `pinky` verbs per ADR-3 or supersede by ADR.
- **Wave P3 — Make records mean something:** `midreq`/`decision` require `--text`; every
  record gets timestamp + git HEAD + actor; reject `git_head:"unknown"` for completion.
- **Wave P4 — Finish the gate engine, wake the constitution:** EARS gate (error on
  unedited scaffold stub), approval/design-stub gates, steering + memory into the
  context manifest (bounded, budget-counted), byte-identical parity preserved.
- **Wave P5 — Surface & config reconciliation:** resolve 16-vs-18 by ADR (recommended:
  cut `triage`; fold `memory` in via superseding ADR only after P4 makes it functional);
  `config.yml` seeded by `init`, fail-loud; CLI consistency (`task <slug> <id>`,
  normalized manifest paths, one-line green `check` summary).
- **Wave P6 — Hardening & release:** CI (build+vet+test -race+fuzz smoke+e2e), static
  release binaries (`CGO_ENABLED=0`, version stamped), `--version`, dogfood gate (this
  repo's `.specd/` carries waves P1–P6 closed with real evidence), docs pass.

**What NOT to do:** no new packages/dependencies/"task engine" abstraction (fixes land
in existing files); don't restore flywheel features to fix F7 — subtract the stub;
don't move task status back into `tasks.md` markers (ADR-1 chose state.json — finish
that choice); **don't ship before P0 + P1 + the MCP deny-list land** — a spec-discipline
tool whose own tracker lies, whose loop can't close, and whose agent surface can
self-approve teaches users that evidence is theater.

---

## 9. Repository map (now)

```
/
├── PROJECT.md            ← this file (supersedes the removed fresh-start artifacts)
├── AGENTS.md / CLAUDE.md ← thin pointers here
├── The_New_SDLC_With_Vibe_Coding.pdf  ← the paper
├── main.go, go.mod       ← the rebuilt binary (zero deps)
├── internal/{cli,cmd,core,context,integration,mcp,orchestration}
├── specs/                ← 13 authored specs (spec.md + tasks.md) + progress.md
├── .specd/               ← this repo's own scaffold (needs the P0.2 reset)
├── reference/            ← FROZEN v1 — read-only museum; never import/build/copy
└── scripts/, .github/
```
