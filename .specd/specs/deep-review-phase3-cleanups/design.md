# Design — deep-review-phase3-cleanups

references: R1, R1.1, R1.2, R2, R2.1, R2.2
disposition: accepted
owner: 0xkhdr

## Boundaries

- Owned: the four `contains*` helper sites, the three `sortedKeys` copies, `internal/cmd/registry.go` layout and the per-verb files receiving moved handlers.
- Excluded: handler behavior, exported APIs, the gates registry, anything outside `internal/cmd` and the named core files.

## Interfaces

- No CLI or package API change. Go 1.26 `slices`/`maps` are stdlib — zero-dependency invariant untouched.

## Invariants

- Byte-stable outputs: `sortedKeys` replacements must preserve identical ordering (lexicographic — `slices.Sorted` matches).
- Determinism of reports (prometheus.go, intake.go) unchanged.

## Failure

- Any ordering regression is caught by existing golden/byte-stability tests and `-count=2` order-dependence runs.

## Integration

- Pure-move registry split keeps `go vet`, staticcheck, and test-lint green; no import cycles (handlers already live in package cmd).

## Alternatives

- Full `sort` → `slices` sweep across 63 files — deferred: churn without payoff beyond the drop-in sites (non-goal).
- Splitting registry.go into subpackages — rejected: package cmd is fine; only file layout is dishonest today.

## Verification

- `grep` proves zero remaining `func contains`/`sortedKeys` definitions; full suite `-race -count=1` plus `-count=2` green; `registry.go` line count drops to map + plumbing (< ~500 lines).

## Deployment

- Two commits: stdlib swap, then registry split. Each with full gate set green.

## Rollback

- Mechanical changes; revert individual commit restores prior layout.
