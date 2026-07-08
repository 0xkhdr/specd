#!/usr/bin/env sh
# stress-program.sh (SPEC-01 T-01-04) — cross-spec program scheduling contention.
#
# Scaffolds M distinct crashed specs and races concurrent `brain resume`
# processes across ALL of them at once. Focus is per-spec isolation: the
# per-spec lock must keep concurrent recovery of one spec from bleeding into
# another. Each spec must converge independently to exactly one dispatch of its
# own mission — no cross-spec double-claim, no missed spec.
#
# Runs in a throwaway tree; exits non-zero on the first violation.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=$(mktemp -d)/specd
go build -o "$bin" "$root"

tree=$(mktemp -d)
trap 'rm -rf "$tree"' EXIT
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

specs="alpha bravo charlie"
for s in $specs; do
	spec="$tree/.specd/specs/$s"
	mkdir -p "$spec"
	printf '{"id":"%s","revision":1,"state":"running","leases":[]}\n' "$s" > "$spec/session.json"
	printf '{"session_id":"%s","step":1,"mission_id":"%s.s1.T1","task_id":"T1","time":"%s"}\n' "$s" "$s" "$now" > "$spec/checkpoint.json"
	: > "$spec/acp.jsonl"
done

# Race resumes across every spec at once: each spec gets several racers,
# interleaved with the others.
i=0
while [ "$i" -lt 4 ]; do
	for s in $specs; do
		( cd "$tree" && "$bin" brain resume "$s" >/dev/null 2>&1 || true ) &
	done
	i=$((i + 1))
done
wait

for s in $specs; do
	ledger="$tree/.specd/specs/$s/acp.jsonl"
	dispatches=$(grep -c "\"mission_id\":\"$s.s1.T1\"" "$ledger" || true)
	if [ "$dispatches" != "1" ]; then
		echo "stress-program: spec $s expected exactly one dispatch, got $dispatches" >&2
		exit 1
	fi
	# No other spec's mission should have leaked into this ledger.
	foreign=$(grep -c '"mission_id"' "$ledger" || true)
	if [ "$foreign" != "1" ]; then
		echo "stress-program: spec $s ledger has $foreign mission events (cross-spec bleed)" >&2
		exit 1
	fi
done

echo "stress-program: ok (3 specs recovered concurrently, one isolated dispatch each)"
