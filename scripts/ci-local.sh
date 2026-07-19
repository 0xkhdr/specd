#!/usr/bin/env sh
# ci-local: run the same gate set CI runs (.github/workflows/ci.yml), in order,
# failing on the first failing gate. Mirrors CI so a green run here means a green
# pipeline. External tools that may be absent locally (golangci-lint, the vuln
# scanner, the shell linter) are guarded with command -v and SKIPped when missing.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

# --- gofmt (fail on any unformatted file) ---
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
	echo "ci-local: gofmt found unformatted files:" >&2
	echo "$unformatted" >&2
	exit 1
fi

# --- go vet ---
go vet ./...

# --- go mod tidy: assert no diff (zero runtime deps stay tidy) ---
go mod tidy
git diff --exit-code -- go.mod go.sum

# --- structural / docs / install-script lints ---
./scripts/test-lint.sh
./scripts/docs-lint.sh
./scripts/install-scripts-test.sh

# --- tests: race, then an order-dependence rerun ---
go test ./... -race -count=1
go test ./... -count=2

# --- perf gate and coverage floor ---
./scripts/perf-gate.sh
./scripts/coverage-check.sh

# --- staticcheck via golangci-lint (guarded: absent on a clean checkout) ---
if command -v golangci-lint >/dev/null 2>&1; then
	golangci-lint run
else
	echo "SKIP: golangci-lint not installed (CI runs it — staticcheck)"
fi

# --- govulncheck (guarded) ---
if command -v govulncheck >/dev/null 2>&1; then
	govulncheck ./...
else
	echo "SKIP: govulncheck not installed (CI runs it)"
fi

# --- shellcheck over scripts/*.sh (guarded) ---
if command -v shellcheck >/dev/null 2>&1; then
	shellcheck scripts/*.sh
else
	echo "SKIP: shellcheck not installed (CI runs it)"
fi

echo "ci-local: ok"
