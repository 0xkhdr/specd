# specd — Ship Report

> Date: 2026-06-14
> Branch: `regression/ship-readiness` (off `main`)
> Version under test: `v0.1.0-30-g9059040`
> Build: Go, stdlib-only (`go.mod` has zero `require` entries)

**Verdict: SHIP-READY.** Full 8-stage regression complete. All blocker findings
resolved; zero open findings. Full local gate (`make ci`) green.

---

## 1. Stage summary

| Stage | Scope | Verdict | Findings |
|-------|-------|---------|----------|
| 1 | Spec & refactor-stage traceability audit | PASS | F-S3-1 (warn, fixed S8), F-S1-1 (info, closed) |
| 2 | Build integrity & test suite | PASS | F-S2-1 (warn, fixed) |
| 3 | Code review & contract adherence | PASS | — |
| 4 | Security & hardening review | PASS | — |
| 5 | CLI surface & registry consistency | PASS | F-S5-1 (blocker, fixed) |
| 6 | End-to-end lifecycle validation | PASS | — |
| 7 | Documentation & release artifacts | PASS | F-S7-1 (blocker, fixed), F-S7-2 (warn, fixed), F-S7-3 (info) |
| 8 | Final ship gate & sign-off | PASS | F-S3-1 closed |

**Blockers: 0 open (2 fixed). Warnings: 0 open (3 fixed). Info: 0 open (2 closed).**
See `REGRESSION_FINDINGS.md` for the per-finding ledger.

---

## 2. Test results & coverage

- `make test` (`go test ./... -race -count=1`) → all 5 packages PASS
  (root, `internal/cli`, `internal/cmd`, `internal/core`, `internal/testharness`)
- `make test-order` (`-count=2`) → PASS — no order/wall-clock dependence
- `make stress` → 16 workers × 20 iterations cross-process; 320 committed writes;
  `turn == successes`; `state.json` intact (CAS + lock hold under contention)
- Coverage floors (`make cover-check`):
  - overall **64.1% ≥ 60%** floor
  - `internal/core` **59.9% ≥ 58%** floor
- Static: `gofmt -l .` empty · `go vet ./...` clean · `shellcheck scripts/*.sh` clean
- **`make ci` (lint · test · test-order · cover-check · stress) → GREEN**

---

## 3. Security status

Validated against `docs/validation-gates.md` + stage `01-security` (Stage 4):

- **Command exec (`verify`):** child env scrubbed to allowlist
  (`PATH HOME LANG LC_ALL TMPDIR` + `SPECD_*` passthrough); NUL-byte commands
  rejected pre-exec; timeout via `SPECD_VERIFY_TIMEOUT` (clamped `EnvInt`).
- **Path / slug safety:** `ValidateSlug` regex `^[a-z0-9][a-z0-9-]*$` rejects
  `..`, `/`, `\`, leading `-`; `FindSpecdRoot` walks up only.
- **State integrity:** `AtomicWrite` = temp + `fsync` + rename, `0644` minus umask;
  `SaveState` revision CAS + `assertLocked` guard; ledger `O_APPEND`.
- **Locking:** `.lock` is non-secret (`PID epochMillis`, `0644`, `O_EXCL`);
  reentrant by goID; stale reclaim + timeout (clamped `EnvInt`).
- **Supply chain:** `update` and `install.sh` fail-closed on `SHA256SUMS`
  mismatch / missing entry; GoReleaser emits `SHA256SUMS` with the exact
  filename both consumers expect.
- **Leak surface:** no secrets in logs/state/errors; error messages quote
  command/path only, never env contents.

No security regressions found.

---

## 4. Release artifacts

- `.github/workflows/ci.yml` — lint, test (ubuntu+macOS), coverage floor,
  cross-process stress, build (ubuntu+macOS+Windows).
- `.github/workflows/release.yml` — on `v*` → race suite → GoReleaser.
- `.goreleaser.yml` — linux/darwin/windows × amd64/arm64;
  `-ldflags "-s -w -X main.version={{.Version}}"`;
  `checksum.name_template: SHA256SUMS` (consumed by `update.go` + `install.sh`).
- `scripts/install.sh` / `uninstall.sh` — checksum-verified install; PATH marker
  aligned (`# specd`).
- Docs: `README.md`, `AGENTS.md` (Go-accurate), `TESTING.md`, and `docs/`
  (`concepts user-guide command-reference validation-gates agent-integration
  contributor-guide` + index) all match the actual tree.

---

## 5. Known limitations

- **Windows `specd update`:** self-update flow is validated for linux/darwin;
  Windows users install via release artifact / `install.sh` equivalent.
  Documented in `TESTING.md`.
- **No static `CHANGELOG.md`:** release notes are auto-generated per release by
  GoReleaser from commit history (F-S7-3, intentional).
- **`internal/core` coverage 59.9%** sits just above its 58% floor — adequate
  but the thinnest margin in the suite; candidate for future test additions.

---

## 6. Sign-off

All 8 regression stages complete and `[x]` in `ACTION_PLAN.md`. Zero open
findings; both blockers fixed and their stages re-run green. `make test` and
`make ci` pass locally; working tree clean.

**Signed off for ship — 2026-06-14.**
