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

# Run ledger (spec 07 R2.4): racing writers must not duplicate an attempt, and a
# crash mid-append must leave no partial line. Race concurrent `specd verify`,
# each of which allocates one run/attempt under the spec lock, then assert the
# ledger is well-formed with a single run_id and no duplicate attempt.
printf '{"schema_version":2,"slug":"demo","mode":"default","status":"tasks","phase":"plan","revision":0,"records":{}}\n' > "$spec/state.json"
printf '| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n' > "$spec/tasks.md"

i=0
while [ "$i" -lt "$racers" ]; do
	( cd "$tree" && "$bin" verify demo T1 >/dev/null 2>&1 || true ) &
	i=$((i + 1))
done
wait

runs="$spec/runs.jsonl"
runlines=0
while IFS= read -r line; do
	[ -n "$line" ] || { echo "stress-acp: blank line in run ledger (torn append)" >&2; exit 1; }
	case "$line" in
		\{*\}) ;;
		*) echo "stress-acp: malformed run ledger line: $line" >&2; exit 1 ;;
	esac
	case "$line" in
		*'"run_id":"'*'"'*) ;;
		*) echo "stress-acp: run ledger line missing run_id: $line" >&2; exit 1 ;;
	esac
	runlines=$((runlines + 1))
done < "$runs"

attempts=$(grep -o '"attempt":[0-9][0-9]*' "$runs" | sed 's/.*://')
runiq=$(printf '%s\n' "$attempts" | sort -u | grep -c '.' || true)
runtotal=$(printf '%s\n' "$attempts" | grep -c '.' || true)
if [ "$runiq" != "$runtotal" ]; then
	echo "stress-acp: duplicate attempt in run ledger ($runtotal runs, $runiq distinct)" >&2
	exit 1
fi
runidcount=$(grep -o '"run_id":"[^"]*"' "$runs" | sort -u | grep -c '.' || true)
if [ "$runidcount" != "1" ]; then
	echo "stress-acp: expected one run chain, found $runidcount run_id(s)" >&2
	exit 1
fi

echo "stress-acp: ok ($racers racing appends, $lines well-formed line(s), $uniq distinct seq; $runlines run(s), $runiq distinct attempt(s), 1 chain)"
