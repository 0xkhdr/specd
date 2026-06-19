# Pinky & The Brain — Gap Analysis & Hardening Plan

> **Status:** Analysis — derived from the current `main`/`pinky-and-the-brain` tree (37/37 spec tasks complete).
> **Scope:** What is missing or weak between the *shipped deterministic control plane* and the stated vision of a *worldwide coding-agent harness that adopts any spec and drives it from beginning to delivery using Brain + Pinky.*
> **Date:** 2026-06-19

---

## 0. How to read this document

The design doc (`pinky-and-the-brain-analysis.md`) describes an **LLM-native Brain** that "autonomously runs init → steering → requirements → … → complete." The **shipped implementation deliberately diverged** to a safer model:

- **Brain = deterministic controller.** Zero LLM calls, zero provider SDK, zero network. It *senses* specd state and emits **one bounded decision per `step`** (`internal/core/orchestration_decide.go`).
- **Pinky = host-executed worker contract.** specd writes missions/leases/reports over file-based ACP; it **never spawns a model agent or kills a process** (`docs/agent-integration.md:127-139`).
- **The real LLM (Claude Code / MCP host) is the outside driver.** It runs the loop, reads decisions, and does creative work.

This divergence is **correct and worth keeping** — it is the source of the project's auditability and safety. The gaps below are therefore *not* "make Brain an LLM." They are: **the glue, the missing decision branches, and the host-side runtime that together let a real agent actually run a spec end-to-end without a human hand-walking every step.**

---

## 1. What is already solid (do not rebuild)

| Area | Evidence | Verdict |
|---|---|---|
| Deterministic decision engine | `orchestration_decide.go` — total switch, idempotency keys, escalation codes | Strong. Replayable, testable. |
| Snapshot/decision/session validation | `orchestration.go` `Validate*` functions | Strong. Fail-closed on malformed state. |
| ACP file transport + leases + CAS | `core/acp*.go`, `acp_lease.go`, `acp_security_test.go` | Strong. Lease reclaim, idempotent terminal reports, security tests. |
| Trust labeling of host telemetry | `roles/pinky.md`, mission `ACPAuthority`, `hostReported` semantics | Strong. Token/cost/stdout never count as proof. |
| Evidence gating | completion still requires `specd verify` record + task integrity | Strong. The core safety invariant holds. |
| Config back-compat / fail-closed | `agent-integration.md:150-153` | Strong. |
| Program DAG + frontier | `program.go`, `StepProgramOrchestration` | Present and exercised. |

**Conclusion:** the *control plane* is production-grade. The *harness experience* — the thing that makes it "drive any spec from beginning to delivery" — is incomplete. Every gap below sits in that second layer.

---

## 2. Gap inventory (prioritized)

Severity: **P0** blocks the end-to-end vision · **P1** major friction/risk · **P2** polish/scale.

---

### GAP-1 (P0) — Brain orchestrates *execution only*, not the *planning phases* — ✅ DONE

> **Implemented (Milestone A):** authoring frontier in `senseAuthoring` (`orchestration_authoring.go`), `dispatch-authoring` + `advance-phase` decision branches (`orchestration_decide.go`, gated on `planning`/`session` policy; `manual` still requests approval), artifact missions via reserved `A1/A2/A3` work IDs (`BuildAuthoringMission`), and `AdvancePlanningPhase` mirroring `specd approve`'s gate. Execution dispatch gated to execution statuses so reconciled tasks.md never runs before the `tasks→executing` gate. Tests: `orchestration_authoring_test.go`, golden drive in `orchestration_driver_test.go`.


**The single biggest gap.** `DecideOrchestration` and `senseRunnableTasks` operate exclusively on the **task DAG** (`orchestration_sense.go:59` reads `RunnableFrontier`). For a spec in `requirements`, `design`, or `tasks` status there are **no DAG tasks yet**, so:

- `senseRunnableTasks` → empty
- `DecideOrchestration` → `request-approval` (if gate awaiting) **or** `wait — "no runnable work"`

There is **no decision branch that dispatches an authoring mission** ("write EARS requirements", "write design.md", "decompose task DAG"). The original design's decision tree (analysis doc §4.3, Appendix B) had exactly these branches; they were not carried into code.

**Consequence:** Brain cannot take a fresh spec from `new` to a runnable task DAG. A human/host must hand-author requirements → design → tasks *outside* the brain loop, then Brain can finally schedule execution. That is **not** "from beginning to deliver" — it is "deliver, once a human has already planned."

**The `planning` approval policy is not a substitute.** It only *advances gates when they already pass* (`agent-integration.md:143-144`); it never *produces* the artifact that makes the gate pass.

**Action:**
1. Extend the snapshot model with an **authoring frontier**: when `Status ∈ {requirements, design, tasks}` and the phase artifact is absent/failing `specd check`, surface a synthetic "authoring work item" with `role=builder` (or `investigator` when the repo/steering is unknown).
2. Add decision branches `dispatch-authoring` in `orchestration_decide.go` that emit a mission whose contract is "author `<artifact>` for `<spec>` to pass `specd check`."
3. `BuildPinkyMission` must support **artifact missions** (no `taskId`): contract = phase contract, verify = `specd check <spec>`, files = the phase artifact path.
4. Keep fail-closed: authoring dispatch only under `planning`/`session` policy; `manual` still requests approval first.

**Acceptance:** `specd brain start <new-spec>` followed by repeated `step` (under `planning` policy) drives requirements → design → tasks → executing → verifying → complete with **no manual artifact authoring**, every transition still gated by `specd check`/verify.

---

### GAP-2 (P0) — No reference driver loop; the "harness" is a single bounded `step` — ✅ DONE

> **Implemented (Milestone A):** `DriveOrchestration` (`orchestration_driver.go`) — a shipped, tested outer loop that steps, hands each dispatch to a host worker callback, blocks until the worker returns (the dispatch→spawn contract), and stops on a terminal `DriverOutcome` (`complete|escalated|awaiting-approval|worker-stop|max-steps|stalled`). Surfaced as `specd brain run <slug> [--worker-cmd] [--max-steps]` with sane planning defaults and resumable sessions (`ActiveOrchestrationSessionForSpec`). Golden test drives a fresh spec to completion with a stub worker, zero model call in core (`orchestration_driver_test.go`).


`specd brain step` performs **one** decision + at most one action. The outer loop ("step → if dispatch, spawn worker → worker claims/executes/reports → step again, until complete/escalate") **exists only as prose** in `agent-integration.md:161-173`. There is no executable artifact that ties it together.

**Consequence:** every host re-implements the loop, the back-off, the "is the session done?" check, and the dispatch→spawn handoff. This is the difference between "a set of correct primitives" and "a harness." For a *worldwide* harness that adopts *any* host, the loop must be a shipped, testable asset — not folklore.

**Action (keep the no-LLM-in-core boundary intact):**
1. Ship a **host-side driver** as a first-class deliverable — a `specd-driver` skill/SKILL.md plus a thin reference script (`scripts/drive.sh` or a `specd brain run` command that loops `step` with poll/back-off but still **delegates creative work to the host**, blocking on a host-provided worker callback).
2. Define the **dispatch→spawn contract** explicitly: when a `step` returns `action=dispatch` with a mission digest, the driver must (a) materialize the mission, (b) spawn a worker, (c) not `step` again for that slot until the worker reports or the lease expires.
3. Provide a **session-complete predicate** the loop can poll (`status` already returns enough; document/encode the terminal set: `complete | failed | escalated | awaiting-approval`).

**Acceptance:** a documented, tested loop (golden test over a scripted ACP event sequence) that drives a spec to completion with a stub worker, proving the handoff without any model call in core.

---

### GAP-3 (P0) — No bridge from a Pinky mission to an actual worker agent — ✅ DONE

> **Implemented (Milestone A):** worker-agent templates embedded + scaffolded to `.claude/agents/pinky-{builder,investigator,reviewer,verifier}.md` (`embed_templates/agents/`, wired in `scaffold.go`; fresh-init external-write support added in `initplan.go`). Mission→prompt renderer `specd pinky brief --session --worker --spec (--task | --artifact) [--json]` (`pinky_brief.go`, `RenderMissionBrief`) emits a paste-ready, context-engineered brief (or claimable mission JSON). Brief assembly lives in the harness, not each host's head.


`BuildPinkyMission` (`core/pinky.go:39`) produces rich mission JSON (role, contract, files, acceptance, verify, authority). `ClaimPinkyMission` leases it. **Nothing turns that mission into a running LLM worker.** The host is told "start or assign its own worker" (`agent-integration.md:166`) with no template, no sub-agent definition, no prompt assembly.

For Claude Code specifically, the natural worker is a **`Task` sub-agent** (or a `.claude/agents/pinky-*.md` definition) whose system prompt = `roles/<role>.md` + the mission brief (Appendix C template already exists in the design doc). That wiring is absent from the repo.

**Consequence:** the "Pinky executes" half of the slogan is a manual copy-paste job. There is no reproducible, context-engineered worker spawn.

**Action:**
1. Ship **worker-agent templates** in `embed_templates` — e.g. `.claude/agents/pinky-builder.md`, `pinky-investigator.md`, etc. — each a thin shell that loads `roles/<role>.md`, the `specd-pinky` skill, and instructs "read mission from `--mission`, claim, execute, verify, report."
2. Add a **mission→prompt renderer**: `specd pinky brief <session> <worker> --task <id>` that emits the fully-assembled mission brief (Appendix C of the design doc, already specced) ready to paste into a sub-agent prompt. This keeps context engineering *in the harness*, not in each host's head.
3. Document the **two supported worker modes** clearly: `host` (current — operator wires it) and a documented `subagent` recipe for Claude Code's `Task` tool. (Do **not** make core spawn agents — keep the boundary.)

**Acceptance:** a one-page recipe + embedded agent templates such that "Brain dispatches → operator runs one documented command → a Claude sub-agent claims, works, verifies, and reports" with zero ad-hoc prompt writing.

---

### GAP-4 (P1) — Cost / budget limit is declared but not enforced — ✅ DONE

> **Implemented (Milestone B):** `SenseOrchestration` now accumulates host-reported cost across a session's evidence events (`senseHostReportedCost` + `parseHostCostUSD` in `orchestration_limits.go`, dedup by MessageID, fail-soft on unparseable untrusted input) into `snapshot.AccumulatedCostUSD`, and surfaces a fixed wall-clock `snapshot.SessionExpired` from `session.ExpiresAt`. Two new `DecideOrchestration` branches escalate with `EscalationPolicyViolation` — `costLimitExceeded` (when `sum(host-cost) ≥ limit > 0`) and session timeout — placed before dispatch so the brain halts instead of scheduling more work. Cost stays labeled advisory/untrusted; a limit of `0` disables enforcement. Tests: `TestOrchestrationCostLimitEscalates`, `TestOrchestrationSessionTimeoutEscalates`, `TestParseHostCostUSD`, and end-to-end `TestOrchestrationCostLimitEndToEnd` (real evidence → step escalates → session marked failed).

`hostReportedCostLimitUSD` lives in policy (`orchestration.go:55`) and config, but it is **host-reported and explicitly untrusted** (`agent-integration.md:133-135`), and **no decision branch stops the session when cumulative reported cost exceeds the limit.** `DecideOrchestration` never reads cost. The design doc lists "cost explosion" as a High risk (§13.3) with "cost limits" as the mitigation — the mitigation is not wired.

**Consequence:** "full autonomy" (Level 3) has no real brake. Even as a soft/advisory limit it should halt-and-escalate.

**Action:** accumulate host-reported cost per session in the session file; add a `DecideOrchestration` branch → `escalate` with `EscalationPolicyViolation` (new code or reuse) when `sum(host-cost) ≥ limit > 0`. Label it advisory (untrusted input) but make it *act*. Same pattern for `time_limit` / `sessionTimeoutSeconds` (verify the timeout actually forces a terminal decision, not just lease expiry).

**Acceptance:** a session whose stub workers report cost over the limit escalates on the next `step` instead of dispatching more work.

---

### GAP-5 (P1) — No high-level MCP orchestration verb; raw CLI passthrough only — ✅ DONE

> **Implemented (Milestone B):** six intent-level MCP tools (`internal/mcp/intent.go`) — `brain_orchestrate`, `brain_status`, `brain_approve`, `brain_pause`, `brain_resume`, `brain_cancel` — modelled with named arguments (no positional `args` array) and translated to the same deterministic `brain`/`approve` primitives (no new core authority). `brain_orchestrate(spec[,goal,worker_cmd,…])` wraps `brain run` (bootstraps a missing spec via `goal`→`--title`, planning policy default, stops at first dispatch without a `worker_cmd`). Routed in `callTool` ahead of the raw `specd_*` passthrough, which is retained for power users; `serverInstructions` now points hosts at the intent layer first. Validation fails closed (missing required / wrong-typed args → invalid-params, never dispatches). Tests: `intent_test.go` (`TestIntentToolTranslation`, `TestIntentToolValidation`) + updated tool-list/golden/e2e parity counts via exported `IntentToolCount`. Docs: `agent-integration.md` "Driving Brain/Pinky from MCP hosts".

MCP exposes `specd_brain` / `specd_pinky` as **generic `args`-array passthrough** (`agent-integration.md:155-159`, `mcp.go` re-dispatches via `cmd.Dispatch`). There is **no** `brain_orchestrate(goal, spec, constraints)` semantic tool as the design doc promised (§7.1).

**Consequence:** the MCP client must know the full flag surface (`--approval-policy --max-workers --max-retries --timeout-seconds …`) and run the whole loop itself. That is high cognitive load and brittle — the opposite of "drivable from any MCP client." For context engineering, one well-described high-level tool beats six low-level ones.

**Action:** add a small number of **intent-level MCP tools** that wrap the loop and sane policy defaults: `brain_orchestrate`, `brain_status`, `brain_approve`, `brain_pause/resume/cancel`. They translate to the same deterministic primitives (no new core authority) but give the model a single clear affordance. Keep the raw passthrough for power users.

**Acceptance:** an MCP client can start and monitor an orchestration with one tool call carrying a goal + spec, no flag plumbing.

---

### GAP-6 (P1) — Steering / init bootstrap is outside the orchestration loop — ✅ DONE (deterministic preflight)

> **Implemented (Milestone A):** `OrchestrationPreflight(root, slug)` (`orchestration_preflight.go`) detects missing workspace/steering/spec and reports the deterministic remedy for each. `specd brain run --bootstrap` auto-creates a missing spec (`specd new`) before driving; missing workspace/steering fail closed with actionable guidance. *Note:* authoring steering **content** via an investigator→builder mission chain from a truly bare repo (vs. deterministic init+new) remains a Milestone B follow-up; the preflight + `--bootstrap` covers init/new bootstrap, and authoring the spec's content is then the GAP-1 frontier's job.


The design decision tree starts with "Is `.specd/` initialized? Is steering bootstrapped?" (Appendix B). The shipped Brain assumes a spec already exists and is loaded (`SenseOrchestration` → `LoadSpec`). There is no orchestrated path for **init**, **steering bootstrap**, or **`specd new <spec>`**.

**Consequence:** "adopt *any* spec or program and work from beginning" can't start from a bare repo. A human must `init`, bootstrap steering, and create the spec before Brain has anything to sense.

**Action:** add a **pre-spec orchestration preflight** (can live in the driver, GAP-2): detect missing `.specd/`, missing steering, missing spec; dispatch an `investigator` mission to inspect the repo, then a `builder` mission to author steering, then `specd new`. Couple tightly with GAP-1 (authoring missions).

**Acceptance:** `brain orchestrate --goal "<X>"` against a repo with no `.specd/` reaches a runnable spec without manual `init`/steering/new.

---

### GAP-7 (P1) — Program Brain reuses per-spec planning gap; no autonomous cross-spec walk proven — ✅ DONE

> **Implemented (Milestone B):** `DriveProgramOrchestration` (`orchestration_driver.go`) — the program-scoped peer of `DriveOrchestration`. It loops `StepProgramOrchestration` and hands every child dispatch to the same host worker callback (`ProgramDriverDispatch{Slug, ChildSessionID, Dispatch}`), blocking per dispatch, until the program session reaches a terminal `ProgramDecision` (`complete | escalate`), with stall/max-step guards. Surfaced as `specd brain run --program [--worker-cmd] [--max-steps]` (`brain.go`, reusing the single-spec shell-out worker contract via `brainRunProgramWorker`). **Frontier auto-advance verified:** `StepProgramOrchestration` already calls `releaseCompleteProgramChildren` → rebuild snapshot → `DecideProgram` at the top of every step, so the loop advances to the next spec on child completion with **no external nudge** — no fix needed there. **The "stops between specs" cause that *was* found:** children stall at `verifying` because the `verifying → complete` acceptance-evidence gate (`specd approve` Case 2) is verifier/host-owned by design — core marks the child *session* complete but never auto-clears the spec's evidence gate, so `releaseCompleteProgramChildren` (keyed on spec `StatusComplete`) never fires. The golden test's stub worker stands in for that verifier+approve step (the host's job), preserving Invariant 2. Test: `TestDriveProgramOrchestrationCrossSpecWalk` — a 3-spec linear program (`auth → api → web`) of *fresh* `requirements`-stage specs driven to full completion (all 9 artifacts authored, all 3 specs `complete`, program session `complete`) by the loop alone with a stub worker, zero model call in core.

`StepProgramOrchestration` resolves the program frontier and delegates to per-spec stepping. But since per-spec stepping can't author planning artifacts (GAP-1), a program of *fresh* specs stalls the same way, only wider. Also unverified: does the program loop **re-resolve the frontier and advance to the next spec automatically** on child completion, or does it need an external nudge per spec?

**Action:** after GAP-1/GAP-2 land, add a program-level golden test: a 3-spec linear DAG (`auth → api → web`) of *empty* specs driven to full completion by the loop alone. Fix any "stops between specs" behavior.

**Acceptance:** one `program orchestrate` call + the driver loop completes a multi-spec program end-to-end with stub workers.

---

### GAP-8 (P2) — Observability: decision log exists, but no `replay`/`why` narrative for humans

There is `replay.go` and a decision/event log, but the design promised "every Brain decision … replayable via `specd replay`" with human-readable *reasoning*. Confirm `specd replay` reconstructs a session timeline (decisions + reasons + escalations) in one view, and add a `brain why --session <id>` that explains the *current* decision in plain language. Critical for trust when autonomy is high.

**Action:** verify `replay` covers orchestration events; add a one-shot human-readable session timeline. Low effort, high trust payoff.

---

### GAP-9 (P2) — Context-engineering assets for workers are thin — ✅ DONE

> **Implemented (Milestone C):** Pinky missions now carry a deterministic `contextManifest` (`PinkyMission.ContextManifest`, emitted in mission JSON and ACP mission payloads) with read order, phase-scoped skill, required vs optional items, per-item token hints, and a soft token ceiling. `RenderMissionBrief` prints the manifest as the canonical worker context contract. Worker templates and `specd-pinky` guidance now tell hosts to follow the manifest instead of assembling context ad hoc. Tests cover execution manifests, authoring manifests, ACP/schema conformance.

Worker context today = `specd context <spec>` + `roles/<role>.md` + mission files. For a *worldwide* harness, the worker prompt budget is the scarce resource. Missing:

- **Token-budgeted context packaging** per mission (the harness, not the model, should decide how much steering/design to inline vs. reference).
- **Phase-scoped skill loading** (a builder in `executing` shouldn't load steering-authoring guidance).
- **A "minimal sufficient context" contract** so two different hosts give a worker the same context for the same mission (reproducibility).

**Action:** define a mission-context manifest (what to read, in what order, with a soft token ceiling) and emit it from `pinky brief` (GAP-3). This is the "context engineering" half of the user's ask and currently lives only as role-prompt prose.

---

### GAP-10 (P2) — Sub-agent communication is one-way request/report; no mid-task `query`

The design's ACP defined a `query` message (Pinky → Brain: "I need clarification") and `directive` (Brain → Pinky: correction/reassign). The shipped Pinky verbs are `claim|heartbeat|progress|report|block|release` — **no `query`**. A blocked worker must fully `block` and stop rather than ask a bounded question and continue. For long autonomous runs this forces unnecessary escalations.

**Action (optional, scale):** add a lightweight `query`/`directive` round-trip, or explicitly document that "block + re-dispatch with augmented mission" is the sanctioned substitute. Decide deliberately; don't leave it as an unspoken omission.

---

## 3. Prioritized action plan

### Milestone A — "Beginning to delivery" (unblocks the core vision) — P0 — ✅ COMPLETE
1. ✅ **GAP-1** authoring-frontier + `dispatch-authoring` decisions + artifact missions.
2. ✅ **GAP-3** worker-agent templates + `pinky brief` mission renderer.
3. ✅ **GAP-2** reference driver loop + dispatch→spawn contract + golden test.
4. ✅ **GAP-6** pre-spec preflight (init/new) folded into the driver (`--bootstrap`).

*Exit:* a workspace → `specd brain run <slug> --bootstrap --worker-cmd <w>` → completed spec, every gate honored, no manual artifact authoring. Verified by the `orchestration_driver_test.go` golden test (fresh `requirements` spec → `complete` with a stub worker) and an end-to-end CLI drive. *Retained gate:* final `verifying → complete` still requires the acceptance-evidence gate (`specd approve` Case 2 / a verifier worker) — by design, the evidence invariant is never auto-cleared.

### Milestone B — Safety & ergonomics — P1 — ✅ COMPLETE
5. ✅ **GAP-4** enforce cost/time limits → escalate.
6. ✅ **GAP-5** intent-level MCP tools (`brain_orchestrate` etc.).
7. ✅ **GAP-7** program-level end-to-end golden test + driver loop (`brain run --program`).

### Milestone C — Trust, context, scale — P2
8. ✅ **GAP-8** `replay` timeline + `brain why`.
9. ✅ **GAP-9** mission-context manifest with token budget.
10. **GAP-10** decide `query`/`directive` vs. documented block-and-redispatch.

---

## 4. Invariants every change must preserve

These are the project's crown jewels — **no gap fix may violate them**:

1. **Core stays deterministic.** Zero LLM calls, zero provider SDK, zero network in `internal/core`. All creative work happens in host workers. (Driver loop and MCP intent-tools are orchestration glue, not core authority.)
2. **Evidence gates completion.** No mission completes without a `specd verify` record (or the read-only proof path) bound to the task scope. Host telemetry stays `hostReported` and untrusted.
3. **Fail closed.** Unknown state, graph errors, conflicting leases, CAS/retry exhaustion → escalate, never guess.
4. **Human-only gates remain human-only.** High/critical mid-requirement gates can never be auto-cleared under any policy.
5. **One bounded action per step.** The driver may loop, but each `step` stays single-decision/single-action and replayable.

---

## 5. One-line summary

> The control plane is excellent; the **harness around it is half-built**. Brain can *schedule execution* but cannot *author the plan* (GAP-1), there is *no shipped loop* tying steps to worker spawns (GAP-2/3), and *no bootstrap* from a bare repo (GAP-6). Close Milestone A and "Pinky & The Brain" becomes a real beginning-to-delivery harness instead of a correct-but-manual set of primitives.
