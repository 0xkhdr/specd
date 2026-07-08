#!/usr/bin/env sh
# stress-acp.sh (SPEC-01 T-01-04) — ACP ledger append/replay contention.
#
# Races N concurrent `brain resume` processes appending to one spec's acp.jsonl.
# Focus is ledger LINE integrity under concurrent append (not the dispatch
# count, which stress-brain-recovery.sh owns): the append path must never leave
# a torn or blank line, and every event's `seq` must be unique.
#
# Invariants over acp.jsonl after the race:
#   - every non-empty line is a self-contained JSON object ({ … } with "seq")
#   - no duplicate seq values (append offsets don't collide)
# Runs in a throwaway tree; exits non-zero on the first violation.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=$(mktemp -d)/specd
go build -o "$bin" "$root"

tree=$(mktemp -d)
trap 'rm -rf "$tree"' EXIT
spec="$tree/.specd/specs/demo"
mkdir -p "$spec"
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

printf '{"id":"demo","revision":1,"state":"running","leases":[]}\n' > "$spec/session.json"
printf '{"session_id":"demo","step":1,"mission_id":"demo.s1.T1","task_id":"T1","time":"%s"}\n' "$now" > "$spec/checkpoint.json"
: > "$spec/acp.jsonl"

racers=8
i=0
while [ "$i" -lt "$racers" ]; do
	( cd "$tree" && "$bin" brain resume demo >/dev/null 2>&1 || true ) &
	i=$((i + 1))
done
wait

ledger="$spec/acp.jsonl"
lines=0
while IFS= read -r line; do
	[ -n "$line" ] || { echo "stress-acp: blank line in ledger (torn append)" >&2; exit 1; }
	case "$line" in
		\{*\}) ;;
		*) echo "stress-acp: malformed ledger line: $line" >&2; exit 1 ;;
	esac
	case "$line" in
		*'"seq":'*) ;;
		*) echo "stress-acp: ledger line missing seq: $line" >&2; exit 1 ;;
	esac
	lines=$((lines + 1))
done < "$ledger"

seqs=$(grep -o '"seq":[0-9][0-9]*' "$ledger" | sed 's/.*://')
uniq=$(printf '%s\n' "$seqs" | sort -u | grep -c '.' || true)
total=$(printf '%s\n' "$seqs" | grep -c '.' || true)
if [ "$uniq" != "$total" ]; then
	echo "stress-acp: duplicate seq values in ledger ($total events, $uniq distinct)" >&2
	exit 1
fi

echo "stress-acp: ok ($racers racing appends, $lines well-formed line(s), $uniq distinct seq)"
