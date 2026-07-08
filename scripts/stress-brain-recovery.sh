#!/usr/bin/env sh
# stress-brain-recovery.sh (SPEC-01 T-01-04) — brain recovery retry/reclaim.
#
# Scaffolds a crashed controller state (a write-ahead checkpoint whose mission
# never reached the ledger), then races N concurrent `brain resume` processes
# at it. Invariant: exactly one resume re-issues the mission and the ledger
# carries that dispatch exactly once — no double-dispatch under a race; one
# holder wins the session-revision CAS. Runs in a throwaway tree so the working
# repo's `.specd/` is never touched. Exits non-zero on the first violation.
#
# (This is the recovery invariant ci.yml expects at stress-brain-recovery.sh;
# the older scripts/stress-brain.sh is an orphan swept by SPEC-07.)
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

dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
if [ "$dispatches" != "1" ]; then
	echo "stress-brain-recovery: expected exactly one dispatch of demo.s1.T1, got $dispatches" >&2
	exit 1
fi

echo "stress-brain-recovery: ok ($racers racing resumes converged on one dispatch)"
