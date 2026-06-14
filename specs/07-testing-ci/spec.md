# Stage 07 — Testing & CI/CD

## Scope

Lock in every prior stage: close coverage gaps, enforce `-race`, guarantee
golden determinism, and harden the release/CI pipeline (`.github/workflows/`,
`.goreleaser.yml`, `Makefile`, `scripts/stress.sh`). Run last so it gates all
earlier work.

## Current state & findings

### F1 — [MEDIUM] No enforced coverage floor on critical functions
Prompt §8 says the project targets ~100% on critical functions but nothing
**enforces** it. The integrity-critical paths — `task.go` evidence gate,
`lock.go`, `state.go` CAS, `dag.go` — must not silently lose coverage in a
refactor.

**Intent:** add a CI step that runs `go test -coverprofile` and fails if total
(or per-package for `internal/core`) drops below a documented threshold (e.g.
85% overall, 95% for `internal/core`). Document the policy in `TESTING.md`.

### F2 — [MEDIUM] `-race` enforcement in CI
Verify `.github/workflows/` actually runs `go test -race ./...`. The concurrency
work (Stage 02) is only protected if `-race` runs on every PR. If the release
workflow is the only one, add a dedicated test/CI workflow triggered on PR +
push.

**Intent:** ensure a PR-triggered workflow runs `go vet`, `gofmt -l` (failing on
nonempty), `go test -race -coverprofile`, and `shellcheck` on `scripts/*.sh`.

### F3 — [MEDIUM] Multi-OS / multi-arch test matrix
Prompt §8 asks about OS/arch coverage. specd handles paths, locks (PID/epoch),
and self-update across Linux/macOS/Windows. `lock.go` uses Unix-friendly
`os.Rename`-over-running-binary semantics that **break on Windows**
(see Stage 01 F2). CI should at least build on all three and test on Linux +
macOS.

**Intent:** add a `strategy.matrix` over `{ubuntu, macos}` for tests and
`{ubuntu, macos, windows}` for build. Mark Windows self-update as known-limited
(documented), not silently broken.

### F4 — [LOW] Golden-file determinism
Prompt §8: report goldens must be deterministic. specd already abstracts time
via `core.Clock` + `testharness.FakeClock` (`state.go:109-116`) — good. Verify
**no** golden depends on map iteration order, real timestamps, absolute paths,
or git head. The DAG/boot stages (05) add goldens; ensure they sort before
emit.

**Intent:** add a `go test ./... -count=2` (or shuffle) CI run to catch
order-dependence; document the golden-update procedure in `TESTING.md`.

### F5 — [LOW] Release checksum/signing pipeline
Ties to Stage 01 F2/F3: `update.go` and `install.sh` will verify `SHA256SUMS`,
so the release pipeline **must produce** `SHA256SUMS`. Check `.goreleaser.yml`
emits a checksums file (goreleaser does by default as `checksums.txt` —
rename/align to `SHA256SUMS` or update Stage 01 to match the actual name).
Document the release process + verification in `docs/` and the README.

**Intent:** make the checksum filename consistent across `.goreleaser.yml`,
`update.go`, and `install.sh`. Optionally add cosign signing as a documented
follow-up (out of scope to implement).

### F6 — [LOW] `scripts/stress.sh` comprehensiveness
Prompt §8 asks if the concurrency stress test is comprehensive. Stage 02 adds
in-process tests; `stress.sh` should exercise **cross-process** contention
(multiple `specd task` invocations on one spec). Verify it asserts no lost
writes / no corrupt `state.json` and wire it into CI (or a nightly job).

**Intent:** ensure `stress.sh` spawns concurrent processes against one spec and
checks final revision == number of successful writes; run it in CI on Linux.

## Non-goals
- 100% coverage as a hard gate (brittle); a sensible floor instead.
- Implementing cosign signing now (document as follow-up).

## Acceptance criteria
1. PR-triggered CI runs `go vet`, `gofmt -l` (fail on output), `go test -race
   -coverprofile`, and `shellcheck`.
2. Coverage floor enforced (overall + `internal/core`), documented in
   `TESTING.md`.
3. Test matrix: Linux + macOS test, +Windows build; Windows update limitation
   documented.
4. `go test ./... -count=2` (or `-shuffle=on`) passes — no order dependence.
5. `SHA256SUMS` filename consistent across goreleaser / update / install.
6. `stress.sh` runs cross-process contention in CI and asserts integrity.
