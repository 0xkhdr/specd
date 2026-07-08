#!/usr/bin/env sh
# stress-orchestration.sh (SPEC-01 T-01-04) — Brain/Pinky orchestration contention.
#
# Races N concurrent `brain resume` processes at one session and inspects the
# controller state file (not the ledger). Focus is the session-revision CAS:
# concurrent controllers must not corrupt session.json, and exactly one may win
# the transition (advance the revision past its scaffolded value).
#
# Invariants after the race:
#   - session.json still parses (starts { ends }, carries "revision")
#   - the session revision advanced past its scaffolded value (1), i.e. the CAS
#     transition happened
#   - exactly one controller won: the ledger carries exactly one dispatch
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

session="$spec/session.json"
body=$(cat "$session")
case "$body" in
	\{*\}*) ;;
	*) echo "stress-orchestration: session.json is not a JSON object after contention" >&2; exit 1 ;;
esac

rev=$(sed -n 's/.*"revision"[ ]*:[ ]*\([0-9][0-9]*\).*/\1/p' "$session" | head -n1)
[ -n "$rev" ] || { echo "stress-orchestration: session.json lost its revision field" >&2; exit 1; }
if [ "$rev" -le 1 ]; then
	echo "stress-orchestration: session revision is $rev, expected an advance past scaffolded 1" >&2
	exit 1
fi

dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
if [ "$dispatches" != "1" ]; then
	echo "stress-orchestration: expected exactly one dispatch, got $dispatches" >&2
	exit 1
fi

echo "stress-orchestration: ok ($racers racers, session revision advanced 1->$rev once, one dispatch)"
