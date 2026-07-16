# Domain 04 — Verification, Evals, and Quality

> **Status:** Historical assessment; proposals are non-normative.
> **As of commit:** `f62f16f44f92de5fa59a9304b8b10b0721564eaa` (2026-07-10).
> **Superseded by:** [`specs/11-workflow-coherence`](../../specs/11-workflow-coherence/README.md) and current normative docs.

## Purpose

Define the verification domain needed for `specd` to satisfy the paper's boundary between vibe coding and agentic engineering. The goal is not to replace `specd verify`; it is to preserve its deterministic, no-bypass evidence contract while adding the missing contracts for output quality, observable agent trajectory, rubric and dataset provenance, and production regression evaluation.

This analysis uses `sdlc-with-vibe-coding.md` as the comparison source and `sdlc-paper.md` as its extracted paper text. Recommendations preserve the Go standard-library-only binary and keep non-deterministic scoring outside deterministic gate bodies.

## Paper position

The paper makes verification the primary dividing line between casual prompting and production agentic engineering:

- Deterministic **tests** check deterministic behavior; **evals** check non-deterministic behavior using labelled datasets, scoring rubrics, and, where useful, LM judges (`sdlc-paper.md:121-140`). Both are required for the paper's strongest production claim.
- **Output evaluation** checks the final artifact, including compilation and tests. **Trajectory evaluation** checks the observable sequence of steps and tool calls. A fluent result that skipped required verification is specifically identified as dangerous (`sdlc-paper.md:220-226`).
- Tests and evals should be authored before generation because together they communicate what “correct” means (`sdlc-paper.md:472`).
- An eval has no meaning without an explicit rubric. The paper names task success, tool-use quality, trajectory compliance, hallucination, and response quality as separate scoring targets (`sdlc-paper.md:480-482`).
- Production substrate includes trajectory and final-response evals in CI plus traces for every agent run (`sdlc-paper.md:492-494`).
- The **80% problem** is not syntax. It is plausible code with wrong business assumptions, missed edge cases, incomplete error handling, bad integrations, or long-term architectural cost; such code may pass basic tests (`sdlc-paper.md:352-360`).

The resulting principle is: a single successful command proves one bounded proposition at one revision. It does not, by itself, prove reliable task success, safe tool use, or coverage of the final 20%.

## Current `specd` handling with evidence paths

| Capability | Current handling | Evidence |
|---|---|---|
| Task test contract | Every task must declare a non-empty `verify` command. Read-only tasks may use a trivially passing command. | `internal/core/gates/core.go` (`verifyCommands`); `internal/core/tasksparser.go`; `docs/validation-gates.md` |
| Command execution | `specd verify` runs the task command, optionally under a `bwrap`-compatible sandbox, with an optional timeout. | `internal/cmd/registry.go` (`runVerify`); `internal/core/verify/exec.go` |
| Evidence integrity | The append-only record includes task id, command, exit code, git HEAD, time/actor, and optional telemetry. Completion requires exit `0` and a non-empty, non-`unknown` HEAD. | `internal/core/evidence.go`; `internal/core/task_complete.go`; `internal/cmd/lifecycle.go` |
| No bypass | Escalation override only resets the failure counter; it cannot substitute for passing evidence. | `internal/cmd/registry.go:595-601`; `internal/cmd/lifecycle.go:152-161`; `docs/concepts.md` |
| Acceptance evidence | Optional per-criterion records accept operator-supplied `pass|fail` plus evidence text/path. They cannot create task evidence or replace `verify`. | `internal/cmd/registry.go` (`runVerifyCriterion`); `internal/core/criteria.go`; `docs/validation-gates.md` |
| Human review | An optional review gate requires an approve verdict at the expected HEAD. | `internal/core/gates/core.go`; `internal/core/review.go`; `internal/cmd/review.go` |
| Orchestrated evidence primitives | An unexposed report-validation primitive checks for a live matching lease and passing verify evidence at the same reported HEAD. The ACP ledger can store dispatch/claim/report events, changed-file claims, and verify references, but no public non-test path wires the complete report lifecycle yet. | `internal/cmd/brain_worker.go`; `internal/orchestration/acp.go`; `internal/orchestration/lease.go` |
| Context budget | Task context is a bounded manifest of core spec/task/role references plus steering and memory; optional context is shed deterministically. | `internal/context/manifest.go`; `internal/context/budget.go`; `internal/core/gates/contextbudget.go` |
| Repository regression checks | Unit/integration tests and `scripts/regress-*` re-run task verification and domain invariants for this repository. | `TESTING.md`; `scripts/regress-all.sh`; `scripts/regress-domains.sh`; `scripts/regress-lint.sh` |

This is a strong **test-evidence** implementation. It is not yet a general eval system. In particular, an exit-zero shell line is treated as sufficient task evidence even if it is shallow, and the ACP ledger is not a complete or enforced tool trajectory.

## Common contract and fields

The target should use one evidence envelope with explicit evidence classes rather than calling every check an “eval.”

| Contract field | Paper meaning | `specd` today | Target contract |
|---|---|---|---|
| `spec_id`, `task_id`, `run_id`, `attempt` | Identify the evaluated mission and repeat | Spec/task exist; attempt exists in ACP only | Required stable identifiers across test, output-eval, and trajectory records |
| `evidence_class` | Distinguish what was proved | Implicitly task verification | Enum: `test`, `output_eval`, `trajectory_eval`, `review`; never infer from command text |
| `subject_revision` | Pin verdict to evaluated artifact | `git_head` is recorded | Require current/reachable commit and, where relevant, clean-tree or diff digest |
| `producer` and `producer_version` | Reproduce the evaluator | Command string and actor only | Tool/adapter name, version, config digest, and execution environment identity |
| `command` / `check_id` | Deterministic test or evaluator invoked | `command` | Stable check id plus exact invocation; command remains data, not gate logic |
| `dataset_id`, `dataset_version`, `dataset_digest` | Labelled benchmark provenance | Missing | Required for dataset-backed evals; immutable digest, owner, and source policy |
| `case_id` and labels | Explain per-case coverage/failure | Missing | Per-case result with expected label/reference and redacted input reference |
| `rubric_id`, `rubric_version`, `rubric_digest` | Define what the score means | Acceptance prose/review report only | Versioned dimensions, scales, thresholds, critical-failure rules, and owner |
| `scorer_type` | Code, human, heuristic, or LM judge | Shell exit or operator verdict | Enum with scorer-specific metadata; `lm_judge` remains optional |
| `model_provider`, `model_id`, `prompt_digest`, `sampling` | Make LM scoring interpretable | Missing | Required only for LM judges; record, never execute inside a gate |
| `repetitions`, `aggregation`, `threshold` | Manage non-deterministic variance | Missing | Predeclared policy such as minimum pass count, median score, and critical-case rule |
| `output_ref`, `output_digest` | Pin final artifact/result | Optional free-form evidence reference | Content-addressed artifact reference with size/type and redaction status |
| `trace_ref`, `trace_digest` | Pin observable trajectory | Partial ACP events | Ordered, content-addressed action/tool-result trace; no hidden chain-of-thought |
| `required_steps`, `forbidden_steps` | Check trajectory compliance | Role prompt instructions only | Deterministic policy evaluated over normalized observable events |
| `verdict`, `score`, `dimension_scores` | State result without conflating meanings | Exit code or criterion status | Normalized verdict; scores retained separately from deterministic pass/fail policy |
| `waiver_ref` | Govern an accepted exception | Escalation reason/decision records are separate | Reasoned, scoped, expiring approval; cannot waive non-bypass test evidence |
| `created_at`, `actor`, `signature_or_digest` | Audit provenance | Time/actor on task evidence | Required provenance and tamper-evident digest for every imported record |

### Evidence-class semantics

- **Test:** deterministic executable assertion such as compilation, unit, integration, property, race, lint, schema, or deployment smoke check. Exit status and artifacts are evidence.
- **Output eval:** evaluates the final produced artifact. A test is one deterministic form of output evaluation, but qualitative API usefulness, response groundedness, or patch maintainability may require a rubric and human or LM scorer.
- **Trajectory eval:** evaluates observable actions: tools selected, ordering, retries, scope, required verification, and forbidden operations. It must not require private model reasoning or chain-of-thought.
- **Review:** human or policy-controlled judgment for architecture, maintainability, and risk. It complements rather than overrides failing tests.

## Gaps and failure modes

1. **Tests and evals are conflated.** `verify:` records a process exit code but has no evidence-class declaration. Reports cannot say which part of the paper's quality contract is covered.
2. **No rubric or dataset provenance.** There is no first-class evalset, case label, rubric version, ownership, digest, scorer metadata, or stale-result invalidation when those inputs change.
3. **No trajectory evaluation.** ACP records selected lifecycle events, not a normalized trace of tool calls and results. `ChangedFiles` is worker-reported and is not validated. A worker may reach a passing artifact through a forbidden or misleading path without the evidence gate detecting it.
4. **LM-judge support is absent, but embedding it in a gate would be the wrong fix.** Network/model calls inside `CoreRegistry()` would destroy purity, determinism, offline behavior, and reproducibility. Judge execution belongs in an external adapter; a gate may only validate its pinned artifact against a predeclared policy.
5. **The 80% problem remains author-dependent.** A shallow `printf ok`, compile-only command, or happy-path unit suite can certify a write task. Optional criteria evidence is operator-declared and does not measure edge-case or integration coverage.
6. **Evidence freshness is weaker than the desired contract.** Completion checks that `git_head` is present and not `unknown`; it does not compare the record to the current HEAD or validate a content digest. A later change can therefore leave earlier passing task evidence logically stale.
7. **No repeatability policy for stochastic systems.** One pass cannot establish reliability, and there is no deterministic aggregation of repeated externally scored runs.
8. **No continuous eval flywheel.** Production samples, clustered failures, regression promotion, and prompt/tool version comparisons are outside the current lifecycle and report model.
9. **No verify-quality policy.** The `verify` gate checks presence, not whether the command exercises acceptance criteria, error paths, integration boundaries, concurrency, or rollback behavior.
10. **Free-form evidence can misguide the driver.** Criterion evidence accepts text or a path without a digest/schema. A model can mistake an assertion for proof unless the context packet labels evidence class, provenance, and freshness.

## Target best-practice workflow

1. **Classify production risk during planning.** Requirements identify deterministic behavior, stochastic behavior, critical cases, and prohibited trajectories. Tasks declare required evidence classes and named check/eval ids.
2. **Author the contract before implementation.** Commit tests, dataset manifest, cases, rubric, threshold policy, and trajectory policy before the implementing task becomes runnable. Prototypes may use a lighter profile, but a production profile fails closed on missing required coverage.
3. **Give the agent a compact quality packet.** `specd context` includes only the task's required check ids, rubric summary, critical cases, and commands. Full datasets and traces remain references loaded by tools on demand.
4. **Implement inside declared scope.** The driver follows its role and emits normalized observable action events. Do not capture hidden reasoning; record tool name, sanitized arguments, result class, affected paths, time, and correlation ids.
5. **Run deterministic tests first.** `specd verify` remains the non-bypass foundation. A failed required test always fails the task regardless of an eval score.
6. **Run eval adapters outside gates.** A local/CI command produces schema-valid JSON/JSONL artifacts. Code, human, heuristic, or optional LM judges may produce scores. Each artifact pins revision, dataset, rubric, scorer config, trace/output digest, and repetitions.
7. **Import and validate artifacts.** `specd` verifies schema, digests, provenance, freshness, required cases, aggregation, and configured thresholds using standard-library code only. It does not call a model or network from a gate.
8. **Complete only when all declared evidence classes pass.** Required eval evidence gets the same no-bypass semantics as required test evidence. A waiver may relax a non-critical policy only where the project policy explicitly permits it; it never fabricates passing evidence.
9. **Review the hard 20%.** The auditor packet emphasizes ambiguous assumptions, integrations, error handling, security, rollback, and cases not exercised by the suite. Human architectural judgment remains explicit.
10. **Feed production failures back.** Redacted production cases are triaged, labelled, added to a versioned regression dataset, and linked to the corrective spec. Historical results remain interpretable against their original dataset/rubric digests.

## Recommended action plan

| Priority | Recommended change | Code/artifact surface | Deterministic acceptance checks |
|---|---|---|---|
| **P0** | Define evidence classes and a versioned eval artifact envelope. Add task-level required evidence references without weakening existing `verify`. | `internal/core/eval*.go`; `internal/core/tasksparser.go` or a companion `.specd/specs/<slug>/evals.md`; `docs/open-spec-format.md`; `docs/validation-gates.md` | Round-trip fixtures are byte-stable; unknown versions/classes fail closed; old task files preserve behavior; a required class cannot be satisfied by another class. |
| **P0** | Add deterministic eval-evidence and freshness gates. Pin HEAD plus dataset/rubric/output/trace digests. Keep scoring execution external. | `internal/core/gates/`; `internal/cmd/registry.go`; `.specd/specs/<slug>/eval-evidence.jsonl`; report model | Missing/corrupt/stale/wrong-digest/wrong-task artifacts fail; identical input yields byte-identical ordered findings; gate tests make no network/model calls; required test failure cannot be overruled. |
| **P0** | Define a normalized observable trajectory envelope and require proof of mandatory steps for production tasks. | `internal/orchestration/acp.go`; MCP/driver event import; `.specd/specs/<slug>/traces/*.jsonl` | Sequence numbers are monotonic; duplicate run/event ids fail; forbidden tool or missing verify event fails; secrets and hidden reasoning fields are rejected/redacted; trace digest matches artifact. |
| **P0** | Add verify-quality lint for production profiles and risk-critical tasks. Require explicit coverage mapping from acceptance criteria to test/eval ids. | Task/schema gates; `project.yml`; requirements/task artifacts; `scripts/regress-lint.sh` | Write tasks cannot use trivial verify; every critical criterion maps to at least one required check; unknown ids and uncovered critical criteria fail closed. |
| **P1** | Add an external eval runner contract with code/human/heuristic/optional LM-judge adapters and deterministic import. | New `specd eval import/status` command surface; adapter examples under `scripts/` or a separate integration package; config schema | Golden adapter artifacts import identically; judge metadata is mandatory for LM records; repetitions aggregate by configured pure function; offline gate suite passes with adapters unavailable. |
| **P1** | Add dataset and rubric governance: owner, version, digest, critical cases, review state, and change invalidation. | `evals/<id>/{dataset.jsonl,rubric.json,manifest.json}` or project-configured external refs; schema/report code | Editing dataset/rubric changes digest and invalidates prior result; duplicate case ids, missing labels, unowned rubrics, and threshold-free rubrics fail. |
| **P1** | Add explicit 80%-problem review templates and risk-sensitive suites. | Review template, role prompts, context manifest, docs | Production review refuses empty edge-case/integration/error-handling assessment; context includes the summary but not the full dataset. |
| **P2** | Build the continuous quality flywheel: production sample intake, failure taxonomy, regression promotion, trend/baseline reports. | Append-only quality ledger; `internal/core/report.go`; CI export/import adapters | PII/secret policy is checked before ingestion; promoted cases are immutable/versioned; trend calculations are deterministic from ledger inputs. |
| **P2** | Add statistical reliability and comparative evaluation policies without making gates stochastic. | Eval policy schema and report projections | Given fixed imported runs, aggregation and confidence rules are reproducible; insufficient sample count fails or reports “insufficient,” never silently passes. |

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| Unit tests pass but the agent never ran the required security/smoke tool | Output test passes; trajectory requirement fails; task cannot complete. |
| LM judge scores a patch highly while a required deterministic test fails | Completion fails. Judge evidence cannot bypass test evidence. |
| Same judge/model produces variable scores across five runs | Adapter records all runs; gate applies the predeclared aggregation and critical-case policy over fixed records only. |
| Dataset or rubric changes after a passing run | Digest mismatch makes the prior eval stale; re-evaluation is required. |
| Code changes after evidence was recorded | Revision/diff freshness gate fails until required checks run against the new subject. |
| Compile-only verification for a high-risk integration task | Verify-quality/coverage gate identifies missing integration, failure-path, and acceptance mappings. |
| Trace claims only declared files changed but git diff shows another file | Trajectory/scope evidence fails; worker self-report is not trusted as the authority. |
| Eval artifact is truncated, malformed, duplicated, or references an unknown case | Import/gate fails closed with stable, local findings. |
| Eval service or network is unavailable | External eval execution reports blocked/fail according to policy; deterministic `specd check` remains offline and does not downgrade. |
| A production incident reveals an unseen edge case | A redacted labelled case is versioned into the regression dataset and linked to a new corrective spec before the fix is certified. |

## Context-safety considerations

- Context packets should contain the **quality contract**, not all quality data: ids, critical-case summaries, rubric dimensions, thresholds, required commands, and content-addressed references.
- Large datasets, raw traces, verbose test output, and prior successful runs should be retrieved only on demand. Passing them by default wastes tokens and can anchor the model on obsolete behavior.
- Every item shown to the driver should be labelled `instruction`, `untrusted_input`, `expected_output`, or `evidence`. Evidence must display class, revision, digest, and freshness so prose cannot masquerade as proof.
- Do not store or request hidden chain-of-thought. Trajectory evaluation should use observable tool calls, sanitized arguments/results, file effects, and lifecycle events.
- Eval cases and production samples can contain prompt injection, secrets, personal data, or copyrighted material. Scan/redact before inclusion; keep sensitive bodies outside the manifest and use access-controlled references.
- Failure output must be bounded and summarized deterministically. Provide the smallest failing cases first; never dump an entire benchmark or session transcript into the agent context.
- Version role instructions, rubric summaries, and context-selection policy so a result can be reproduced without freezing an entire conversation.

## Non-goals and risks

- `specd` should not become an ML evaluation platform, host models, or acquire runtime SDK dependencies. It should define and validate portable contracts and delegate execution to operator-owned adapters.
- An LM judge is optional and never a trust anchor. It can be biased, variable, prompt-injected, or self-preferential. Critical correctness and security properties need deterministic checks or accountable human approval.
- Trajectory capture must not become surveillance or chain-of-thought retention. Store only the observable minimum needed for audit and policy.
- More gates can create false confidence if rubrics, datasets, or tests are shallow. Coverage mapping and ownership matter more than the number of checks.
- Statistical thresholds can hide critical failures inside an average. Critical-case rules must override aggregate scores.
- Backward compatibility must not silently label existing exit-zero evidence as full paper-level eval coverage. Reports should state “test evidence only” until additional classes are explicitly required and present.
- Evidence artifacts may grow quickly. Use bounded, content-addressed references and retention policies; keep gate inputs compact.
