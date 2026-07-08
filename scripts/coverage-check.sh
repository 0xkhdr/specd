#!/usr/bin/env sh
# coverage-check.sh (SPEC-01 T-01-03) — produce coverage.out and enforce a floor.
#
# SPEC-01 sets a PROVISIONAL floor at (just below) the current measured total
# coverage so the job is green today. SPEC-05 owns the coverage policy and
# ratchets FLOOR up to a real target.
#
#   Measured total coverage on the SPEC-01 HEAD: 74.8%
#   Provisional floor: 74.0% (small margin absorbs run-to-run jitter; not a
#   policy target — SPEC-05 raises it).
#
# Exits non-zero if total coverage drops below FLOOR.
set -eu

FLOOR=74.0

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
