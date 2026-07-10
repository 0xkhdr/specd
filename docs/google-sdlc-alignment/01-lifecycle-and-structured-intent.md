# Domain 01 — Lifecycle and Structured Intent

## Purpose

Define the shared lifecycle contract between `specd` and the Google SDLC paper as represented
in `sdlc-with-vibe-coding.md`, then identify the changes needed for `specd` to prevent planning
defects before agents begin implementation. This domain covers requirements, design, task
decomposition, phase compression, human judgment, and the paper's “80% problem.” It does not
assume that faster phase transitions are better; it asks how fast feedback can coexist with a
forward-only, evidence-preserving production workflow.

Paper claims in this document come only from `sdlc-with-vibe-coding.md`. That comparison file
preserves extracted line references from the paper because the original PDF was unavailable in
the workspace. Statements marked **Inference** are recommendations derived from those claims,
not claims made verbatim by the paper.

## Paper position

The comparison document attributes five relevant positions to the paper:

1. Production “agentic engineering” is distinguished from vibe coding by structured intent,
   tests/evals, guardrails, and human oversight (extracted paper lines 121–140 and 510–514).
2. AI compresses and blurs parts of the SDLC, especially requirements-to-prototype and
   implementation-to-review feedback, but the compression is uneven (lines 190–204).
3. AI can help refine requirements, expose edge cases, and accelerate prototype feedback
   (lines 204 and 472).
4. Architecture remains a human-centric decision boundary even when agents generate design
   options or implementation detail (lines 196, 287–293, and 514).
5. The easy first 80% of generated implementation hides the expensive final 20%: integration
   points, edge cases, error handling, and subtle correctness (lines 352–360).

**Inference:** Compatibility therefore means preserving explicit human responsibility for intent
and architecture while using the harness to make early feedback cheap and to stop incomplete
planning from becoming apparently complete code. A phase may contain short generate/check/revise
loops, but phase compression must never become artifact or approval bypass.

## Current specd handling

`specd` already implements a strong lifecycle skeleton:

- `internal/core/phases.go` defines the forward sequence
  `requirements → design → tasks → executing → verifying → complete`, maps it to
  `perceive → analyze → plan → execute → verify → reflect`, and rejects backward movement.
- `internal/cmd/lifecycle.go` scaffolds `requirements.md`, `design.md`, `tasks.md`, per-spec
  `memory.md`, and `state.json`; `runApprove` runs deterministic gates and records an approval
  through compare-and-swap state mutation.
- `internal/core/gates/ears.go` rejects the untouched requirements stub and warns when a
  requirement bullet lacks “shall.” This is an EARS-shape heuristic, not a semantic EARS parser.
- `internal/core/gates/approval.go` rejects an untouched or empty-section design when design
  approval is requested. It also prevents task progress before requirements and design approvals.
- `internal/core/tasksparser.go` gives every task the fields `id`, `role`, `files`,
  `depends-on`, `verify`, and `acceptance` while preserving byte-stable Markdown round trips.
- `internal/core/gates/core.go`, `internal/core/dag.go`, and `internal/core/frontier.go` validate
  task structure and compute dependency-safe waves without an LLM in the decision path.
- `internal/core/evidence.go`, `internal/core/task_complete.go`, and
  `internal/core/verify/exec.go` require a passing verify result pinned to a resolvable git HEAD
  before task completion.
- `internal/core/gates/criteria.go` can address acceptance criteria and
  `internal/core/gates/review.go` can require a current-HEAD approval report, but both policies
  are opt-in (`docs/validation-gates.md`).
- `specd midreq` and `specd decision` append stamped records without destroying history
  (`internal/cmd/lifecycle.go`).
- The host-facing loop and role boundaries are scaffolded from
  `internal/core/embed_templates/AGENTS.md`,
  `internal/core/embed_templates/steering/workflow.md`, and
  `internal/core/embed_templates/roles/*.md`.

This is a credible harness implementation of “structure scales, vibes don't.” Its present
strength is enforcing that artifacts and evidence exist. Its weaker area is proving that the
artifacts cover the right intent before the agent spends implementation effort.

## Common contract/fields

| Contract | Paper concept | Current `specd` fields/artifact | Hardened common fields |
|---|---|---|---|
| Requirement | Refined, testable intent; edge-case discovery | EARS-shaped bullets in `requirements.md`; approval record | Stable requirement ID; EARS trigger/state/condition and response; acceptance criteria; edge cases/failure behavior; source/owner; priority; risk; explicit exclusions; approval revision and artifact digest |
| Design | Architecture is a human judgment boundary | `design.md` sections for modules, on-disk contracts, invariants | Requirement references; system/module boundaries; interfaces and data contracts; alternatives and rationale; preserved invariants; integration/failure modes; security/operability impact; migration/rollback; named human decision and digest |
| Task | Atomic agent work under a harness | `id`, `role`, `files`, `depends-on`, `verify`, `acceptance` | Existing fields plus requirement/design references; work kind; risk tier; assumptions; required context; negative/edge checks; evidence classes; estimated scope; explicit non-goals |
| Phase | Compressed feedback with retained oversight | `status`, `phase`, `revision`, approval records | Entry/exit criteria; responsible actor class; artifact digest; stale/valid status; allowed feedback loop; next legal actions; exception/change record |
| Evidence | Tests and review prevent “almost done” | Verify command, exit code, git HEAD; optional criterion/review records | Requirement coverage; output/test evidence; integration/edge evidence; review decision; context/plan digest; freshness; environment identity; risk-dependent evidence policy |
| Change | Feedback continues after planning | Free-text `midreq`/`decision` record with scope | Change ID; affected requirement/design/task IDs; rationale; artifact before/after digests; invalidated approvals/evidence; required re-checks; human disposition |

The hardened fields should remain plain Markdown plus validated JSON state. They are contracts
for deterministic parsing and reporting, not invitations to put an LLM inside a gate.

## Gaps and failure modes

### Planning quality is weakly enforced

`internal/core/gates/ears.go` rejects the untouched stub but only warns on bullets without the
word “shall.” It does not require stable IDs, parse EARS clauses, detect duplicate/conflicting
requirements, or require edge and failure behavior. A fluent but incomplete requirements file
can pass. The design gate in `internal/core/gates/approval.go` proves that sections contain text,
not that the design traces to requirements, evaluates alternatives, identifies integration
boundaries, or records the human architecture decision.

### Traceability ends at prose

`internal/core/tasksparser.go` has no requirement or design-reference field. The harness cannot
deterministically answer whether every requirement has design coverage, whether every design
component has an implementation task, or whether a task is justified by approved intent.
Acceptance criteria exist per task, but no mandatory relation connects them to requirement
criteria. This permits internally consistent task DAGs that implement the wrong or incomplete
thing.

### Documented role and scope guarantees exceed current gates

`docs/validation-gates.md` says unknown roles are rejected, but `roles()` in
`internal/core/gates/core.go` only rejects an empty role. Likewise, the `files` gate only checks
that the field is non-empty; it does not enforce actual writes against that declaration. A typoed
role or out-of-scope edit can therefore depend on agent convention instead of deterministic
enforcement.

### Mid-stream change does not invalidate downstream truth

`midreq` and `decision` preserve audit history, which is valuable, but they do not calculate
impact or stale the design approval, task plan, criterion evidence, or review evidence. A major
requirement change can coexist with earlier approvals that still appear valid. Moving the status
backward would violate the ratchet; leaving downstream records current is also unsafe.

### The final 20% can hide behind a shallow verify line

The verify gate requires a non-empty command, and passing evidence is rigorous about exit status
and git identity. It does not grade whether the command exercises behavior, integration, error
handling, or edge cases. A write task can use `printf ok`, a formatter-only check, or a narrow
unit test and still satisfy the base evidence contract. Optional criterion and review gates reduce
this risk only when production config enables them.

### Agent guidance is execution-heavy

The generated `AGENTS.md` gives a clear task loop (`status → context → work → verify → check`),
but it does not give phase-specific authoring instructions for an empty requirements, design, or
tasks phase. `specd context` is unavailable in the initial perceive phase according to
`internal/core/commands.go`. A general-purpose coding agent can know how to execute a task yet
still lack a deterministic “what must I produce next?” path during planning.

### Phase compression has no explicit safe-loop model

The forward-only lifecycle is safe, but prototypes, spikes, and requirement feedback have no
first-class planning-loop representation. Teams may either bypass `specd` for exploratory work or
prematurely approve an artifact to reach execution tools. Both outcomes weaken the paper's goal:
use fast feedback to improve intent before scaling implementation.

## Target best-practice workflow

1. **Bootstrap and orient.** The agent verifies the binary/config handshake, runs status, and
   receives machine-readable legal next actions for the current phase. Planning actions are
   available without pretending an execution task exists.
2. **Refine requirements.** The agent helps the human convert intent into stable, EARS-shaped
   requirements with acceptance criteria, exclusions, edge cases, failure behavior, and risk.
   A bounded spike may produce evidence, but cannot itself approve intent.
3. **Approve intent.** Deterministic syntax/coverage gates pass; a human owns the approval.
   The approval pins the requirements digest.
4. **Design with human judgment.** The agent may propose alternatives and analyze trade-offs.
   The selected architecture records requirement coverage, interfaces, invariants, failure modes,
   operations, migration, and rollback. A human decision pins the design digest.
5. **Plan prevention-first.** Tasks trace to requirements and design, form an acyclic frontier,
   declare scope and role, and carry risk-proportionate verification. Planning checks look for
   uncovered criteria, integration boundaries, negative cases, and oversized tasks before any
   implementation dispatch.
6. **Execute bounded waves.** The agent selects only frontier work, loads task-specific context,
   honors role and file authority, and records deviations before expanding scope.
7. **Verify the difficult 20%.** Completion requires deterministic task evidence and, according
   to risk, criterion, integration, security, and current-HEAD review evidence. No qualitative
   evaluator may manufacture a deterministic pass.
8. **Handle change as a new ratchet segment.** A mid-stream requirement creates an amendment with
   affected IDs and digests. Downstream records become stale; the status need not move backward,
   but dispatch pauses until amended design/tasks are re-approved.
9. **Reflect.** Only verified and reviewed lessons are candidates for durable memory. Reports show
   coverage, stale records, changes, and escaped defects, not merely task counts.

**Inference:** This workflow realizes phase compression inside controlled feedback loops while
keeping the harness's monotonic audit history and the paper's human architecture boundary.

## Recommended action plan

| Priority | Action | Artifact/code surface | Deterministic acceptance check |
|---|---|---|---|
| P0 | Add stable requirement parsing and coverage fields | Requirements format/scaffold; `internal/core/gates/ears.go`; a new pure requirements parser; `docs/open-spec-format.md` | Fixture tests reject missing/duplicate IDs and malformed EARS clauses; parsing the same bytes twice yields identical records; every declared criterion has a stable address |
| P0 | Add task traceability and risk | `internal/core/tasksparser.go`; task scaffold; task gates; byte-stable rewrite tests | A plan fails when a task lacks required requirement/design refs, a referenced ID is absent, or a requirement has no implementing/explicitly-deferred task; parse/write remains byte-identical |
| P0 | Strengthen the design contract without an LLM | Design scaffold; pure design-section/trace gate in `internal/core/gates`; approval records | Design approval fails on missing required sections, unknown requirement refs, or absent decision metadata; the approval record stores the exact artifact digest |
| P0 | Add amendment invalidation | `state.json` schema/records; `midreq`; approval/evidence freshness gates | Given an approved plan, a change affecting R2 deterministically marks dependent design/task approvals stale and blocks `next` until re-approved; unrelated records remain current; status never moves backward |
| P0 | Publish phase-native next actions | Command metadata, `status --json` or a read-only `guide --json`; scaffolded `AGENTS.md` | Golden tests for every status return only legal commands, required artifact, gate blockers, and human-only actions; an agent in `requirements` is never instructed to call task context/verify |
| P0 | Enforce known roles and verify quality baseline | `internal/core/gates/core.go`; role registry; verify gate/profile config | Unknown roles fail; write tasks using a configured trivial-command denylist fail; read-only trivial verification remains explicitly allowed by role |
| P1 | Add a production planning profile | `project.yml`; criteria/review/security policy; gate registry | With `profile: production`, criterion coverage and current-HEAD review are armed by default; config digest changes when the profile changes; default/backward-compatible profile is explicit |
| P1 | Add risk-proportionate evidence rules | Task schema and pure evidence policy | High-risk or integration tasks fail planning without negative/error-path and integration evidence requirements; low-risk documentation tasks do not inherit irrelevant checks |
| P1 | Represent bounded prototypes/spikes | Planning artifact and `spike`/decision records; no completion bypass | A spike can attach output to a requirement/design decision, but cannot satisfy task completion evidence or approve its own architecture; promotion into the plan requires human approval |
| P1 | Add lifecycle production regressions | `scripts/` black-box fixture harness outside `reference/` | Fresh binary runs happy path, shallow-plan rejection, amendment invalidation, stale review, and agent-wrong-command scenarios in isolated git repos with fixed exit codes |
| P2 | Add coverage and escape metrics | Deterministic report fields | Reports derive requirement→design→task→evidence coverage and post-completion defect links entirely from on-disk records, with stable JSON ordering |
| P2 | Add portfolio-level change impact | Cross-spec program graph and reports | A changed shared requirement identifies affected dependent specs and blocks only their unsafe execution transitions; graph output is deterministic and cycle-safe |

## Production validation scenarios

1. **Greenfield service:** create a spec, author complete intent/design/tasks, drive every phase,
   restart the process between steps, and finish from a release binary in a clean git repository.
   Assert stable next actions, CAS revisions, approval digests, frontier order, and evidence.
2. **Ambiguous happy-path requirement:** omit failure behavior and an acceptance criterion. The
   planning gate must fail before design/tasks, with a finding that names the exact requirement.
3. **Architecture without human disposition:** provide a technically complete design but no
   selected alternative/owner. The harness must permit agent suggestions but refuse approval.
4. **Uncovered integration edge:** define an external boundary in design and omit its error-path
   task/evidence. Production-profile planning must fail before execution.
5. **Shallow verification:** give a write task `printf ok` or a formatting-only command. The
   production policy must reject it while allowing an explicitly read-only task's trivial check.
6. **Mid-stream requirement change:** after several tasks pass, amend a covered requirement.
   Dependent approvals/evidence become stale, dispatch pauses, unaffected history remains intact,
   and the ratchet never rewrites prior state.
7. **Misguided agent:** ask an agent to verify in requirements phase, approve its own human gate,
   use an unknown role, or edit undeclared files. Each action must fail closed or appear as a
   deterministic policy violation rather than rely on prompt obedience.
8. **Crash/concurrency:** kill a process during approval and race two workers on one spec. Atomic
   writes, locks, and CAS must preserve a valid state with at most one successful transition.

## Context-safety considerations

- Planning quality must not be achieved by injecting the entire paper, repository, or all prior
  decisions into every model call. Give the agent the current artifact, relevant IDs, gate
  findings, and phase-specific instructions.
- Requirement and design summaries are not authoritative substitutes for source artifacts. Pin
  references and digests so a stale summary cannot guide current work.
- Edge-case prompts should be generated from declared boundaries/risk metadata or requested as
  human reasoning; they must not become an LLM-based pass/fail gate.
- Next-action output should separate machine-allowed actions from human-only approvals. This keeps
  the model from interpreting descriptive command knowledge as authority.
- Amendment impact should select affected requirement/design/task slices. Do not reload the whole
  project history simply because one requirement changed.
- Store concise structured rationale and evidence references; avoid copying chat transcripts into
  `state.json` or steering memory.

## Non-goals/risks

- Do not put an LLM in EARS, design, DAG, approval, coverage, or evidence gates. Human judgment can
  approve architecture; the harness deterministically proves that the approval and required
  structure exist.
- Do not turn `specd` into a deployment platform or autonomous-agent runtime. This domain ends at
  a trustworthy software-delivery lifecycle and can expose adapters to later domains.
- Do not treat more fields as automatically better. Excess ceremony can push users back to vibe
  coding; production profiles and risk tiers should add only decision-relevant fields.
- Do not erase or rewrite history to model feedback loops. Staleness plus a new amendment segment
  is safer than backward state mutation.
- Schema evolution risks breaking existing specs and integrations. Migrations, explicit versions,
  stable Markdown round trips, and docs/CLI parity are mandatory.
- Verify-quality heuristics can reject valid specialized commands. Prefer explicit profiles,
  role-aware allowlists, and actionable findings over an unconfigurable global ban.
