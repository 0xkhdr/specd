#!/usr/bin/env sh
# stress-checkpoint-fault.sh (SPEC-01 T-01-04) — crash mid-checkpoint.
#
# Injects a fault: a write-ahead checkpoint whose mission never reached the
# ledger, PLUS a stale lease left behind by the crashed worker. Then races N
# concurrent `brain resume` processes at it. Recovery must reconcile the crash
# cleanly under contention.
#
# Invariants after the race:
#   - no double-claim: the ledger carries the crashed mission's dispatch exactly
#     once
#   - no orphaned lease: the stale lease is reclaimed — no "holder" survives in
#     session.json
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

printf '{"id":"demo","revision":1,"state":"running","leases":[{"task_id":"T1","mission_id":"demo.s1.T1","holder":"pinky-crashed"}]}\n' > "$spec/session.json"
printf '{"session_id":"demo","step":1,"mission_id":"demo.s1.T1","task_id":"T1","time":"%s"}\n' "$now" > "$spec/checkpoint.json"
: > "$spec/acp.jsonl"

racers=8
i=0
while [ "$i" -lt "$racers" ]; do
	( cd "$tree" && "$bin" brain resume demo >/dev/null 2>&1 || true ) &
	i=$((i + 1))
done
wait

dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
if [ "$dispatches" != "1" ]; then
	echo "stress-checkpoint-fault: double-claim — expected one dispatch, got $dispatches" >&2
	exit 1
fi

if grep -q '"holder"' "$spec/session.json"; then
	echo "stress-checkpoint-fault: orphaned lease survived recovery:" >&2
	cat "$spec/session.json" >&2
	exit 1
fi

echo "stress-checkpoint-fault: ok ($racers racers, one dispatch, stale lease reclaimed)"
