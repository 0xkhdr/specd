#!/usr/bin/env sh
# stress-brain.sh (spec 07 T7) — cross-process crash-recovery stress.
#
# Scaffolds a crashed controller state (a write-ahead checkpoint whose mission
# never reached the ledger), then races N concurrent `brain resume` processes at
# it. The invariant: exactly one resume re-issues the mission and the ledger
# carries that dispatch exactly once — no double-dispatch under a race, one
# holder wins the session-revision CAS. Runs in a throwaway tree so the working
# repo's `.specd/` is never touched. Exits non-zero on the first violation.
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
	echo "stress-brain: expected exactly one dispatch of demo.s1.T1, got $dispatches" >&2
	exit 1
fi

echo "stress-brain: ok ($racers racing resumes converged on one dispatch)"
