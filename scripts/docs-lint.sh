#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! cmp -s "$root/docs/command-reference.md" "$root/docs/CHEATSHEET.md"; then
	echo "docs-lint: docs/CHEATSHEET.md must mirror docs/command-reference.md" >&2
	exit 1
fi

echo "docs-lint: ok"
