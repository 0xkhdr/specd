# Requirements — deep-review-closure-v2

> Source: `DEEP-REVIEW.md`, `DEEP-REVIEW-PHASE6.md`, the 2026-07-20 audit, and the failed `deep-review-closure` planning gate.

## R1 — Documentation remains coherent after subtractive changes

owner: 0xkhdr
priority: must
risk: medium

- R1.1: When a documented file or command surface is removed, the system shall contain no live documentation link or instruction that names the removed surface.
- R1.2: When a retired documentation or script path is reintroduced in live documentation, the documentation gate shall fail with the offending path.
- R1.3: When documentation states the number of registered core gates, the documentation gate shall reject every count that differs from the registry.

## R2 — CI tiers match their approved contract

owner: 0xkhdr
priority: must
risk: medium

- R2.1: When a pull request or non-main branch push occurs, the system shall run the fast CI tier without merge-only stress, regression, performance, or release validation.
- R2.2: When `main` changes or the scheduled validation time arrives, the system shall run the heavy test, regression, stress, performance, install, and coverage gates.
- R2.3: When a release tag is built, the system shall run production smoke and cross-platform release construction in the release workflow rather than duplicate cross-compilation in the pull-request tier.
- R2.4: When the local CI mirror reports success, the system shall have run every required local fast-tier tool and shall never silently skip a missing tool.

## R3 — Local checks preserve repository hygiene and stable interfaces

owner: 0xkhdr
priority: should
risk: low

- R3.1: When the coverage-floor check runs locally, the system shall leave no coverage artifact in the repository root on success or failure.
- R3.2: When the stress harness is invoked without a domain, the system shall execute the default domain.
- R3.3: When the stress harness receives an unknown domain, the system shall exit 1 and print the valid domains.

## R4 — Closure work remains evidence-honest

owner: 0xkhdr
priority: must
risk: medium

- R4.1: When prior deep-review commits lack a dedicated task-evidence trail, the system shall record that traceability gap without manufacturing retrospective verification evidence.
- R4.2: When implementation begins for this closure, the system shall use task-scoped files, current-HEAD verification evidence, and gated task completion.
- R4.3: When an action is human-only, the system shall leave that action to the human owner and report the exact pending gate.

## R5 — Generated planning artifacts match their parsers

owner: 0xkhdr
priority: must
risk: high

- R5.1: When `specd new` generates a design scaffold, the system shall emit requirement-reference syntax accepted by the canonical design parser.
- R5.2: When an approved generated design declares requirement references, the coverage gate shall recognize those references during task approval.
- R5.3: When scaffold and parser syntax diverge, an integration test shall fail before a user can approve an untraceable design.

## Edge and failure behavior

- Historical review documents may name removed surfaces when clearly describing past state; live operator and contributor documentation may not.
- Missing local static-analysis tools fail with installation guidance rather than a successful skip.
- CI workflow redistribution shall not remove any existing validation lane from the union of fast, heavy, and release workflows.
- A failed local check shall clean up temporary coverage output before returning non-zero.
- Existing approved artifacts retain their recorded digests; the implementation shall not rewrite `deep-review-closure` or manufacture approval records.

## Non-goals

- No runtime CLI, state schema, DAG, evidence, lock, CAS, or parser behavior changes beyond making the generated scaffold conform to the existing design parser.
- No new dependency or documentation framework.
- No retroactive evidence records for already-landed commits.
- No implementation of the deferred exception rename or the `recurring`/`spike` deletion before its 2026-10-19 review date.
