# Design — deep-review-phase1-deletion

- references: R1, R1.1, R1.2, R2, R2.1, R2.2, R3, R3.1, R4, R4.1
disposition: accepted
owner: 0xkhdr

## Boundaries

- Owned: `internal/orchestration/a2a.go` (+ test), `internal/adapter/{runner,feedback,identity,a2a}.go` (+ tests), the `triage` palette entry and dispatch stub, docs rows for deleted surfaces, stray `coverage.out`.
- Excluded: every reachable verb, the gates registry, state/lock/CAS machinery, tasks parser, remaining adapter symbols (`Adapter`, `Request`, `SchemaVersion`, `MissionFromRequest`, `ExportOTel`).

## Interfaces

- CLI surface shrinks by one deferred verb (`triage` → unknown verb, exit 2). No other CLI contract changes.
- `internal/adapter` public (package-internal) API shrinks to the symbols production calls today.

## Invariants

- Determinism, evidence integrity, atomic writes/CAS/lock, byte-stable parser, zero runtime deps, fail-closed dispatch — all preserved; deletions only remove unreferenced code.

## Failure

- A deletion that breaks compilation or tests is caught by the per-commit gate set; the commit is rejected, not patched around.

## Integration

- `docs/command-reference.md` and `docs/CHEATSHEET.md` must both drop the `triage` rows in the same commit (docs-lint enforces byte equality until phase 2 lands).
- CI `go mod tidy` diff check unaffected (no deps touched).

## Alternatives

- Keep A2A/adapter machinery behind an issue with a driving verb and deadline — rejected for A2A (no consumer possible: `internal/` unimportable), allowed per R2.2 only with a named verb and deadline.
- Ship `triage` instead of cutting — rejected: no implementation exists and no user demands it (subtractive bias).

## Verification

- Full suite `go test ./... -race -count=1`, `gofmt -l .` empty, `go vet`, `./scripts/test-lint.sh`, `./scripts/docs-lint.sh`, `./scripts/regress-domains.sh` green after each deletion commit.
- `specd triage` exits 2 after R3.1.

## Deployment

- Each deletion is its own commit on `optimization`; merge via normal PR gates. No runtime rollout — single static binary.

## Rollback

- Pure deletions: revert the individual commit restores the surface byte-identically.
