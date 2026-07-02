# S9 Tasks — Cross-Platform Regression

Requirement coverage: R9. Dependencies: earlier specs' code stable.

## Wave 1 — Baseline

- [x] Confirm host build works and record binary version stamp. File: `Makefile`.
- [x] Inventory Windows-specific code + skips: `internal/worker/runner_windows.go`,
  `runner_windows_test.go`.
- **Validation:** `make build && ./specd --version`

## Wave 2 — Cross-compile smoke (depends on Wave 1)

- [x] Add cross-compile smoke check for windows/amd64. Command in CI or local
  script. Files: `.github/workflows/ci.yml` build job (verify targets).
- [x] Add cross-compile smoke check for darwin/arm64 and linux/arm64.
- [x] Guard any POSIX-only path so `GOOS=windows go build ./...` succeeds.
- **Validation:** `GOOS=windows GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./...`

## Wave 3 — Documentation of limits (depends on Wave 2)

- [x] Document Windows degraded features (self-update, brain/pinky) in
  `TESTING.md`/`README.md`.
- **Validation:** `make docs-lint`

## Rollout & cleanup

- [ ] Verify CI `build:` matrix (ubuntu/macos/windows) is green on PR (pending remote CI).
- **Rollback:** revert build-tag/skip changes.
- **Completion evidence:** green cross-compiles + green CI build matrix.
