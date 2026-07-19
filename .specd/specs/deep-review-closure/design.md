# Design — deep-review-closure

references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2, R2.3, R2.4, R3, R3.1, R3.2, R3.3, R4, R4.1, R4.2, R4.3
disposition: accepted
owner: 0xkhdr

## Boundaries

- Documentation closure owns the live indexes and contributor/operator instructions that still name deleted surfaces: `README.md`, `AGENTS.md`, `TESTING.md`, `docs/README.md`, `docs/contributor-guide.md`, `docs/observability.md`, and `scripts/README.md`.
- Documentation enforcement owns `scripts/docs-lint.sh`; historical review prose remains historical and is not rewritten to hide past findings.
- CI closure owns `.github/workflows/{ci,heavy,release}.yml`, `scripts/ci-local.sh`, and their existing integration contract test.
- Local hygiene owns `scripts/coverage-check.sh` and `scripts/stress.sh`.
- Traceability closure owns a factual note in `DEEP-REVIEW-PHASE6.md` and this successor spec. It does not create retroactive evidence.
- Excluded: runtime CLI behavior, state schema, gate registry, orchestration, adapters, evidence semantics, and deferred Phase 6 decisions.

## Interfaces

- Fast workflow: pull requests and non-main pushes; parallel lint, analysis, race-test, coverage, and portable build lanes.
- Heavy workflow: main pushes plus a nightly schedule; order-dependence, domain regression, stress, performance, install, and coverage lanes.
- Release workflow: tag-triggered release verification, production lifecycle smoke, and GoReleaser cross-platform artifacts.
- `scripts/ci-local.sh`: Linux fast-tier mirror; missing `golangci-lint`, `govulncheck`, or `shellcheck` is a failure with installation guidance.
- `scripts/stress.sh [domain]`: omitted domain selects `default`; invalid domain prints the closed set and exits 1.
- `scripts/coverage-check.sh`: temporary coverage profile outside the repository, removed by an exit trap.

## Invariants

- The union of fast, heavy, and release workflows retains every existing validation lane.
- PR CI contains no merge-only stress/regression/performance work and no redundant `GOOS` cross-compilation.
- Documentation generation remains a pure function of the command palette; the added drift checks use existing shell tools and add no dependency.
- Live documentation contains neither `CHEATSHEET.md` nor the five deleted `stress-*.sh` paths, and every stated core-gate count equals the registry count.
- Coverage cleanup occurs on both success and failure.
- No approval, criterion, verification, completion, or decision evidence is fabricated for commits predating this spec.

## Failure

- A retired path or wrong gate count in live documentation fails `docs-lint.sh` and prints the offending line.
- A missing local CI prerequisite fails before the script can claim success.
- An invalid stress domain returns usage status 1 without building or mutating a workspace.
- Workflow contract drift fails the existing integration test that scans `.github/workflows`.
- Any task verification failure is backpropagated before retry; human-only approvals remain external blockers.

## Integration

- Reuse `internal/integration/production_smoke_test.go` for CI lane assertions instead of adding a workflow parser or dependency.
- Keep `go run ./tools/gendocs -check` as the command-reference source-of-truth check.
- Keep the current release build mechanism: GoReleaser performs cross-platform artifact construction, so PR-only `GOOS` probes are removed rather than duplicated elsewhere.
- `deep-review-closure` is linked to `deep-review-phase5-decisions` with kind `maintains`.

## Alternatives

- Add a Markdown link-checking dependency — rejected; repository-wide checks for the known retired paths close this regression with existing tools.
- Parse GitHub Actions YAML in production code — rejected; the existing text contract test is sufficient for repository-owned static workflows.
- Keep best-effort skips in `ci-local.sh` — rejected because success would not mean the declared gates ran.
- Recreate evidence for prior commits — rejected as an evidence-integrity violation.

## Verification

- Documentation: `./scripts/docs-lint.sh` plus repository-wide absence checks for retired live paths.
- CI: focused integration tests assert triggers and lane placement; `shellcheck -S error scripts/*.sh` validates scripts.
- Hygiene: `./scripts/coverage-check.sh && test ! -e coverage.out`; default and invalid stress invocations assert their documented behavior.
- Final: formatting, build, vet, race suite, count-two suite, both lint gates, domain regressions, all stress domains, performance gate, and install-script tests.
- Harness: each task receives current-HEAD verify evidence before `complete-task`; `specd check deep-review-closure` remains clean.

## Deployment

- Land as one successor spec with atomic tasks: docs, CI, local scripts, then validation.
- Human owner advances requirements, design, tasks, and final lifecycle gates; the agent only implements and records task evidence.

## Rollback

- Revert the individual task diff. No data migration or state rollback exists because runtime formats are unchanged.
- If CI trigger redistribution misbehaves, restore the previous workflow files while retaining the documentation and local hygiene fixes.
