# Action Prompt: specd Production Regression Planning

## Context

You are a coding agent tasked with implementing a comprehensive production regression test plan for the `specd` CLI tool. You have read `specd-production-regression-analysis-plan.md` (this analysis plan's companion file) and must now re-inspect the live repository, validate all material assumptions, and produce implementation-ready specs and tasks.

**Repository:** `https://github.com/0xkhdr/specd`  
**Task:** Create regression specs that cover every production-critical development area before v1.0.0 release.

## Agent Rules

1. **Read before writing.** Clone or inspect the repository before creating any files.
2. **Preserve repository conventions.** Use existing patterns for tests, naming, and structure.
3. **Validate each wave before advancing.** Do not proceed to Wave N+1 until Wave N tests pass.
4. **Add no orphaned code or undocumented configuration.** Every test must have a clear purpose documented in its spec.
5. **Include migrations, compatibility, observability, security, and rollback where relevant.**
6. **Measure optimization before and after.** Record baselines for any performance tests.
7. **Never hide uncertainty or silently diverge from the plan.** Document discrepancies.
8. **Split specs by coherent capability, not arbitrary file count.**
9. **Keep tasks comprehensive enough for implementation without requiring chat history.**

## Required Output Structure

Create the following under `specs/`:

```
specs/
├── progress.md
└── <spec-name>/
    ├── spec.md
    └── tasks.md
```

### Spec Names

Create these 15 regression specs:

| Spec ID | Directory Name | Responsibility |
|---------|---------------|----------------|
| S1 | `regression-cli-commands` | CLI command exit-code and output correctness |
| S2 | `regression-state-atomicity` | CAS, lock, and concurrent write safety |
| S3 | `regression-dag-scheduling` | Frontier, cycle detection, wave ordering |
| S4 | `regression-verify-sandbox` | Shell/bwrap/container execution and isolation |
| S5 | `regression-mcp-server` | stdio/HTTP/SSE protocol, auth, tool dispatch |
| S6 | `regression-onboarding` | `init` idempotency, packs, host detection |
| S7 | `regression-reporting` | Markdown/HTML/PR summary output correctness |
| S8 | `regression-install-integrity` | Checksum verification and script correctness |
| S9 | `regression-cross-platform` | Build verification on all target platforms |
| S10 | `regression-security-boundaries` | Path traversal, env scrubbing, auth, isolation |
| S11 | `regression-performance-baselines` | Performance regression prevention |
| S12 | `regression-coverage-floors` | Coverage threshold maintenance |
| S13 | `regression-ci-pipeline` | CI reliability and completeness |
| S14 | `regression-documentation-accuracy` | Doc accuracy against code behavior |
| S15 | `regression-fuzz-parsers` | Parser robustness against malformed input |

## Required Spec Content

Every `spec.md` MUST contain:

1. **Purpose and requirement coverage** — Which R# requirements this spec addresses
2. **Verified current state** — What the code currently does, with exact file paths and symbol references
3. **Proposed design and end-to-end flow** — How the regression tests will exercise the surface
4. **Interfaces, contracts, data, configuration, and dependencies** — What must remain stable
5. **Invariants, security, errors, observability, compatibility, and rollback** — What must not break
6. **Acceptance criteria and validation commands** — Exact commands to run and expected outcomes
7. **Open decisions and deviations from the analysis** — Any divergence from the analysis plan

Every `tasks.md` MUST contain:

1. **Dependency-aware waves** — Organize work into waves where Wave N+1 depends on Wave N
2. **Atomic, actionable checkboxes** — Each task is a single concrete action
3. **Likely files/modules** — Name files without inventing unverified paths
4. **Tests and validation in each wave** — Every wave ends with a validation command
5. **Setup/baseline, core changes, integration, regression, rollout, and cleanup** — Include all phases
6. **Dependencies, completion evidence, and rollback considerations** — State what each task needs and produces

`specs/progress.md` MUST track:

- Overall status and current wave
- Requirement-to-spec coverage (R1-R15 → S1-S15 mapping)
- Spec status, dependencies, blockers, and validation
- Baselines/targets for optimization work
- Decisions and deviations
- Completed and remaining waves

## Validation Requirements

Before declaring complete, verify:

1. **100% requirement coverage** — Every R1-R15 requirement is addressed by at least one spec
2. **All specs have tasks** — No spec without a `tasks.md`
3. **All tasks are actionable** — Every task can be executed by a developer reading it
4. **No invented paths** — Every referenced file path exists in the repository
5. **Validation commands are exact** — Every task's validation command can be copy-pasted and run
6. **Progress is tracked** — `specs/progress.md` accurately reflects status

## Discrepancy Handling

If you find that the analysis plan contains incorrect claims:

1. Record the discrepancy in the spec's "Open decisions and deviations" section
2. Resolve it from repository evidence, not the analysis plan
3. Update `specs/progress.md` with the deviation and rationale

## Stop Condition

Stop before production implementation (writing test code) unless the user separately requests execution. Your deliverable is the spec and task structure only.
