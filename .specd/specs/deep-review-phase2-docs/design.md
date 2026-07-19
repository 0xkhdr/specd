# Design — deep-review-phase2-docs

references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2
disposition: accepted
owner: 0xkhdr

## Boundaries

- Owned: new `tools/gendocs` (small stdlib-only Go program), `docs/command-reference.md` (becomes generated output), `docs/CHEATSHEET.md` (deleted), `scripts/docs-lint.sh` (repointed), CI docs step, CI gate list in `CLAUDE.md`/`CONTRIBUTING.md`, new `scripts/ci-local.sh`.
- Excluded: the palette itself (`internal/core/commands.go`), all other docs.

## Interfaces

- `go run ./tools/gendocs` writes `docs/command-reference.md`; `go run ./tools/gendocs -check` exits non-zero on drift.
- `scripts/docs-lint.sh` becomes: run generator in check mode (replaces byte-copy comparison of two files).

## Invariants

- Zero runtime dependencies: `tools/gendocs` uses stdlib only and lives outside the shipped binary's import graph.
- Determinism: generator output is a pure function of the palette; sorted iteration, no timestamps.

## Failure

- Generator drift → docs-lint fails CI, same failure surface as today, but pointing at the palette instead of a manual copy step.

## Integration

- CI docs step swaps `docs-lint.sh` implementation; no workflow structure change.
- `CLAUDE.md` docs-sync rule updates from "edit both files" to "regenerate".

## Alternatives

- `specd docs` verb instead of `tools/gendocs` — rejected for now: adds a verb to a palette the review says to shrink; a build-time tool needs no palette entry.
- Keep CHEATSHEET as generated second output — deferred; single file preferred (subtractive bias).

## Verification

- `./scripts/docs-lint.sh` green; running the generator twice produces byte-identical output; deleting a palette flag makes the lint fail.

## Deployment

- Lands on `optimization` branch; contributors regenerate instead of hand-copying.

## Rollback

- Revert restores CHEATSHEET and the byte-copy lint; no state migration.
