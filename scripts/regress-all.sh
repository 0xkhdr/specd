#!/usr/bin/env sh
# regress-all.sh — cross-wave regression harness.
#
# Re-runs every completed task's `verify:` command literally, exactly as written
# in the domain tasks tables under specs/, one per task, and aggregates by exit
# code. The verdict comes from the log, never from judgment: exit non-zero iff
# any verify fails.
#
# Scope: all ten domains, specs/[0-1][0-9]-*/tasks.md, rows marked `[x]`
# (a completed task's verify is its standing evidence and must still hold).
# Release-validator rows whose verify calls this script are skipped to avoid
# infinite self-recursion — the same guard the pre-split harness applied.
#
# Runs at HEAD in the repo root (not a copy): the verifies reference `go test
# ./internal/...`, `./scripts/*.sh`, `.git`, and other repo-relative state that
# only exists here.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
LOG="${REGRESS_LOG:-$ROOT/scripts/regress-all.log}"

: >"$LOG"
failed=0
ran=0

# Extract one markdown table cell (1-based pipe field N), trimmed. The split is
# both backtick-aware (a `|` inside a `...` code span is not a delimiter) and
# escape-aware (a backslash-escaped `\|` is not a delimiter), so multi-pattern
# `-run 'TestA|TestB'` selectors survive intact whether wrapped in a code span
# or backslash-escaped.
cell() {
	printf '%s\n' "$1" | awk -v n="$2" '{
		f=1; c=""; bt=0
		for(i=1;i<=length($0);i++){ ch=substr($0,i,1)
			if(ch=="\\" && i<length($0)){ c=c ch substr($0,i+1,1); i++; continue }
			if(ch=="`"){bt=!bt; c=c ch; continue}
			if(ch=="|" && !bt){ if(f==n){print c; exit} f++; c=""; continue }
			c=c ch }
		if(f==n) print c
	}' | sed 's/^[ \t]*//;s/[ \t]*$//'
}

# Reduce a verify cell to its runnable command. If the cell contains a backtick
# code span (optionally followed by a human annotation like ` (GREEN)`), use the
# first span's contents; otherwise use the cell verbatim. Any table-escaped `\|`
# is un-escaped to a literal pipe.
verify_cmd() {
	printf '%s\n' "$1" | awk '{
		s=index($0,"`")
		if(s==0){print; next}
		rest=substr($0,s+1); e=index(rest,"`")
		if(e==0){print; next}
		print substr(rest,1,e-1)
	}' | sed 's/\\|/|/g'
}

run_row() {
	id=$1
	cmd=$2
	if [ -z "$cmd" ]; then
		printf '%-8s %-6s %s\n' "$id" "SKIP" "(no verify command)" >>"$LOG"
		return 0
	fi
	case "$cmd" in
		*regress-all.sh*)
			printf '%-8s %-6s %s\n' "$id" "SKIP" "(self-recursive: $cmd)" >>"$LOG"
			return 0 ;;
	esac
	ran=$((ran+1))
	if (cd "$ROOT" && sh -c "$cmd") >/dev/null 2>&1; then
		printf '%-8s %-6s rc=%-3s %s\n' "$id" "PASS" "0" "$cmd" >>"$LOG"
	else
		rc=$?
		printf '%-8s %-6s rc=%-3s %s\n' "$id" "FAIL" "$rc" "$cmd" >>"$LOG"
		failed=1
	fi
}

for tasks in "$ROOT"/specs/[0-1][0-9]-*/tasks.md; do
	[ -f "$tasks" ] || continue
	while IFS= read -r line; do
		# Completed task rows only: id cell like `[x] T07`.
		id=$(cell "$line" 2 | sed -n 's/^\[x\] \(T[0-9][0-9]*\)$/\1/p')
		[ -n "$id" ] || continue
		verify=$(verify_cmd "$(cell "$line" 6)")
		run_row "$id" "$verify"
	done <"$tasks"
done

cat "$LOG"
printf '\nregress-all: %d verifies executed\n' "$ran"
if [ "$ran" -eq 0 ]; then
	printf 'regress-all: executed zero rows — layout/parse mismatch, treating as failure\n' >&2
	exit 1
fi
[ "$failed" -eq 0 ] || printf 'regress-all: one or more verifies FAILED\n' >&2
exit "$failed"
