# Design — deep-review-closure-v2

- references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2, R2.3, R2.4, R3, R3.1, R3.2, R3.3, R4, R4.1, R4.2, R4.3, R5, R5.1, R5.2, R5.3
- boundaries: live documentation, CI workflows, local checks, and generated planning scaffold syntax
- interfaces: workflow triggers, ci-local exit contract, stress arguments, coverage cleanup, and design scaffold metadata
- invariants: validation coverage, evidence integrity, zero dependencies, and parser/scaffold agreement
- failure: drift fails before implementation or a successful local-CI claim
- integration: reuse docs lint, workflow integration tests, ParseDesign, and the specd task loop
- alternatives: no new parser, YAML library, Markdown checker, or retrospective evidence
- disposition: accepted
- owner: 0xkhdr

## Boundaries

- Documentation closure owns `README.md`, `AGENTS.md`, `TESTING.md`, `docs/README.md`, `docs/contributor-guide.md`, `docs/observability.md`, `scripts/README.md`, and `scripts/docs-lint.sh`.
- CI closure owns `.github/workflows/{ci,heavy,release}.yml`, `scripts/ci-local.sh`, `CONTRIBUTING.md`, and the existing workflow integration test.
- Local hygiene owns `scripts/coverage-check.sh` and `scripts/stress.sh`.
- Scaffold closure owns `designStub` in `internal/cmd/lifecycle.go` and its focused test in `internal/cmd/lifecycle_test.go`; `ParseDesign` remains unchanged.
- Traceability closure owns a factual note in `DEEP-REVIEW-PHASE6.md` and this successor spec. It does not alter the approved `deep-review-closure` artifacts.
- Excluded: runtime state schema, DAG, evidence semantics, lock/CAS/parser behavior, and deferred Phase 6 decisions.

## Interfaces

- Fast workflow: pull requests and non-main pushes; parallel lint, analysis, race-test, coverage, and portable build lanes.
- Heavy workflow: main pushes plus a nightly schedule; order-dependence, domain regression, stress, performance, install, and coverage lanes.
- Release workflow: tag verification, production lifecycle smoke, and GoReleaser cross-platform artifacts.
- `scripts/ci-local.sh`: Linux fast-tier mirror; missing `golangci-lint`, `govulncheck`, or `shellcheck` fails with installation guidance.
- `scripts/stress.sh [domain]`: omitted domain selects `default`; invalid domain prints the closed set and exits 1.
- `scripts/coverage-check.sh`: temporary coverage profile outside the repository, removed by an exit trap.
- `designStub`: emits `- references:`, `- disposition:`, and `- owner:` metadata accepted by `ParseDesign`.

## Invariants

- The union of fast, heavy, and release workflows retains every existing validation lane.
- PR CI contains no merge-only stress/regression/performance work and no redundant `GOOS` cross-compilation.
- Live documentation contains neither `CHEATSHEET.md` nor deleted `stress-*.sh` paths, and every stated core-gate count equals the registry count.
- Documentation and workflow checks add no dependency.
- Coverage cleanup occurs on success and failure.
- A generated design's references survive `ParseDesign` and coverage analysis.
- No approval, criterion, verification, completion, or decision evidence is fabricated for commits predating this spec.

## Failure

- A retired path or wrong gate count fails `docs-lint.sh` with the offending line.
- A missing local CI prerequisite fails before the script can claim success.
- An invalid stress domain returns status 1 without building or mutating a workspace.
- Workflow contract drift fails the existing integration test scanning `.github/workflows`.
- Scaffold/parser drift fails a focused `specd new` regression test.
- Task verification failures backpropagate before retry; human-only approvals remain external blockers.

## Integration

- Extend `internal/integration/production_smoke_test.go` rather than add a YAML parser.
- Keep `go run ./tools/gendocs -check` as the command-reference source-of-truth check.
- GoReleaser remains the cross-platform artifact builder; redundant PR `GOOS` probes are deleted.
- Keep the canonical parser strict and make generated text conform to it.
- `deep-review-closure-v2` maintains `deep-review-phase5-decisions`; the blocked predecessor remains visible as diagnostic history.

## Alternatives

- Add Markdown/YAML parsing dependencies — rejected; known-path grep checks and existing workflow contract tests are sufficient.
- Broaden `ParseDesign` to accept the erroneous scaffold — rejected; one canonical bullet syntax already exists and avoids ambiguous prose matches.
- Keep best-effort tool skips — rejected because success would not prove declared gates ran.
- Rewrite approved predecessor artifacts or recreate evidence — rejected as evidence-integrity violations.

## Verification

- Scaffold: focused lifecycle test creates a spec and asserts `ParseDesign` reads generated references.
- Documentation: `./scripts/docs-lint.sh` plus absence checks for retired live paths.
- CI: integration tests assert triggers and lane placement; shellcheck validates scripts.
- Hygiene: coverage leaves no root artifact; default and invalid stress invocations match the interface.
- Final: formatting, build, vet, race suite, count-two suite, lint gates, domain regressions, all stress domains, performance, and install-script tests.
- Harness: current-HEAD verify evidence precedes every task completion; `specd check deep-review-closure-v2` stays clean.

## Deployment

- Atomic tasks: scaffold regression, docs, CI, local scripts, final validation.
- Human owner advances lifecycle gates; the agent implements and records task evidence only.

## Rollback

- Revert an individual task diff; no data migration exists.
- If CI redistribution misbehaves, restore previous workflows while retaining scaffold, documentation, and local hygiene fixes.
