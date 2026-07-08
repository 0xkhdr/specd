#!/usr/bin/env sh
# coverage-check.sh (SPEC-01 T-01-03; policy floor set by SPEC-05 T-05-02) —
# produce coverage.out and enforce the total-coverage floor.
#
# SPEC-05 owns the policy. The floor is a RATCHET: raise it as coverage climbs,
# never lower it. Measured total on the SPEC-05 HEAD: 75.7%. Policy floor 75.0%
# — a deliberate target above SPEC-01's provisional 74.0%, with ~0.7% headroom
# absorbing run-to-run jitter (atomic-mode counters vary slightly). See
# TESTING.md for the per-package table and the ratchet rule.
#
# Exits non-zero if total coverage drops below FLOOR.
set -eu

FLOOR=75.0

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

go test ./... -count=1 -coverprofile=coverage.out >/dev/null

total=$(go tool cover -func=coverage.out | awk '/^total:/ { gsub(/%/, "", $NF); print $NF }')
if [ -z "$total" ]; then
	echo "coverage-check: could not parse total coverage from coverage.out" >&2
	exit 1
fi

if awk -v t="$total" -v f="$FLOOR" 'BEGIN { exit !(t + 0 < f + 0) }'; then
	echo "coverage-check: total coverage ${total}% is below floor ${FLOOR}%" >&2
	exit 1
fi

echo "coverage-check: ok (total ${total}% >= floor ${FLOOR}%)"
