#!/usr/bin/env sh
# regress-all.sh (P7.1) — cross-wave regression harness.
#
# Runs every `verify:` command literally, exactly as written in the W0–W6
# tasks tables, one per task, and aggregates by exit code. The verdict comes
# from the log, never from judgment: exit non-zero iff any verify fails.
#
# Scope is W0–W6 (dirs 00..06). The W7 wave (07-regression-acceptance) is
# excluded on purpose — its own verify is `sh scripts/regress-all.sh`, so
# including it would recurse forever.
#
# Runs at HEAD in the repo root (not a copy): the verifies reference `./specd`,
# `go test ./internal/...`, `.git` (P6.1 `gh run list -b fresh-start`) and
# other repo-relative state that only exists here.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
LOG="${REGRESS_LOG:-$ROOT/scripts/regress-all.log}"

: >"$LOG"
failed=0

# Extract one markdown table cell (1-based pipe field N), trimmed. Backtick-aware:
# a `|` inside a `...` span (e.g. `-run 'A|B'` or an escaped `\|`) is not a delimiter.
cell() {
	printf '%s\n' "$1" | awk -v n="$2" '{
		f=1; c=""; bt=0
		for(i=1;i<=length($0);i++){ ch=substr($0,i,1)
			if(ch=="`"){bt=!bt; c=c ch; continue}
			if(ch=="|" && !bt){ if(f==n){print c; exit} f++; c=""; continue }
			c=c ch }
		if(f==n) print c
	}' | sed 's/^[ \t]*//;s/[ \t]*$//'
}

run_row() {
	id=$1
	cmd=$2
	if [ -z "$cmd" ]; then
		printf '%-8s %-6s %s\n' "$id" "SKIP" "(no verify command)" >>"$LOG"
		return 0
	fi
	if (cd "$ROOT" && sh -c "$cmd") >/dev/null 2>&1; then
		printf '%-8s %-6s rc=%-3s %s\n' "$id" "PASS" "0" "$cmd" >>"$LOG"
	else
		rc=$?
		printf '%-8s %-6s rc=%-3s %s\n' "$id" "FAIL" "$rc" "$cmd" >>"$LOG"
		failed=1
	fi
}

for tasks in "$ROOT"/review-specs/0[0-6]-*/tasks.md; do
	[ -f "$tasks" ] || continue
	while IFS= read -r line; do
		# Task rows only: id cell like P0.1a / P7.3 (letters after the number ok).
		id=$(cell "$line" 2 | sed -n 's/^\(P[0-9][0-9]*\.[0-9][0-9]*[a-z]*\)$/\1/p')
		[ -n "$id" ] || continue
		# verify = field 6, first backtick-wrapped span, un-escape table `\|`.
		verify=$(cell "$line" 6 | sed -n 's/^[^`]*`\([^`]*\)`.*/\1/p' | sed 's/\\|/|/g')
		run_row "$id" "$verify"
	done <"$tasks"
done

cat "$LOG"
[ "$failed" -eq 0 ] || printf '\nregress-all: one or more verifies FAILED\n' >&2
exit "$failed"
