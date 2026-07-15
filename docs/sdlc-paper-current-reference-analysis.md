# specd, the New SDLC paper, and the frozen reference: complete analysis

Date: 2026-07-16  
Repository commit inspected: `69fb28585ace49d9e04ecd44e5eea48174703bb4`  
Scope: current source, tests, top-level SDLC specs, alignment docs, paper text, and read-only `reference/` v0.2.0 museum.

## Executive conclusion

`specd` already implements most mechanical substrate described by *The New SDLC With Vibe
Coding*: structured intent, deterministic gates, bounded context, portable-skill parsing,
evidence classes, trajectory records, authority packets, scope comparison, orchestration,
routing, telemetry, delivery ledgers, maintenance records, MCP, A2A mapping, and adapter
contracts. Current architecture is a stronger production foundation than frozen reference.

Main weakness changed. It is no longer “missing every paper domain.” It is now **contract
coherence and productization**:

1. lifecycle implementation permits phase skipping while docs promise exact-step ratchet;
2. public approval examples name current artifacts while implementation expects target status;
3. tool metadata labels several mutating commands as read-only;
4. agent loop says `verify` marks completion, but code requires separate `task complete`, which
   handshake excludes;
5. current skill engine is strong, but fresh projects receive no bundled stage skills;
6. fresh artifact stubs teach legacy/minimal shape, not current production contracts;
7. SDLC alignment docs describe pre-implementation gaps as current facts;
8. Domain 08/09 W0 task rows disagree with completed program rollup.

Therefore next version should not add more breadth first. It should make one golden workflow
truthful, executable, progressively disclosed, and production-profile complete from fresh init to
observed deployment. **Current core + selected reference UX patterns + paper principles** is best
combination.

## Method and evidence standard

Analysis used three evidence classes:

- **Implemented:** source path exists and behavior is pinned by tests or runnable CLI output.
- **Declared:** requirements/design/tasks or docs claim behavior, without treating claim as proof.
- **Observed:** command was run during this analysis.

Commands run successfully:

- `go test ./... -race -count=1` — 828 tests across 15 packages;
- `go test ./... -count=2`;
- `go vet ./...`;
- `gofmt -l .` — empty;
- `scripts/test-lint.sh`, `scripts/docs-lint.sh`, `scripts/regress-lint.sh`;
- `scripts/regress-domains.sh`, `scripts/regress-all.sh`;
- fresh binary build, `init`, `new`, `check`, `status --guide`, `handshake bootstrap`.

Passing suites prove encoded invariants. They do not prove unencoded workflow claims; several
findings below exist precisely because tests pin local components while cross-surface semantics
drift.

## Paper intent: durable concepts, not literal feature checklist

Paper’s durable boundary is structure, verification, and human judgment around model output
(`sdlc-paper.md:121-140`). It does not prescribe a specific CLI or file schema.

### Core principles

1. **Intent replaces syntax as primary interface.** Formal specs, architecture, and memory
   distinguish agentic engineering from casual prompting (`sdlc-paper.md:123-134`).
2. **Tests and evals are complementary.** Tests prove deterministic behavior; output and
   trajectory evals prove non-deterministic quality and conduct (`sdlc-paper.md:140`,
   `sdlc-paper.md:220-226`).
3. **Context engineering is primary leverage.** Instructions, knowledge, memory, examples,
   tools, and guardrails form six context lanes (`sdlc-paper.md:142-153`).
4. **Static/dynamic boundary is architecture.** Always-loaded rules must stay small; dynamic
   skills and references load only when relevant (`sdlc-paper.md:155-167`).
5. **Developer builds factory, not each widget.** Specs, agents, tests, feedback loops, and
   guardrails become development system (`sdlc-paper.md:242-258`).
6. **Harness is product surface.** Instructions, tools, sandbox, orchestration, hooks, and
   observability surround model (`sdlc-paper.md:260-281`).
7. **Human judgment stays at architecture and acceptance.** Implementation compresses faster
   than requirements, architecture, and verification (`sdlc-paper.md:190-218`).
8. **Conductor and orchestrator are both valid.** Task/risk determines hands-on versus async
   delegation; autonomy is not universal goal (`sdlc-paper.md:325-349`).
9. **Production includes deploy, observe, rollback, maintain.** Merge is not lifecycle end
   (`sdlc-paper.md:228-234`).
10. **Economics matter.** Bounded context, progressive skills, routing, and first-pass quality
    reduce token and maintenance OpEx (`sdlc-paper.md:418-458`).
11. **Open standards preserve choice.** MCP and A2A are recommended connective tissue
    (`sdlc-paper.md:494-498`).
12. **Structure scales; vibes do not.** Human direction, guardrails, evals, and review remain
    production requirements (`sdlc-paper.md:508-514`).

### What paper does not require

- LLM calls inside gates or controller.
- Provider SDKs inside trusted core.
- Automatic architecture approval.
- Maximum autonomy for every task.
- One universal model, host, deployment platform, or data backend.
- Storing hidden chain-of-thought. Observable tool/event trajectory is enough.

`specd`’s deterministic, offline, adapter-oriented interpretation is therefore sound.

## Current specd: foundational model

### Architecture

Runtime flow is `main.go → internal/cli → internal/cmd`. Trusted domain logic stays in
`internal/core`; pure gates stay in `internal/core/gates`; context selection stays in
`internal/context`; deterministic controller stays in `internal/orchestration`; external-system
contracts stay in `internal/adapter`; MCP is transport surface, not decision-maker.

Strong structural properties:

- Go standard library only; single binary; no `go.sum`.
- strict state decoder, schema migration, atomic writes, CAS revision, per-spec reentrant lock;
- byte-stable task parsing and status rewrite;
- no-bypass verify record pinned to resolvable git HEAD (`internal/core/task_complete.go:19-41`);
- optional quality contract adds fresh test/output-eval/trajectory-eval/review evidence
  (`internal/core/task_complete.go:44-64`);
- command palette drives help/MCP/guidance metadata;
- imported/non-deterministic outcomes satisfy gates only after local schema, identity, digest,
  freshness, and policy validation;
- adapter import guard keeps network/provider code out of trusted core.

### Current lifecycle

Declared lifecycle:

```text
requirements → design → tasks → executing → verifying → complete
 perceive       analyze    plan      execute       verify       reflect
```

Intended split:

- human/agent authors semantic artifacts;
- harness validates shape, trace, state, scope, and evidence;
- human owns phase approval;
- agents may execute only role/task authority;
- completed specs remain immutable; maintenance uses linked successors.

### Current command and gate surface

Observed `help --json`: 33 top-level commands; only `triage` deferred. Core registry has 20
ordered gates: task IDs, dependencies, DAG, roles, files, verify, evidence, context budget, EARS,
approval, sync, design, criteria, review, task trace, coverage, evidence policy, intake,
governance, and memory lint. Security is composed separately according to profile.

This breadth now covers paper’s coding-SDLC substrate. Production value depends on composing it
correctly, not increasing command count.

## Ten SDLC domains: present state

| Domain | Implemented foundation | Current maturity limit |
|---|---|---|
| 01 Lifecycle / intent | stable requirement IDs/criteria, design refs/digests, task trace/risk, coverage, amendments, staleness, profiles, spikes | exact transition rule and public examples contradict implementation; fresh stubs under-teach production contract |
| 02 Context / knowledge / skills | typed Manifest V2, canonical paths/digests, required lanes, truthful budget, steering/memory/examples, portable skill parser, receipts | no bundled stage skill pack; operators must invent first skill; capability inputs inherit incorrect tool-effect metadata |
| 03 Agent/tool guidance | bootstrap/guide/dispatch contracts, digests, active-spec resolution, doctor, CLI/MCP projection | base-loop completion route inconsistent; some mutating routes are advertised read-only; `agents doctor --json` emits `null` on clean project rather than explicit typed empty result |
| 04 Verification / evals | four evidence classes, freshness, quality policy, import, trajectory, dataset/rubric digests, review/flywheel records | end-user authoring/execution path is adapter-heavy; no shipped eval-author skill or reference local adapter in fresh project |
| 05 Orchestration / routing | mission, claim, lease, heartbeat, report, cancellation, recovery, routing, brakes, A2A mapping | controller contracts are stronger than host onboarding; prove complete real-host run across supported hosts, not only fake/conformance fixtures |
| 06 Security / governance | production profile, authority packets, harness-derived diff scope, context/change scans, sandbox requirement, dependency/dangerous-change checks, exceptions | default remains prototype/opt-in; cross-platform isolation depends on external capability; all mutable entry points need effect/authority audit |
| 07 Observability / economics | run IDs/attempts, telemetry provenance, token/cost fields, brakes, privacy/cardinality, spans, trace/OTel export, rollups | trustworthy provider measurement remains external; unknown telemetry must stay visibly unknown; operator UX is reports rather than coherent run explorer |
| 08 Delivery / assurance | release/deployment ledgers, environment state machine, canary/health/rollback, CI binding, attestation, incident seed | actual deployment/health systems are adapters; project ships contracts, not production-grade reference integrations; W0 task markers drift |
| 09 Maintenance / operating model | typed successor links/intake, decisions/exceptions, memory aging, drift, recurring checks, incidents, portfolio, archive, policy/templates | schedules remain correctly external; documentation still says draft in places; W0 task markers drift |
| 10 Boundaries / interop | versioned envelopes, identity, classification, runner, capability inspection, offline continuity, conformance, MCP/A2A/OTel mapping | ecosystem needs maintained reference adapters, compatibility matrix, signing/trust distribution, and version migration proof over releases |

## Critical current findings

### F1 — “strict ratchet” permits skipping and same-status approval

`CanAdvanceStatus` accepts any target whose index is greater than or equal to current index
(`internal/core/phases.go:71-87`). Docs promise “You cannot skip ahead”
(`docs/user-guide.md:82-86`), and managed workflow says “No backward or skipping transitions.”

Impact: a caller can request a later valid status directly. Gates are target-sensitive, but no
single invariant enforces `target == NextStatus(current)`. Component gates are not substitute for
transition legality.

Fix:

```text
ordinary approval: target must equal NextStatus(current)
idempotent read/retry: explicit separate behavior, no new approval record
orchestrated mode: separate mode transition
blocked recovery: explicit governed path
```

Add table-driven test for every `(from,to)` pair and black-box test proving all skips fail before
any artifact/gate evaluation or mutation.

### F2 — approval documentation and canonical examples are wrong

Implementation parses `<gate>` as **target status** (`internal/cmd/lifecycle.go:60-74`). Machine
guide correctly computes next target with `NextStatus` (`internal/core/commands.go:639-648`). But
canonical metadata advertises `approve payments requirements` as first example
(`internal/core/commands.go:216-223`), and user guide repeats it
(`docs/user-guide.md:104-129`).

Correct target-status sequence should be:

```text
approve <slug> design
approve <slug> tasks
approve <slug> executing
approve <slug> complete
```

Better API: remove caller-supplied lifecycle target entirely: `specd approve <slug>` advances
exactly one status. Keep explicit nouns only for non-lifecycle actions such as `orchestrated` and
governed exceptions. This matches reference’s simpler UX while preserving current gates.

### F3 — tool contracts infer side effects from a fragile allowlist

`ManifestToolContracts` marks mutable only when `RequiresTask` or name is verify/submit/review
(`internal/core/manifest_tools.go:19-35`). As observed in handshake, `new`, `archive`, `eval`,
`recurring`, `spike`, `link`, and `unlink` are labelled `capability: read, mutable: false`, although
some or all subcommands write state/files. Forbidden list is another hand-maintained name switch
(`internal/core/manifest_tools.go:71-77`).

Impact: host policy, skill capability selection, and authority reasoning consume false effect
metadata. Mixed read/write subcommands cannot be represented by one command-level boolean.

Fix: canonical command metadata needs per-operation records:

```text
operation, actor, effect(read|workspace-write|state-write|external),
phase, task-required, authority-required, scope-source, network-class
```

Derive help, MCP tools, handshake, skill capabilities, and authorization from those records. Add
parity test that every registered handler operation declares effect and every mutating fixture is
never projected as read.

### F4 — generated base agent loop cannot complete a task coherently

Managed `AGENTS.md` says `verify` “marks a task complete”
(`internal/core/embed_templates/AGENTS.md:21-22`). In code, verify only records evidence; explicit
`task complete` performs completion and marker/state CAS
(`internal/cmd/lifecycle.go:175-203`). Yet `task` is excluded from handshake tool contract
(`internal/core/manifest_tools.go:71-74`).

Fix choices:

- preferred: expose narrow `complete-task` operation to assigned agent/worker; it can only consume
  already-valid evidence and current authority;
- alternative: make successful verify atomically complete only when all quality/scope/security
  conditions already pass, clearly documenting transaction semantics.

Do not expose human override or arbitrary task-state mutation through same route.

### F5 — skill platform exists; skill product does not

Current parser validates version, trigger, phases, roles, capabilities, references, provenance,
required policy, positive budget, and mandatory Instructions/Examples/Checks sections
(`internal/context/skills.go:42-163`). Selection supports progressive phase/role/capability loading.

But `init` writes only `.specd/skills/README.md` (`internal/core/scaffold.go:32-43`). Fresh project
contains zero usable skills. Paper explicitly identifies progressive Agent Skills as dynamic
context mechanism (`sdlc-paper.md:165-176`).

Fix: ship a small, versioned, current-schema core pack. Skills remain advisory and digest-pinned;
authority remains in tool contracts/gates.

### F6 — current stubs do not teach current production schema

Fresh requirements stub is one legacy bullet; design has three empty generic sections; tasks has
six columns and a trivial scout placeholder (`internal/cmd/lifecycle.go:529-548`). Production
parsers/gates support much richer contracts: requirement/criterion IDs and metadata, design
decision fields, task refs/risk/context/evidence/checks/capabilities.

Impact: first user experience starts with formats that cannot express strongest paper-aligned
behavior. Production profile turns into error-driven reverse engineering.

Fix: scaffold profile-aware canonical examples with comments, not a fake runnable placeholder.
Default profile may relax enforcement, but template should teach production shape once.

### F7 — current documentation mixes historical diagnosis with current truth

Examples:

- `docs/agent-integration.md:32-35` says role authority and diff scope have not landed, although
  Domain 06 code now contains authority/scope implementation.
- `docs/google-sdlc-alignment/*` still describes missing context, eval, orchestration, security,
  telemetry, delivery, and maintenance features that later specs implemented.
- `sdlc-with-vibe-coding.md` is dated July 10 and scores old code, but lacks historical/superseded
  banner.

Fix: classify docs as `normative`, `current assessment`, `historical baseline`, or `proposal`.
Add `as_of_commit`, `status`, `superseded_by`, and owner to assessment docs. Generate current
capability matrix from tests/command metadata where possible.

### F8 — program rollup and domain task truth disagree

`specs/progress.md` marks Domain 08 W0 and Domain 09 W0 complete, while their T01-T05 rows remain
unmarked (`specs/08-.../tasks.md:14-22`, `specs/09-.../tasks.md:15-23`). Regression script still
passes because its ordering check does not assert every rollup completion against every domain
row.

Fix: one deterministic rollup projection from domain task rows, or lint equality both ways. Never
maintain duplicated completion truth manually.

### F9 — clean doctor JSON is ambiguous

Observed fresh `specd agents doctor --json` emitted `null`. Machine consumers need typed empty
success (`{"findings":[],"healthy":true,...}`), version, and next action. `null` cannot distinguish
clean, unsupported, or absent payload without out-of-band exit interpretation.

## Frozen reference: what it represents

Reference is older, broader v0.2.0 product with 32 commands, 207 non-test Go files, 255 test files,
extra packages for worker execution, runner, schema, packs, observability, host integration, and
test harness. Current tree has 186 non-test Go files and 228 tests, but more focused packages for
adapter, orchestration, context, and pure gates.

Reference is valuable as pattern library, not implementation donor. It contains good UX ideas and
contradictory/unsafe contracts.

## Reference patterns: adopt, rewrite, reject

| Reference pattern | Judgment | Reason / current adaptation |
|---|---|---|
| bundled progressive skill pack | **Adopt, rewrite** | strongest lost pattern; reference scaffolds 12 stage skills (`reference/internal/core/scaffold.go:54-66`); rewrite metadata/content to current parser and commands |
| rich requirements/design/task stubs | **Adopt, rewrite** | design prompts architecture/data/errors/verification/risks (`reference/.../specStubs/design.md`); translate task list format into current byte-stable table + trace/risk/evidence columns |
| foundations skill indexing stage skills | **Adopt, shrink** | excellent progressive disclosure; keep always-on constitution tiny; remove stale commands/status/schema |
| dedicated requirements/design/tasks/execute/review skills | **Adopt, rewrite** | procedural knowledge belongs in dynamic context; generate content from canonical command/field metadata where feasible |
| steering bootstrap skill | **Adopt** | agent inspects manifests/tree/README/CI and authors product/tech/structure; preserves foundational split |
| reviewer/brain/pinky role contracts | **Adopt selected content** | strong adversarial review and worker boundaries; merge into current auditor/Pinky guidance without adding prose authority |
| explicit scaffold manifest | **Adopt** | one inspectable asset inventory with create/managed/user-owned policy beats scattered writes |
| host detection + project-scoped MCP snippets | **Adopt in integration adapter** | improves onboarding; preserve explicit consent, project scope, safe merge, no global mutation; keep host/network logic outside core |
| JSON schema export and migration UX | **Adopt concepts** | machine inspectability and N-1 migration valuable; current strict Go validation remains truth |
| spec/harness packs | **Adopt later with quarantine** | useful team asset; require digest/signature/provenance, file-only content, review/enable step; no remote fetch inside core gates |
| dashboard/live observe | **Optional adapter** | useful operator projection; keep read-only and external so core stays local/short-lived |
| six authored spec files (`decisions`, `mid-requirements`, local memory) | **Selective** | current typed append-only records avoid duplicate truth; expose rendered Markdown views instead of adding parallel mutable ledgers |
| broad worker/runner/provider behavior inside product | **Do not restore to core** | current transport-neutral mission/adapter boundary is safer and more portable |
| Redis/Postgres/build-tag state backends | **Reject for trusted core** | violates subtractive bias and single deterministic file-backed truth unless real scale evidence demands separate service |
| command sprawl, aliases, wrappers | **Reject by default** | raises palette/context/compatibility cost; add only measurable workflow value |
| manual/unverified completion escape | **Reject** | reference execute skill explicitly permits `--unverified` (`reference/.../specd-execute/SKILL.md:47-52`); contradicts current no-bypass invariant |
| `verify: N/A` for read-only work | **Reject** | current explicit trivial verify gives uniform evidence semantics and is easier to audit |
| host-reported changed files as proof | **Reject** | current harness-derived git diff is stronger; host report stays telemetry only |

## Reference skill content audit

Reference ships 12 skills:

| Skill | Useful content | Required correction |
|---|---|---|
| `specd-foundations` | constitution, file map, stage index, progressive disclosure | stale modes, commands, exit codes, six-artifact model; convert YAML front matter to current `specd-skill` metadata |
| `specd-steering` | evidence-based repo inspection; author product/structure/tech; set real verify command | current config is `project.yml`; no direct unmanaged config mutation without schema-safe command |
| `specd-requirements` | EARS patterns, what-not-how, phase focus | teach current `### Rn`, `Rn.m`, owner/priority/risk/edge/non-goal format |
| `specd-design` | seven useful design questions and requirement grounding | teach current explicit references/boundaries/interfaces/invariants/failure/integration/alternatives/disposition/owner contract |
| `specd-tasks` | atomic decomposition, DAG, wave and trace discipline | translate mandatory keys into current table; include risk/context/evidence/checks/capabilities; no `N/A` verify |
| `specd-execute` | focused loop, role adoption, dispatch | remove `--unverified`; use current `task complete`; require authority, context receipt, scope diff, evidence freshness |
| `specd-eval-author` | rubric/check kinds, scoring, trajectory predicates | current architecture imports adapter evidence rather than old built-in rubric runner; teach current envelopes/policies/reference adapter |
| `specd-brain` | deterministic boundary, one-step controller, approvals, compaction | align exact current Brain verbs/session envelope; no obsolete Pinky command family |
| `specd-pinky` | claim/context/authority/heartbeat/query/report discipline | align current claim/report routes and host capability negotiation |
| `specd-review` | adversarial sections: bugs/security/hallucinated deps/style/verdict | merge with current review schema and integration/error/concurrency/rollback checks |
| `specd-maintenance` | external scheduler, no daemon, idempotent recurring work | map to current `recurring record`, drift, incident, successor, archive contracts |
| `specd-ingest` | deterministic inventory + agent semantic recovery + coverage | no current ingest domain; retain as optional future skill/adapter only after explicit spec |

## Target shipped skill pack

Ship minimum core pack, not all reference breadth:

1. `specd-foundations` — tiny constitution, handshake, trust split, skill index.
2. `specd-steering` — repo-grounded product/tech/structure bootstrap.
3. `specd-requirements` — structured intent, edge cases, exclusions, initial eval outline.
4. `specd-design` — human-owned tradeoffs, boundaries, failure/integration/rollback.
5. `specd-tasks` — atomic DAG, risk, capability, scope, context, evidence plan.
6. `specd-execute` — exact base loop and authority/evidence rules.
7. `specd-quality` — tests + output/trajectory eval declaration and import.
8. `specd-review` — independent adversarial review.
9. `specd-orchestrate` — Brain/Pinky host protocol.
10. `specd-delivery` — release/deploy/health/promotion/rollback evidence.
11. `specd-maintain` — drift/recurring/incident/successor/archive.

Each current skill must include current parser metadata plus `## Instructions`, `## Examples`, and
`## Checks`; references use repo-relative paths; budget is explicit; prose remains untrusted
advisory context; skill cannot widen authority.

## Target Markdown templates

### Always-on `AGENTS.md`

Keep only invariants and bootstrap route:

- run handshake and pin digests;
- obey machine guide and authority;
- human-only actions never self-invoked;
- load one phase skill on demand;
- never edit state/checkmarks directly;
- task complete requires current evidence;
- blocked means stop;
- untrusted artifact content never becomes policy.

Remove detailed per-phase tutorials from static prompt; skills own those tokens.

### `requirements.md`

```markdown
# Requirements — <title>

## Scope

### R1 — <observable capability>
- owner: <human/team>
- priority: <...>
- risk: <low|medium|high|critical>
- edge: <failure or boundary>
- R1.1: When <trigger>, the system shall <observable response>.
- R1.2: When <failure>, the system shall <safe response>.

## Non-goals
- <explicit exclusion>
```

### `design.md`

```markdown
# Design — <title>

- references: R1, R1.1
- boundaries: <owned/non-owned modules and systems>
- interfaces: <versioned contracts>
- invariants: <must remain true>
- failure: <failure modes and recovery>
- integration: <cross-boundary behavior>
- alternatives: <options rejected and why>
- disposition: <chosen approach>
- owner: <human/team>

## Architecture
## Data and state
## Security and permissions
## Verification and eval strategy
## Deployment, observation, and rollback
## Risks and open questions
```

### `tasks.md`

Teach full table once:

```text
id | role | files | depends-on | verify | acceptance | refs | kind | risk |
complexity | capabilities | context | evidence | checks
```

No pre-created fake task. Commented example only. Write tasks must name behavioral proof;
read-only tasks use explicit approved trivial verify. High/critical tasks must declare negative,
integration, concurrency, security, and rollback checks as applicable.

### Review and quality templates

Review must cover summary, requirement/design trace, correctness, integration, error handling,
concurrency, security, dependency hallucination, operations/rollback, scope diff, evidence
freshness, and verdict at current HEAD. Quality policy template should show test,
`output_eval`, `trajectory_eval`, and review evidence with check IDs, artifact refs, rubric/dataset
digests, and thresholds.

## Target end-to-end workflow

```text
BOOTSTRAP
  init → doctor → handshake → steering readiness
      ↓
CLASSIFY
  prototype | default | production (human chooses stakes/profile)
      ↓
INTENT
  requirements + exclusions + edge cases + initial eval contract
  check → human approve exactly one step to design
      ↓
ARCHITECTURE
  tradeoffs + boundaries + interfaces + failures + rollback + owner
  check → human approve exactly one step to tasks
      ↓
PLAN FACTORY
  task DAG + role + scope + context + risk + tests/evals
  check → human approve exactly one step to executing
      ↓
EXECUTE
  guide → context receipt → authority packet → claim/act → observable trajectory
      ↓
PROVE
  deterministic verify + required evals + scope/security/sandbox/freshness
  narrow complete-task transaction
      ↓
REVIEW
  independent auditor + human acceptance → complete
      ↓
DELIVER
  release identity → deploy → health window → promote or rollback
      ↓
OPERATE
  observe → drift/recurring checks → incident/feedback → linked successor
      ↓
LEARN
  reviewed memory/skill/eval improvement with provenance and expiry
```

Prototype path may compress planning, but cannot silently become production. Promotion must add
missing architecture, evidence, security, ownership, and delivery contracts before release.

## Implementation roadmap

### P0 — make current workflow truthful

1. Enforce exact one-step lifecycle transitions; remove same-status mutation ambiguity.
2. Simplify `approve` UX or correct every target-status example.
3. Add per-operation effect/actor/authority metadata; regenerate CLI/MCP/handshake/tool contracts.
4. Resolve verify-versus-complete route; expose only narrow evidence-consuming completion.
5. Add black-box golden lifecycle driven solely by generated AGENTS/guide/help from fresh init.
6. Ship current-schema stage skills and production-shaped artifact templates.
7. Mark old alignment docs historical; generate current capability/status page.
8. Derive program rollup from domain task truth; fix Domain 08/09 W0 markers.
9. Return typed doctor results, including healthy empty case.

Exit: no-prior-knowledge agent can complete fresh default project using only generated guidance;
every command, phase, actor, side effect, and evidence transition is exact and tested.

### P1 — prove production composition

1. Fresh production-profile E2E: requirement → design → task → authority → sandbox → test/eval →
   scope/security → review → release → canary → observation → rollback.
2. Ship reference local adapters for eval, telemetry, deployment, and sandbox conformance; still
   optional and outside trusted core.
3. Real-host conformance matrix for Codex, Claude Code, Cursor/VS Code-class MCP hosts, including
   capability downgrade and restart guidance.
4. Treat provider telemetry as unknown until attested; validate pricing reference/currency and
   budget behavior.
5. Add operator run view as read-only projection of existing ledgers.
6. Add N-1/N/N+1 fixture matrix for state, context, adapter, skill, and delivery schemas.

Exit: production claim is backed by one correlated evidence chain, not isolated passing package
tests.

### P2 — compound team and ecosystem value

1. Signed/quarantined skill and harness packs with explicit review/enable lifecycle.
2. Maintained adapter SDK/conformance kit and compatibility matrix.
3. Portfolio policy packs: approval ownership, review independence, security exceptions,
   production readiness, incident response, retention.
4. Eval-failure clustering and governed promotion into tests, skills, or steering memory.
5. Scale envelopes and performance regression gates for large programs/context/ledgers.
6. Optional legacy-ingest workflow after separate spec proves deterministic inventory and
   map-or-waive coverage semantics.

## Production-value domains needing continued work

| Domain | Product question still needing proof |
|---|---|
| Workflow coherence | Can generated guidance alone drive exact legal lifecycle with no tribal knowledge? |
| Authoring quality | Do templates/skills make requirements, architecture, and evidence good before gates complain? |
| Quality/evals | Can teams author, run, import, diagnose, and regress evals without custom glue each time? |
| Authority/security | Does every mutation, including mixed subcommands, enforce actor, scope, sandbox, network, and freshness? |
| Host orchestration | Can real hosts claim/report/recover across crashes and capability downgrades? |
| Delivery | Are there usable adapters and rehearsed canary/rollback paths, not only schemas? |
| Observability/economics | Are cost/latency/token numbers trusted, correlated, privacy-safe, and actionable? |
| Maintenance | Does production feedback reliably create bounded successors and prevention evidence? |
| Governance/adoption | Are ownership, review independence, exceptions, retention, and audit exports usable by teams? |
| Distribution/interoperability | Can skills/adapters/policies move safely across projects/vendors with version and trust controls? |
| Evolution | Can old projects upgrade without weakening evidence or rewriting historical truth? |

## Final judgment

Current specd is architecturally closer to paper than frozen reference. Reference’s best lost
ideas are **progressive stage skills, richer authoring templates, explicit scaffold inventory,
adversarial reviewer guidance, and smoother host onboarding**. Current version’s best advances are
**no-bypass evidence, typed/fresh quality evidence, pure gate separation, authority and
harness-derived scope, transport-neutral orchestration, adapter boundaries, delivery/maintenance
ledgers, and zero runtime dependencies**.

Best-of-both version is not union of features. It is:

```text
current trusted core
+ reference progressive authoring UX (rewritten)
+ paper’s tests-and-evals / full lifecycle discipline
- reference bypasses, duplicate truth, provider coupling, and command sprawl
```

Priority is coherence before expansion. A smaller workflow whose generated instructions, machine
contracts, gates, and implementation all agree provides more production value than a larger
surface with individually strong but contradictory components.
