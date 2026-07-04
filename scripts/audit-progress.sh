#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PROGRESS="${PROGRESS:-$ROOT/specs/progress.md}"
LOG="${AUDIT_LOG:-$ROOT/scripts/audit-progress.log}"
RUNROOT=$(mktemp -d)
trap 'rm -rf "$RUNROOT"' EXIT

(cd "$ROOT" && tar --exclude=.git -cf - .) | (cd "$RUNROOT" && tar -xf -)
rm -rf "$RUNROOT/.specd/specs/demo"

: >"$LOG"

fail_green=0

run_task() {
	status=$1
	task=$2
	cmd=$3

	if [ -z "$cmd" ]; then
		printf '%-8s %-7s rc=%-3s %-8s %s\n' "$task" "$status" "-" "SKIP" "(no verify command)" >>"$LOG"
		return 0
	fi

	out=$(mktemp)
	err=$(mktemp)
	if (cd "$RUNROOT" && sh -c "$cmd") >"$out" 2>"$err"; then
		rc=0
		result=PASS
	else
		rc=$?
		result=FAIL
	fi
	rm -f "$out" "$err"

	printf '%-8s %-7s rc=%-3s %-8s %s\n' "$task" "$status" "$rc" "$result" "$cmd" >>"$LOG"

	if [ "$status" = "green" ] && [ "$rc" -ne 0 ]; then
		fail_green=1
	fi
}

while IFS= read -r line; do
	case "$line" in
		"| "*T*" | "*" | "*" | "*" |"*)
			status_cell=$(printf '%s\n' "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}')
			task=$(printf '%s\n' "$status_cell" | sed -n 's/.*\(T[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')
			[ -n "$task" ] || continue

			case "$status_cell" in
				*✅*) status=green ;;
				*) status=pending ;;
			esac

			cmd=$(printf '%s\n' "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $5); print $5}')
			cmd=$(printf '%s\n' "$cmd" | sed -n 's/^`\([^`][^`]*\)`$/\1/p')
			run_task "$status" "$task" "$cmd"
			;;
	esac
done <"$PROGRESS"

cat "$LOG"
exit "$fail_green"
