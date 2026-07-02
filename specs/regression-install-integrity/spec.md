# S8 — Install Integrity Regression

## 1. Purpose and requirement coverage

Guarantee the installer verifies checksums and fails closed on mismatch. Covers
**R8**.

## 2. Verified current state

- Installer: `scripts/install.sh`. `verify_checksum <dir> <archive> <version>`
  (`install.sh:48`) uses `sha256sum --ignore-missing -c SHA256SUMS`
  (`install.sh:60`), falls back to `shasum -a 256 -c -` (`install.sh:63`), and
  dies if neither tool is present (`install.sh:66`). A `--no-verify` flag skips
  verification with a warning (`install.sh:52`).
- Installer tests: `scripts/install_test.sh`; general script tests under
  `scripts/testdata/`.
- Lint gate: `make shellcheck` (`shellcheck scripts/*.sh`).
- Release checksums produced by GoReleaser (`.goreleaser.yml`) as `SHA256SUMS`.

## 3. Proposed design and end-to-end flow

Tests assert: a matching `SHA256SUMS` verifies successfully; a tampered archive
fails and the installer exits non-zero (fail closed); `--no-verify` skips with a
visible warning; missing both `sha256sum` and `shasum` aborts with a clear
message. Exercise via `scripts/install_test.sh` with fixture archives.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** `SHA256SUMS` filename + format; `--no-verify` flag; failure exit
  behavior.
- **Dependencies:** none (Wave-1 root).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Security:** checksum mismatch must fail closed — never install a tampered
  archive.
- **Compatibility:** POSIX-compliant shell; both `sha256sum` and `shasum` paths
  supported.
- **Rollback:** installer is idempotent per version; tests are additive.

## 6. Acceptance criteria and validation commands

- `shellcheck scripts/install.sh` clean.
- `bash scripts/install_test.sh` passes (match + tamper + no-verify cases).
- Tampered-archive case exits non-zero.

## 7. Open decisions and deviations

- Deviation F10: artifact signing is deferred (SBOM via `syft` on release
  runner); this spec covers checksum integrity only. Signing tracked separately.
