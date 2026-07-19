# Design — deep-review-phase4-ci

- references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2
disposition: accepted
owner: 0xkhdr

## Boundaries

- Owned: `.github/workflows/ci.yml` (becomes the fast PR tier), a new `.github/workflows/heavy.yml` (main-branch/nightly tier), `scripts/stress.sh` and the five `stress-*.sh` variants it absorbs.
- Excluded: the release workflow's content (only confirmed to already own cross-compiles/smoke), the substance of any gate.

## Interfaces

- PR tier triggers on `pull_request` + non-main `push`; heavy tier on `push` to `main` + `schedule` (nightly).
- `scripts/stress.sh <domain>`; no argument = current default behavior; unknown argument = usage + exit 1.

## Invariants

- Total validation coverage unchanged: every check that runs today still runs in some tier before or at merge.
- Fail-closed: heavy-tier failure marks the `main` commit red.

## Failure

- A check accidentally dropped from both tiers — prevented by a checklist diff of old ci.yml steps against the union of new workflows during review.

## Integration

- `scripts/ci-local.sh` (deep-review-phase2-docs T3) mirrors the PR tier; regress-domains and stress remain callable standalone.

## Alternatives

- Matrix strategy inside one workflow with `if: github.ref` guards — rejected: two small workflows read clearer than conditional steps.
- Keeping six stress scripts — rejected: shared boilerplate ×6 is the maintenance cost the review flags.

## Verification

- `actionlint` (or YAML parse) on both workflows; grep proves ci.yml contains no stress/perf-gate/count=2 steps and heavy.yml contains them all; `stress.sh <each domain>` exits 0, unknown domain exits 1.

## Deployment

- Land workflows first, observe one green PR + one green main run, then delete the five old stress scripts.

## Rollback

- Revert the workflow commit; old ci.yml is fully self-contained.
