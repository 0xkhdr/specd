# Install Scripts Spec

## Context

`specd` is a Go CLI distributed as a single static binary. The repository currently has
active CI, lint, regression, coverage, stress, and release scripts under `scripts/`.
The live release pipeline already builds archives and publishes `checksums.txt` through
GoReleaser. The documentation currently emphasizes source builds and does not define a
curl-based install, update, or uninstall lifecycle.

The `reference/` tree is frozen historical material and is out of scope. Do not import,
copy from, or edit it.

## Script Inventory Finding

Current live scripts are:

- `scripts/coverage-check.sh`
- `scripts/docs-lint.sh`
- `scripts/perf-gate.sh`
- `scripts/regress-all.sh`
- `scripts/regress-domains.sh`
- `scripts/regress-lint.sh`
- `scripts/stress-acp.sh`
- `scripts/stress-brain-recovery.sh`
- `scripts/stress-checkpoint-fault.sh`
- `scripts/stress-orchestration.sh`
- `scripts/stress-program.sh`
- `scripts/stress.sh`
- `scripts/test-lint.sh`

These scripts are referenced by CI, testing docs, contributor docs, regression harnesses,
or the script decision log. The initial cleanup posture is therefore conservative: no
currently live script is presumed removable until a reference audit proves it is unused or
duplicative. The existing changelog already records earlier removal of `stress-brain.sh`
and `verify-progress.sh`.

## G - Goals

G1. Provide a curl-friendly install flow for released `specd` binaries.

G2. Provide an update flow that is idempotent and uses the same safety checks as install.

G3. Provide an uninstall flow that removes only files owned by the installer.

G4. Remove unneeded live scripts only when an audit proves they are unreferenced or
duplicative.

G5. Keep the project aligned with its current constraints: standard-library Go runtime,
deterministic gates, evidence integrity, no dependency drift, and no changes to
`reference/`.

## C - Constraints

C1. Installer scripts SHALL be POSIX `sh` compatible unless a documented requirement
forces `bash`.

C2. Installer scripts SHALL use only common base tools: `sh`, `uname`, `mktemp`, `curl`,
`tar`, `install` or `cp`/`chmod`, `sha256sum` or `shasum`, and `rm`.

C3. Installer scripts SHALL verify downloaded release artifacts against the published
GoReleaser `checksums.txt` before replacing an existing binary.

C4. Installer scripts SHALL install atomically by downloading and verifying in a temporary
directory, then replacing the target binary only after verification passes.

C5. Installer scripts SHALL default to installing `specd` into `/usr/local/bin` when
writable or when run through `sudo`, with `SPECD_INSTALL_DIR` support documented for
custom locations.

C6. Installer scripts SHALL support explicit version selection and latest-release
selection. Version selection must accept tags with or without a leading `v`.

C7. Installer scripts SHALL detect `linux` and `darwin` on `amd64` and `arm64`, matching
the release archive matrix. Unsupported platforms fail closed with a clear message.

C8. Uninstall SHALL remove only the `specd` binary from the chosen install directory and
SHALL NOT remove user project `.specd/` directories.

C9. Script deletion SHALL be backed by a repository reference audit across docs, CI, and
shell scripts. If no scripts qualify, the implementation records that decision in
`scripts/README.md`.

C10. Any new or modified shell script SHALL pass shellcheck in CI or have a narrow,
documented inline exception.

## I - Interfaces

I1. Public install command:

```sh
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh
```

I2. Public update command:

```sh
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | sh -s -- --update
```

I3. Public uninstall command:

```sh
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | sh
```

I4. Installer options:

```text
--version <version>     install a specific release tag
--install-dir <path>    override install directory
--update                replace an existing specd binary after verification
--force                 allow replacing an existing non-specd file after confirmation bypass
--dry-run               print planned actions without modifying files
--help                  print usage
```

I5. Uninstaller options:

```text
--install-dir <path>    override install directory
--dry-run               print planned actions without modifying files
--help                  print usage
```

I6. Environment variables:

```text
SPECD_INSTALL_DIR       default install directory override
SPECD_VERSION           release version override
GITHUB_TOKEN            optional token for GitHub API requests
```

I7. Release URL contract:

```text
https://github.com/0xkhdr/specd/releases/download/<tag>/<archive>
https://github.com/0xkhdr/specd/releases/download/<tag>/checksums.txt
```

Archive names are derived from `.goreleaser.yml` and must be checked before scripts are
documented as stable.

## V - Verification

V1. `go test ./... -race -count=1` passes.

V2. `go test ./... -count=2` passes to catch iteration-order flakiness.

V3. `./scripts/test-lint.sh`, `./scripts/docs-lint.sh`, and shellcheck for all live shell
scripts pass.

V4. `scripts/install.sh --dry-run --version <known-tag> --install-dir <tmp>` resolves the
expected platform archive and target path without writing outside `<tmp>`.

V5. Installer integration tests exercise checksum verification, failed checksum behavior,
existing binary replacement, explicit version selection, unsupported platform failure, and
missing tool failure using local fixture release files or mocked download functions.

V6. `scripts/uninstall.sh --dry-run --install-dir <tmp>` reports only the target binary,
and uninstall removes only `<tmp>/specd`.

V7. Documentation includes install, update, uninstall, custom directory, checksum safety,
and no-user-data-removal notes.

V8. No files under `reference/` are modified.

## B - Blockers And Decisions

B1. Exact release archive naming must be validated against GoReleaser output before
finalizing download URL construction.

B2. GitHub latest-release discovery may use the GitHub API or redirect-based release URLs.
Prefer the simplest deterministic implementation that works with curl and does not add
new runtime dependencies.

B3. If `/usr/local/bin` is not writable, the installer should fail with a precise sudo or
`--install-dir` instruction rather than silently installing elsewhere.
