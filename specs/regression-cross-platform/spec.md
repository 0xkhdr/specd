# S9 — Cross-Platform Regression

## 1. Purpose and requirement coverage

Guarantee the binary builds and behaves on all target platforms. Covers **R9**.

## 2. Verified current state

- Release build matrix: `.goreleaser.yml` (GoReleaser + SBOM via `syft`).
- CI build job: `.github/workflows/ci.yml` `build:` matrix =
  `ubuntu-latest, macos-latest, windows-latest`. The `test:` job runs on
  ubuntu + macOS × go `1.22`/`stable`.
- Windows-specific worker: `internal/worker/runner_windows.go`,
  `runner_windows_test.go`. Known limitations (Windows self-update; POSIX-only
  brain/pinky orchestration) documented in `README.md`/`TESTING.md`.
- `go.mod`: `go 1.22`, `toolchain go1.22.0`, stdlib-only.
- Local build: `make build` (`go build -ldflags "-s -w -X main.version=…"`).

## 3. Proposed design and end-to-end flow

Regression relies on the CI build matrix compiling for linux/darwin/windows and
`go test` passing on ubuntu+macOS. Add `GOOS=windows go build` and `GOOS=darwin`
cross-compile smoke checks locally. Guard POSIX-only paths (brain/pinky) with
build tags or `runtime.GOOS` skips so Windows compiles cleanly.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** cross-compile targets (linux/darwin/windows × amd64/arm64);
  `main.version` ldflag injection.
- **Dependencies:** none for build; consumes all specs' code.

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Compatibility:** Windows must continue to compile; degraded features are
  documented, not silent.
- **Rollback:** build config change is revertible via git.

## 6. Acceptance criteria and validation commands

- `make build` succeeds on the host.
- `GOOS=windows GOARCH=amd64 go build ./...` succeeds.
- `GOOS=darwin GOARCH=arm64 go build ./...` succeeds.
- CI `build:` matrix green on ubuntu/macOS/windows.

## 7. Open decisions and deviations

- Deviation U3: no Windows shell-stress in CI (Windows is build-only). Full
  Windows stress requires WSL/Windows runner — documented in `TESTING.md`.
- Deviation F2: brain/pinky orchestration is POSIX-only on Windows; verified by
  `runner_windows.go` presence. Test skips must document this.
