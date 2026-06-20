#!/usr/bin/env bash
# test-lint.sh — structural lint for the test suite (spec.md §7 "Definition of
# done", tasks.md E1). Fails on the three drift modes the rebuild eliminated:
#
#   1. banned file suffixes  (_more / _regression / _sweep / _scale / wave*)
#   2. space-separated subtest names (sentence-case t.Run labels)
#   3. re-introduced duplicate helpers (a helper defined in >1 _test.go file)
#
# Pure grep/awk so it runs anywhere `bash` does; no Go build required.
set -euo pipefail

cd "$(cd "$(dirname "$0")/.." && pwd)"

status=0
fail() {
	status=1
	echo "FAIL: $1" >&2
}

# 1. Banned file-name suffixes ------------------------------------------------
banned=$(find internal main_test.go -name '*_test.go' 2>/dev/null \
	| grep -E '_(more|regression|sweep|scale)_test\.go$|wave[0-9]' || true)
if [ -n "$banned" ]; then
	fail "banned test-file suffix (_more/_regression/_sweep/_scale/wave*):"
	# shellcheck disable=SC2001
	echo "$banned" | sed 's/^/  /' >&2
fi

# 2. Space-separated (sentence-case) subtest names ----------------------------
spaces=$(grep -rnE 't\.Run\("[^"]* [^"]*"' --include='*_test.go' internal main_test.go || true)
if [ -n "$spaces" ]; then
	fail "space-separated subtest name(s) — use snake_case outcome names:"
	# shellcheck disable=SC2001
	echo "$spaces" | sed 's/^/  /' >&2
fi

# 3. Duplicate helper definitions (within a package) --------------------------
# A helper func defined in more than one _test.go file *in the same directory*
# is the duplication the consolidation removed: it should live once, in that
# package's helpers_test.go (or as an exported testharness symbol). A same-named
# trivial helper in a different package (e.g. core's itoa vs mcp's itoa) is not a
# redeclaration and is not flagged.
dup_report=""
for dir in $(find internal -type d | sort); do
	tests=$(find "$dir" -maxdepth 1 -name '*_test.go' 2>/dev/null)
	[ -z "$tests" ] && continue
	# names defined more than once across files in this directory
	# shellcheck disable=SC2086
	dupes=$(grep -hoE '^func [a-z][A-Za-z0-9_]*' $tests \
		| awk '{print $2}' | sort | uniq -d || true)
	for name in $dupes; do
		# shellcheck disable=SC2086
		files=$(grep -lE "^func ${name}\(" $tests | sort -u)
		# shellcheck disable=SC2001
		dup_report+="  ${name} (in ${dir}) defined in:\n$(echo "$files" | sed 's/^/    /')\n"
	done
done
if [ -n "$dup_report" ]; then
	fail "duplicate test helper(s) — consolidate into the package helpers_test.go or testharness:"
	printf '%b' "$dup_report" >&2
fi

if [ "$status" -eq 0 ]; then
	echo "test-lint: ok (no banned suffixes, no space-named subtests, no duplicate helpers)"
fi
exit "$status"
