# Install Scripts Tasks

## Task Graph

| id | status | role | domain | dependencies | files | verify |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | pending | scout | script-audit | - | `scripts/README.md`, `.github/workflows/ci.yml`, `TESTING.md`, `AGENTS.md`, `CLAUDE.md` | `rg -n "scripts/|coverage-check|docs-lint|regress-|stress-|perf-gate|test-lint" --glob '!reference/**' .` |
| T2 | pending | craftsman | release-contract | T1 | `.goreleaser.yml`, `scripts/README.md`, `specs/install-scripts/spec.md` | `goreleaser check || test $? -eq 127` |
| T3 | pending | craftsman | installer | T2 | `scripts/install.sh`, `scripts/README.md` | `sh -n scripts/install.sh && shellcheck scripts/install.sh` |
| T4 | pending | craftsman | uninstaller | T3 | `scripts/uninstall.sh`, `scripts/README.md` | `sh -n scripts/uninstall.sh && shellcheck scripts/uninstall.sh` |
| T5 | pending | craftsman | installer-tests | T3,T4 | `scripts/install-test.sh`, `scripts/install.sh`, `scripts/uninstall.sh` | `./scripts/install-test.sh` |
| T6 | pending | craftsman | docs | T5 | `README.md`, `docs/user-guide.md`, `docs/README.md`, `TESTING.md`, `scripts/README.md` | `./scripts/docs-lint.sh && rg -n "curl .*specd|uninstall|SPECD_INSTALL_DIR" README.md docs scripts/README.md` |
| T7 | pending | validator | ci-verification | T6 | `scripts/*.sh`, `.github/workflows/ci.yml`, `go.mod` | `go test ./... -race -count=1 && go test ./... -count=2 && ./scripts/test-lint.sh && ./scripts/docs-lint.sh` |
| T8 | pending | auditor | final-audit | T7 | `.` | `git diff --name-only -- . ':!reference/**' && ! git diff --name-only -- reference` |

## T1 - Script Audit

Purpose: prove which scripts are live, removable, or redundant before deleting anything.

Acceptance:

- Search references across `.github/`, docs, root guidance files, and other scripts.
- Classify each live script as keep, remove, or replace.
- Remove only scripts with no live references and no unique coverage.
- Update `scripts/README.md` with the decision. If none qualify, explicitly record that no
  live scripts were removed.

Best practice:

- Treat CI and regression harnesses as owned behavior, not clutter.
- Do not touch `reference/`.

## T2 - Release Contract

Purpose: align installer URL logic with real GoReleaser artifact names and checksums.

Acceptance:

- Confirm archive name pattern for `linux` and `darwin` on `amd64` and `arm64`.
- Confirm `checksums.txt` contains archive checksums in a format usable by
  `sha256sum -c` or a portable fallback.
- Record any needed release config adjustment before installer implementation.

Best practice:

- Prefer adapting scripts to the existing release contract over changing release output.
- Keep releases reproducible and checksum-producing.

## T3 - Installer

Purpose: add `scripts/install.sh` for curl-based install and update.

Acceptance:

- Supports `--version`, `--install-dir`, `--update`, `--force`, `--dry-run`, and `--help`.
- Supports `SPECD_INSTALL_DIR`, `SPECD_VERSION`, and optional `GITHUB_TOKEN`.
- Detects supported OS/architecture pairs.
- Downloads archive and `checksums.txt` into a temporary directory.
- Verifies checksum before extraction and before replacing any binary.
- Replaces binary atomically within the target directory.
- Fails closed with clear messages for unsupported platforms, missing tools, failed
  checksum, unwritable directory, and existing binary conflicts.

Best practice:

- Use `set -eu`; avoid bash-only features unless the shebang is changed deliberately.
- Avoid executing downloaded content except the final installed binary for version checks.
- Clean temporary directories on exit.

## T4 - Uninstaller

Purpose: add `scripts/uninstall.sh` that safely removes installed binaries.

Acceptance:

- Supports `--install-dir`, `--dry-run`, and `--help`.
- Removes only `<install-dir>/specd`.
- Refuses dangerous paths such as empty install dir, `/`, or a missing basename.
- Does not remove `.specd/` project directories, configs, caches, or user specs.

Best practice:

- Make destructive behavior explicit and narrow.
- Print the exact path being removed.

## T5 - Installer Tests

Purpose: validate installer and uninstaller behavior without requiring network access.

Acceptance:

- Build a fixture archive containing a fake `specd` executable.
- Build a fixture `checksums.txt`.
- Exercise install, update, dry-run, checksum failure, unsupported platform, custom
  install dir, and uninstall.
- Tests run from a temporary directory and leave no repository artifacts.

Best practice:

- Mock downloads through local file URLs or script-internal override hooks.
- Keep tests deterministic and shell-only.

## T6 - Documentation

Purpose: make the curl lifecycle discoverable and safe to operate.

Acceptance:

- README quickstart shows curl install, source build fallback, update, and uninstall.
- User guide explains version pinning, custom install directories, checksum verification,
  and the fact that uninstall does not remove project `.specd/` data.
- Testing docs mention the installer test script and shellcheck coverage.
- `scripts/README.md` includes ownership for `install.sh`, `uninstall.sh`, and
  installer tests.

Best practice:

- Keep command examples copy-pasteable.
- Keep supply-chain claims precise and tied to checksum verification.

## T7 - CI Verification

Purpose: ensure the new scripts are covered by existing project gates.

Acceptance:

- Add installer shellcheck coverage if current CI glob does not already include it.
- Add installer test execution to CI only if it is deterministic and network-free.
- Run full Go race suite, order-dependence suite, script lints, and docs lint locally.

Best practice:

- CI should not depend on GitHub release availability or external network.
- Prefer fixture tests over live-release tests.

## T8 - Final Audit

Purpose: verify scope, safety, and project invariants.

Acceptance:

- Diff contains no `reference/` changes.
- Diff contains no new Go runtime dependencies.
- Installer lifecycle is documented and tested.
- Script removal decisions are justified.
- The final report lists any verification commands that were not run.

Best practice:

- Keep final review focused on operational safety and reproducibility.
