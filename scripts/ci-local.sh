#!/usr/bin/env sh
# ci-local: run the Linux fast-tier gates from .github/workflows/ci.yml,
# failing on the first failure. Required external tools fail closed when absent.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

for tool in golangci-lint govulncheck shellcheck; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "ci-local: required tool not found: $tool" >&2
		exit 1
	fi
done

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

# --- structural / docs lints ---
./scripts/test-lint.sh
./scripts/docs-lint.sh

# --- static analysis and vulnerability scan ---
golangci-lint run
govulncheck ./...
shellcheck -S error scripts/*.sh

# --- race tests, coverage floor, and build ---
go test ./... -race -count=1
./scripts/coverage-check.sh
go build ./...

echo "ci-local: ok"
