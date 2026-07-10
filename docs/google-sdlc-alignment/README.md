# Google SDLC Paper Alignment — Domain Analysis

## Purpose and conclusion

This directory decomposes `sdlc-with-vibe-coding.md` into implementation domains that can be hardened independently while preserving `specd`'s central invariants: deterministic gates, evidence integrity, human approval, atomic/CAS state, bounded context, zero runtime dependencies, and subtractive scope.

The conclusion is deliberately two-part:

- **`specd` is already a strong deterministic harness foundation.** Structured intent, phase gates, task DAG/frontier, local state, evidence, roles, bounded-manifest intent, MCP discovery, and crash-conscious orchestration map directly to the paper's shift from vibe coding to agentic engineering.
- **It is not yet production-proven as a self-guiding agent workflow or as the paper's broader production-agent substrate.** First-class evals, enforced trajectory/scope, default production security, truthful bounded context, end-to-end worker dispatch, model routing, deployment feedback, and long-running governance remain incomplete.

Alignment should not mean putting model calls, provider SDKs, deployment clients, or a production-agent runtime inside trusted core paths. `specd` should own deterministic policy and evidence contracts; optional external adapters should perform non-deterministic or networked work.

## Source and method

The analysis uses:

- `sdlc-with-vibe-coding.md` as the requested comparison and domain seed;
- `sdlc-paper.md` as the locally extracted paper text for section/line verification;
- current non-`reference/` code, tests, templates, and docs as implementation evidence; and
- the repository's stated production invariants in `AGENTS.md` and `docs/contributor-guide.md`.

Paper statements are separated from implementation inferences. Recommendations are not claims that the paper mandates a specific filename, CLI verb, or schema.

## Domain map

| # | Domain | Paper concepts | Current assessment | Primary ownership |
|---:|---|---|---|---|
| 01 | [Lifecycle and structured intent](01-lifecycle-and-structured-intent.md) | Intent over syntax, phase compression, human architecture judgment, factory model | Strong ratchet and artifacts; weak traceability/change invalidation and production profiles | Core |
| 02 | [Context, knowledge, and skills](02-context-knowledge-and-skills.md) | Six context types, static/dynamic context, portable skills, token economics | Strong steering/roles direction; manifest content/path/accounting and skills are incomplete | Core + adapter contract |
| 03 | [Agent–tool driving and native guidance](03-agent-tool-driving-and-native-guidance.md) | Harness legibility, tools, conductor/orchestrator, safe next action | Palette/handshake/MCP are strong; current generated guidance and task packet can misguide | Core + host conformance |
| 04 | [Verification, evals, and quality](04-verification-evals-and-quality.md) | Tests plus evals, output/trajectory assessment, rubrics, 80% problem | Strong no-bypass shell evidence; no eval classes, datasets/rubrics, enforced trajectory, or quality lint | Core schema/gates + eval adapters |
| 05 | [Orchestration, multi-agent work, and model routing](05-orchestration-multi-agent-and-model-routing.md) | Conductor/orchestrator, MCP/A2A, model routing | Strong ledger/lease/controller foundation; no wired worker claim/report/launch or routing | Core controller + host/A2A/model adapters |
| 06 | [Security, permissions, and governance](06-security-permissions-and-governance.md) | Deterministic hooks, least privilege, production governance | Opt-in/narrow security and prompt roles; declared scope/role authority are not fully enforced | Core policy/gates + sandbox/security adapters |
| 07 | [Observability, cost, and operational economics](07-observability-cost-and-operational-economics.md) | Traces, latency/cost, token burn, economic sustainability | Deterministic reports and optional annotations; incomplete run traces and trusted measurement | Core events/export + telemetry adapters |
| 08 | [Deployment and production assurance](08-deployment-and-production-assurance.md) | AI-aware CI/CD, monitoring, rollback, production operation | CI/reporting foundation; no release/environment/deploy/observe/rollback evidence lifecycle | Core contracts + CI/CD adapters |
| 09 | [Maintenance, modernization, and operating model](09-maintenance-modernization-and-operating-model.md) | Evolution, persistent learning, governance, team change | Memory/decisions/program links are useful; recurring invariants, drift, incidents, and aging are missing | Core projections/templates + external work systems |
| 10 | [Scope boundaries and interoperability](10-scope-boundaries-and-interoperability.md) | Full harness ecosystem, MCP/A2A, production-agent runtime | Core boundary is sound but adapter contracts/capability negotiation/data policy need definition | Architecture contract |

## Common field backbone

The paper's concepts converge on a shared provenance chain. Exact schemas should be designed through the normal spec process, but every domain needs compatible meanings for these fields:

| Group | Common fields | Why they are common |
|---|---|---|
| Contract | `schema_version`, `kind`, `capabilities_required` | Make every context, mission, evidence, adapter, and deployment artifact negotiable and fail-closed. |
| Intent identity | `spec_slug`, `requirement_ids`, `task_id`, `acceptance_ids` | Preserve traceability from intent to work and evidence. |
| Run identity | `run_id`, `session_id`, `mission_id`, `attempt`, `correlation_id` | Join driver, worker, tool, eval, trace, and release records without prose matching. |
| Subject identity | `git_head`, `diff_digest`, `release_id`, `environment` | Prove exactly what code/config/output was assessed or deployed. |
| Policy identity | `phase`, `profile`, `role`, `authority_ref`, `policy_digest`, `config_digest`, `palette_digest`, `guidance_digest` | Prevent stale or unauthorized actions and explain the governing rules. |
| Scope/context | `declared_files`, `context_refs`, `context_digests`, `selection_reason`, `required` | Bound work and model knowledge while detecting omission/drift. |
| Execution | `actor`, `worker_id`, `host`, `adapter`, `provider`, `model`, `lease`, `limits` | Attribute actions and enforce capability, time, cost, retry, and transport boundaries. |
| Evidence | `evidence_class`, `check_id`, `dataset_digest`, `rubric_digest`, `trace_digest`, `output_ref`, `verdict`, `score` | Keep tests, output evals, trajectory evals, review, security, and deployment evidence distinct but composable. |
| Operations | `started_at`, `finished_at`, `tokens`, `cost`, `duration`, `health`, `status`, `blocker`, `next_action` | Support production control and economics without interpreting missing data as success/zero. |
| Governance | `owner`, `approver`, `reason`, `source_ref`, `expires_at`, `supersedes` | Preserve human accountability and prevent permanent invisible exceptions/stale knowledge. |

Three rules apply across all fields:

1. use a reference and digest instead of copying large/sensitive content into ledgers;
2. keep “unknown” distinct from zero, empty, pass, or not applicable; and
3. never allow an imported/model-generated record to satisfy a gate until deterministic code validates its schema, identity, freshness, and policy.

## Highest-risk findings in current behavior

These findings should be resolved before describing the agent-driving workflow as production-ready:

1. `internal/context/manifest.go` emits `specs/<slug>/...` for artifacts stored at `.specd/specs/<slug>/...`.
2. The context manifest omits `design.md` and the task's declared source files.
3. Core context entries estimate token use from labels/paths rather than the content a host must load; a passing budget can therefore understate effective context.
4. Generated `AGENTS.md` uses some slugless command shorthand that current handlers reject.
5. MCP configuration emits `SPECD_SPEC`, but the runtime does not consume it.
6. The roles gate checks only for a non-empty value; unknown roles can fall back to craftsman prompt/mode behavior, and auditor is not explicitly mapped by `ModeForTask`.
7. The `files` gate checks declaration presence rather than enforcing actual diff scope; worker `ChangedFiles` is self-reported and not compared with the task.
8. Completion checks for a resolvable evidence HEAD but does not uniformly prove that required evidence is fresh for the current subject HEAD/digests.
9. Security is opt-in, `.specd/` is excluded from injection scanning despite being model context, scan enumeration/read failures can miss coverage, and verify sandboxing is optional.
10. Brain durably records missions/leases but does not itself launch a Pinky worker; public worker claim/report lifecycle is not wired end to end.
11. Brain requires state mode `"orchestrated"`, but the declared modes are `"default"` and `"agent"` and no supported CLI path sets `"orchestrated"`; real users cannot satisfy the documented precondition without the forbidden act of hand-editing state.
12. `scripts/regress-domains.sh` W0 reads absent `specs/progress.md`; `awk` reports the missing file, but the loop remains empty and W0 prints success. The script exits zero without evaluating that advertised invariant.

The full Go race-enabled suite, `go vet`, formatting check, structural/docs lints, `regress-all`, and `regress-lint` passed during this analysis. `regress-domains` exited zero and its W1–W6 probes passed, but W0 has the fail-open input defect above. These results support the existing encoded invariants; they do not establish production readiness for the twelve defects above.

## Recommended implementation order

Priorities inside each domain are **local dependency priorities**: that domain's P0 must precede
its P1/P2. The sequence below is the **cross-domain release order** based on current production
risk. For example, maintenance provenance is Domain 09 P0 but enters the global program after
the immediate driver/authority/evidence blockers; adapter envelope and data-boundary rules from
Domain 10 are pulled forward wherever a P0 core feature crosses a process or network boundary.

### P0-A — Make current contracts truthful

- Correct context paths, include design and declared files, account for actual referenced content, and fail on missing required items.
- Make scaffolded commands executable and either implement active-spec resolution or remove the inert MCP promise.
- Reject unknown/mis-mapped roles and make machine guidance return exact actor-aware next actions.

**Exit condition:** a fresh project can be driven by an agent with no prior `specd` knowledge using only generated guidance and machine surfaces; every returned path/command resolves and context omissions are explicit.

### P0-B — Enforce production authority and scope

- Add explicit prototype/production profiles.
- Enforce declared-file scope against a harness-derived diff, including untracked/rename/delete/mode/symlink cases.
- Bind role to host/tool capability, make production security current at completion/submit, and require a hardened sandbox policy where configured.
- Scan actual runtime context and pending changes; scanner inability fails closed.

**Exit condition:** a passing test cannot complete production work performed by an unauthorized role, outside declared scope, without current required security/sandbox evidence.

### P0-C — Close the quality/evidence gap

- Add evidence classes and current-subject freshness.
- Define versioned output/trajectory eval artifacts, rubric/dataset/trace provenance, and deterministic import/gates.
- Add risk-based verify-quality and acceptance-to-evidence coverage lint.

**Exit condition:** reports say what was proved; deterministic test failure is never bypassed; required eval/trajectory evidence cannot be confused with prose, another class, or a stale run.

### P0-D — Make orchestration executable end to end

- Add a validated, CAS-protected public transition into orchestrated mode, or remove the unreachable precondition and document the replacement authority model.
- Version mission/claim/heartbeat/report envelopes.
- Separate pending controller dispatch from a real worker lease.
- Validate role, authority, scope, context/config/palette digests, actual diff, evidence, and HEAD on report.
- Publish a fake-host end-to-end and crash/recovery conformance fixture.

**Exit condition:** “dispatch” has one documented meaning, a real worker identity owns every active lease, and no result completes outside the correlated mission/evidence chain.

### P1 — Add operational and economic substrate

- Normalize observable run events and trusted/unknown measurement provenance.
- Add deterministic model capability routing and wire budget/deadline brakes; providers remain external.
- Add eval, trace export, deployment, rollback, and runtime-feedback adapter contracts.
- Add governed exceptions, memory aging/supersession, drift, and recurring invariants.

### P2 — Add ecosystem and portfolio scale

- A2A round-trip support without weakening authority/evidence fields.
- Portable, versioned, progressively loaded Agent Skills.
- Cross-host driver/adapter conformance, production canary/rollback suites, and portfolio governance exports.
- Optional organizational templates and external dashboards/work-system integrations.

## Production verification ladder

“Production-ready” should be an evidence profile, not a prose label.

| Level | What must be proven | Representative checks |
|---|---|---|
| L0 — Core integrity | Build, unit/integration/race, format/vet/lint, deterministic outputs | Existing CI suite, repeated-order tests, schema golden files |
| L1 — Fresh-project lifecycle | Installed binary and generated docs drive a clean repo through every phase | Black-box `init/new/approve/context/verify/check/review/submit` fixtures |
| L2 — Agent/host conformance | A no-prior-knowledge agent receives correct context/actions on each supported host | CLI/MCP/Codex/Claude/Pinky contract tests, drift/upgrade cases |
| L3 — Failure and recovery | Crashes, concurrency, timeouts, stale state, and retries preserve safety | CAS/lock stress, checkpoint fault injection, expired lease, partial ledgers |
| L4 — Authority/security | Roles, scope, tools, sandbox, context trust, and exceptions fail closed | Undeclared edits, injection/secrets, missing scanner/sandbox, credential/network probes |
| L5 — Quality/evals | Tests, output evals, trajectory evals, rubrics/datasets, and freshness compose correctly | Stochastic repeat fixtures, stale digest, forbidden/missing step, shallow verify |
| L6 — Release assurance | CI/CD identity, health, observation, and rollback are correlated and recoverable | Ephemeral environment, canary failure, stale release, rollback rehearsal |
| L7 — Long-running operation | Drift, recurring invariants, incidents, memory/decision aging, and portfolio scale work | Scheduled re-check, incident successor, expired policy, large-program envelope |

This analysis directly ran L0's full race-enabled Go suite, vet/format/docs/test lints, and regression scripts. The W0 regression defect means L0 is not wholly clean despite successful process exit. It did not claim L1–L7 are currently satisfied; the domain documents define the missing acceptance scenarios needed to make those claims responsibly.

## Context design rules across the roadmap

- Required intent and authority context wins over optional memory/examples.
- Return compact decisions, findings, ids, and references; retrieve large datasets, traces, source, and logs on demand.
- Label instructions, trusted policy, untrusted input, examples, and evidence separately.
- Include selection/omission reasons and content digests in manifests.
- Never store or request hidden chain-of-thought; trajectory means observable tools, results, file effects, and lifecycle events.
- Redact secrets and production/user data before terminal output, durable evidence, adapter transport, or model context.
- Stable structured blocker codes plus one valid next action prevent retry loops and reduce the need to load troubleshooting prose.

## How to turn this analysis into work

Each domain contains its own P0/P1/P2 plan, likely code/artifact surfaces, deterministic acceptance checks, production scenarios, context controls, risks, and non-goals. Before implementation:

1. create/amend the project `SPEC.md` or top-level planning specs using the repository's normal spec process;
2. turn the P0 sequence above into small DAG tasks with declared files and verification;
3. preserve current on-disk/CLI compatibility or explicitly version and migrate it;
4. add the failing black-box/conformance test before each production-contract fix; and
5. keep adapters optional and trusted core network/model-free.
