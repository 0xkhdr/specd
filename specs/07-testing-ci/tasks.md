# Stage 07 — Tasks

Branch: `refactor/07-testing-ci`. Inspect the actual workflow files first:
`ls .github/workflows/` and read `.goreleaser.yml`, `Makefile`, `scripts/stress.sh`.

## T1 — PR CI workflow (F2)
**File:** `.github/workflows/ci.yml` (new, if only `release.yml` exists).

1. Trigger on `pull_request` and `push` to `main`.
2. Steps: checkout → setup-go (pin version from `go.mod`) →
   `go vet ./...` →
   `test -z "$(gofmt -l .)"` (fail if any unformatted) →
   `go test -race -coverprofile=cover.out ./...` →
   `go tool cover -func=cover.out` →
   `shellcheck scripts/*.sh`.

**Verify:** push branch, confirm workflow runs green.

## T2 — Coverage floor (F1)
**Files:** `.github/workflows/ci.yml`, `TESTING.md`.

1. After `cover.out`, add a gate script: parse `go tool cover -func` total;
   fail if `< 85`. Add a second per-package check for `internal/core` `< 95`
   (use `go test -cover ./internal/core/`).
2. Document thresholds + how to run coverage locally in `TESTING.md`.

**Verify:** `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1`

## T3 — OS/arch matrix (F3)
**File:** `.github/workflows/ci.yml`.

1. `strategy.matrix.os: [ubuntu-latest, macos-latest]` for the test job.
2. Separate build job: `[ubuntu-latest, macos-latest, windows-latest]` running
   `go build ./...` (no `-race` needed on build-only).
3. Document the Windows self-update limitation (rename over running binary) in
   `docs/` and `update.go` doc comment.

**Verify:** matrix runs green; Windows build passes.

## T4 — Order-dependence guard (F4)
**Files:** `.github/workflows/ci.yml`, `TESTING.md`.

1. Add `go test -shuffle=on ./...` (Go ≥1.17) as a CI step, or `-count=2`.
2. Fix any test that fails under shuffle (usually map-order or shared-state
   leak). Stage 05 goldens must sort before emit.
3. Document golden-update procedure (`UPDATE_GOLDEN=1 go test ...` or whatever
   the harness uses) in `TESTING.md`.

**Verify:** `go test -shuffle=on ./...`

## T5 — Checksum filename alignment (F5)
**Files:** `.goreleaser.yml`, `internal/cmd/update.go` (Stage 01), `scripts/install.sh` (Stage 01).

1. Read `.goreleaser.yml` `checksum:` block. Note the produced filename
   (default `checksums.txt`).
2. Pick one canonical name (`SHA256SUMS` recommended) and make goreleaser emit
   it (`checksum: { name_template: "SHA256SUMS" }`).
3. Ensure Stage 01 `fetchChecksums` and `install.sh` reference the same name.
   If Stage 01 already shipped, this task reconciles them.

**Verify:** `goreleaser check` (if installed) + grep the three files for the same filename.

## T6 — Cross-process stress in CI (F6)
**Files:** `scripts/stress.sh`, `.github/workflows/ci.yml`.

1. Ensure `stress.sh` spawns N concurrent `specd task` processes against one
   spec and asserts final `state.json` revision == successful writes and the
   file is valid JSON (no corruption).
2. Add a CI step (Linux only) running `bash scripts/stress.sh`.

**Verify:** `bash scripts/stress.sh` locally, then in CI.

## Done-when
- PR CI green with vet + gofmt + race + coverage floor + shellcheck + shuffle +
  stress.
- OS matrix builds; Windows limitation documented.
- Checksum filename consistent across release/update/install.
- `TESTING.md` documents coverage policy + golden update + stress.
