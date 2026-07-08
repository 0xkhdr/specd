#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if grep -RIn 't\.Run("[^"]* [^"]*"' "$root/internal" --include='*_test.go'; then
	echo "test-lint: subtest names must not contain spaces" >&2
	exit 1
fi

if find "$root/internal" -name '*_test.go' -print | grep -E '(_new|_old|_copy)_test\.go$'; then
	echo "test-lint: banned test filename suffix" >&2
	exit 1
fi

echo "test-lint: ok"
