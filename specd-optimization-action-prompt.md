# Action Prompt: specd Production-Grade Optimization Review

## Context

You are a coding agent assigned to optimize the `specd` repository (https://github.com/0xkhdr/specd) for production-grade quality across performance, code quality, readability, security, and operational excellence.

**Read before proceeding:**
1. Repository instructions: `AGENTS.md`, `TESTING.md`, `CONTRIBUTING.md`
2. Analysis plan: `specd-optimization-analysis-plan.md` (this document's companion)
3. Re-inspect the live repository and validate all material assumptions

## Agent Rules

1. **Read before writing.** Inspect every file you plan to modify. Never invent paths, symbols, or behavior.
2. **Preserve conventions.** Follow existing Go idioms, naming, and project structure.
3. **Validate each wave.** Run `make test` (and relevant targets) after every wave. Do not advance on failure.
4. **No orphaned code.** Every function must be called; every config must be documented; every test must assert.
5. **Include migrations, compatibility, observability, security, and rollback** where relevant.
6. **Measure before and after.** Record benchmark baselines before performance changes; reject regressions &gt;5%.
7. **Never hide uncertainty.** If an assumption from the analysis plan is wrong, record the discrepancy and resolve from evidence.
8. **Split specs by capability.** Do not split arbitrarily by file count. Each spec owns a coherent vertical.
9. **Keep tasks comprehensive.** Tasks must be implementation-ready without requiring chat history.

## Validation Requirement

Before creating specs, you must:
1. Re-inspect all `internal/` packages and validate the analysis plan's findings.
2. Record discrepancies between the analysis plan and live repository.
3. Build your own implementation vision based on live evidence.
4. Only then create specs and tasks.

## Discrepancy Log

Create a `specs/discrepancies.md` file if any analysis plan claims are incorrect. Include:
- Claim from analysis plan
- Actual observed state
- Impact on scope or design
- Resolution

## Spec Structure

Create this directory structure:

```text
specs/
├── progress.md
├── security-hardening/
│   ├── spec.md
│   └── tasks.md
├── performance-optimization/
│   ├── spec.md
│   └── tasks.md
├── code-quality-readability/
│   ├── spec.md
│   └── tasks.md
├── testing-reliability/
│   ├── spec.md
│   └── tasks.md
├── observability/
│   ├── spec.md
│   └── tasks.md
├── cicd-build-hardening/
│   ├── spec.md
│   └── tasks.md
└── documentation-hygiene/
    ├── spec.md
    └── tasks.md
