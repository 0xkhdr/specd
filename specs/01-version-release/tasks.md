# Tasks — 01-version-release

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | internal/version/version.go, internal/version/version_test.go | | `go test ./internal/version/... -race -count=1` | Package with Version/Commit/Date vars defaulting to dev-mode; `debug.ReadBuildInfo()` fallback for commit/date; `Info()` accessor returns resolved struct (R3) |
| T2 | craftsman | internal/core/commands.go, internal/cmd/version.go, internal/cmd/registry.go, internal/cmd/version_test.go | T1 | `go test ./internal/cmd -run TestVersion -race -count=1` | `version` verb registered per existing pattern; human line by default, stable JSON with `--json`; unknown extra args exit 2 (R1, R2, R5) |
| T3 | craftsman | .goreleaser.yml | T2 | `goreleaser check` | Static builds (CGO_ENABLED=0, -trimpath) for linux/amd64, linux/arm64, darwin/arm64; ldflags -X inject version/commit/date into internal/version; reference/ used as design input only, nothing copied (R4) |
| T4 | craftsman | .github/workflows/ (release + CI config) | T3 | `grep -rl "goreleaser" .github/workflows/` | CI validates goreleaser config on every push; tag-triggered release workflow builds snapshot/release artifacts |
| T5 | craftsman | docs/command-reference.md, docs/CHEATSHEET.md | T2 | `./scripts/docs-lint.sh` | `version` verb + `--json` documented in both files, docs-lint green (R5) |
| T6 | validator | (read-only) | T2,T3 | `bash -c 'go build -ldflags "-X github.com/0xkhdr/specd/internal/version.Version=v0.0.0-test" -o /tmp/specd-vtest . && /tmp/specd-vtest version --json | grep -q v0.0.0-test'` | ldflags injection observed end-to-end on built binary (R1–R4) |
