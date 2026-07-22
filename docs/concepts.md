# specd — Concepts

> **Status:** Normative documentation for current `specd` behavior.

> **The agent reasons. The harness enforces.**

`specd` is a **spec-driven coding harness CLI** — Go, standard library only, zero runtime
dependencies, single static binary (`github.com/0xkhdr/specd`, Go 1.26+). It moves process
enforcement out of the LLM's non-deterministic context window into a **deterministic, local,
tool-gated pipeline**.

An AI agent is excellent at reasoning and terrible at reliably following a multi-step process
across a long context. So specd stops asking it to. The plan lives on disk as versioned
Markdown; state changes are gated by a local binary that an LLM cannot talk its way past.

## The foundational split

Two jobs, cleanly separated:

- **The agent reasons** — writes requirements, designs a solution, decomposes work into tasks,
  edits code, explains itself.
- **The harness enforces** — validates structure, computes the runnable task frontier, demands
  evidence before any status change, and requires a human to approve each phase boundary.

Nothing in specd's gates, DAG, or reports calls an LLM. They are pure functions of on-disk
state, which is what makes them trustworthy.

## The lifecycle

Work flows through six phases, each mapping to a spec status:

```
perceive → analyze → plan → execute → verify → reflect
requirements   design   tasks   executing   verifying   complete
```

(Plus `blocked` for a spec that has hit a gate it cannot pass.) The lifecycle is a
**ratchet**: `CanAdvanceStatus` permits forward moves only — you cannot walk a spec backward
to relax a gate it already passed. Each boundary requires an explicit human `specd approve`,
and approval only succeeds once the [validation gates](validation-gates.md) for that
transition pass.

```
requirements.md ──approve requirements──▶ design.md ──approve design──▶ tasks.md
      │                                                                     │
      └──────────── EARS gate, human approval ─────────────────────────────┘
                                                                            ▼
                                          evidence-gated execution (waves) ──▶ complete
```

## The six principles

1. **Determinism first.** No LLM in any gate, DAG, or report path. All are pure functions of
   on-disk `.specd/` state; reports are generated from `state.json` + task artifacts.
2. **Evidence integrity.** A task completes *only* against a passing verify record (exit 0
   pinned to a real git HEAD). **No bypass flag exists** — and none will be added.
3. **Planning ratchet.** Phases advance only on human `approve` once gates pass; status never
   moves backward.
4. **Structural invariants.** Atomic writes, compare-and-swap on the `state.json` revision, a
   reentrant per-spec lock, a byte-stable tasks parser, `go:embed` templates, zero runtime deps.
5. **Subtractive bias.** When unsure, cut or defer and record the decision (`specd decision`).
6. **Agent-agnostic.** Any command-running agent or MCP client drives specd; roles constrain
   *capability*, not identity.

## Evidence, not claims

The central rule: **trust is recorded, not assumed.** A task does not become "done" because
an agent says so. It becomes done when `specd verify` runs the task's verify command, the
command exits `0`, and that result is pinned to a resolvable git HEAD. That record is the only
thing the `evidence` gate accepts. Free-text "I completed this" claims are worthless to the
harness.

Read-only tasks (scouting, auditing) still carry a verify line — a trivially-passing one like
`printf ok` — so the same rule applies uniformly with no special case to exploit.

## Waves, not lines

Tasks form an acyclic **DAG**, not a flat checklist. The **frontier** is the set of tasks
whose dependencies are all resolved — a *wave* of concurrently runnable work. `specd next`
computes it. Agents work a wave, verify, and the next wave unlocks. This is what lets multiple
workers act in parallel without stepping on ordering constraints.

## Activity and readiness

A task carries two separate values. **Activity** is what the task is —
`draft | pending | in_progress | paused | blocked | failed | completed | cancelled | superseded`
— and the tasks.md marker stays its view. **Readiness** is whether it may start —
`ready | waiting_dependency | waiting_approval | waiting_clarification | waiting_schedule` — and
is projected, never stored twice: dependency waits are derived from the DAG plus task activity,
and only manual approval, clarification, schedule, pause, and block facts persist.

`pending` therefore means only "accepted, no active attempt, no terminal disposition"; it never
implies runnable. Only **pending plus ready** reaches the frontier. Every applicable wait is
reported, in the stable order dependency → approval → clarification → schedule, each with a
stable code, the ids it refers to, an owner, and a recovery. A cancelled or superseded
dependency keeps its descendants waiting (`dependency_terminal_unresolved`) until someone
records an explicit acceptance-coverage disposition. Any task still pending blocks parent
completion until it completes or takes an accepted terminal disposition.

## Approval request identity and staleness

An approval is never a bare flag. Every approval runs against an **immutable request**: a
record opened once under a stable id (the interactive `specd approve` uses `approve:<gate>`)
that pins the governing inputs of the decision — the **artifact digest**, the **state
revision**, the **transition-plan digest**, and the **config digest** — plus the requester and
an expiry. The request is never edited: `draft`, `requested`, `approved`, `rejected`,
`withdrawn`, `expired`, `revoked`, and `superseded` are separate appended transitions, and each
one inherits the pinned identities verbatim. Approval history is therefore append-only and
replayable, and the inputs an approval governed can never be rewritten after the fact.

**Staleness is a refusal, not a warning.** When any pinned input drifts from current, the
request no longer describes what is being approved, so approving it is refused: the only legal
continuation is a **new or superseding request**. The same holds past the expiry. `specd status`
projects the current transition of every request — id, state, entity, pins, expiry — in both the
human and `--json` renderings, and names the drift, so a request that would be refused is
visible before the approval is attempted. An already-approved request whose inputs drifted keeps
its approval for what it pinned and is reported as drifted; it does not extend to the new inputs.

## Impact preview before repair

Repair — `undo`, `reopen` — never mutates before it has shown what it would touch. A **preview**
is a pure function of the caller-resolved snapshot: it classifies every reachable artifact,
approval, task, criterion, review, mission, submission, release, deployment, archive, and
cross-spec link as exactly one of *current*, *stale*, *reopened*, *retained*, *superseded*,
*cancelled*, or *forbidden*, each with a reason. Nothing is discovered or loaded during planning,
so the same inputs always yield the same ordering and the same **impact digest**.

**Immutable consumption forbids in-place repair.** When a release, deployment, archive, or
externally accepted submission consumed the target, the preview names the consuming record and
routes to a **linked successor** instead of proposing a rewrite. History is never deleted or
decremented; an undo appends a compensation event and projects the prior effective state at a
*higher* revision.

**Preview equals commit.** Commit re-reads the snapshot and refuses when either the state
revision or the recomputed impact digest moved, so a repair that raced another mutation is
rejected with a fresh-preview route rather than applied to state it never described.

## Execution modes

- **Base mode.** A human (or a single agent) drives the loop by hand: `new → approve →
  next → verify → task complete → approve → submit`. The harness gates every step.
- **Orchestrated mode (opt-in).** The deterministic `specd brain` controller drives the
  wave loop itself using leases and an append-only decision ledger — still with **no LLM in
  the decision path**. It dispatches missions to role-scoped workers ("Pinky") and collects
  their evidence. See [agent-integration.md](agent-integration.md).

## Where things live

```
.specd/specs/<slug>/
├── requirements.md   # EARS requirements (perceive)
├── design.md         # design sections (analyze)
├── tasks.md          # the task DAG, byte-stable (plan)
├── state.json        # machine truth: phase, task status, records, evidence
└── .lock             # reentrant per-spec lock
.specd/roles/*.md     # role prompts (scout, craftsman, validator, auditor)
.specd/steering/*.md  # durable steering constitution
AGENTS.md             # host integration guide, written by `specd init`
```

`tasks.md` markers and `state.json` are two views of task status; the `sync` gate fails closed
if they disagree, so a hand-edited marker can never fake completion.

---

**Next:** [user-guide.md](user-guide.md) to run the lifecycle · [command-reference.md](command-reference.md)
for the full CLI · [validation-gates.md](validation-gates.md) for the enforcement details.
