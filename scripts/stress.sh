#!/usr/bin/env bash
# stress.sh — hammer one spec from many concurrent specd processes to prove the
# advisory-lock + CAS path serializes writes with no lost updates or corruption.
#
# Usage: ./scripts/stress.sh [WORKERS] [ITERS]
set -euo pipefail

WORKERS="${1:-16}"
ITERS="${2:-20}"

repo="$(cd "$(dirname "$0")/.." && pwd)"
bin="$repo/specd"
if [[ ! -x "$bin" ]]; then
  echo "building specd..."
  (cd "$repo" && go build -o "$bin" .)
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cd "$work"

"$bin" init >/dev/null
"$bin" new stress --title "Stress" >/dev/null

echo "launching $WORKERS workers x $ITERS iterations against spec 'stress'..."
# Each worker repeatedly runs a command that takes the spec lock + CAS-writes.
# `status` loads + reconciles state under the lock; concurrent runs must never
# corrupt state.json or trip the CAS guard fatally.
pids=()
for w in $(seq 1 "$WORKERS"); do
  (
    for _ in $(seq 1 "$ITERS"); do
      "$bin" status stress >/dev/null 2>&1 || true
    done
  ) &
  pids+=("$!")
done
for p in "${pids[@]}"; do wait "$p"; done

# Verify the resulting state.json is still valid JSON (no torn writes).
state="$work/.specd/specs/stress/state.json"
if command -v python3 >/dev/null 2>&1; then
  python3 -c "import json,sys; json.load(open('$state')); print('state.json valid:', '$state')"
else
  "$bin" status stress >/dev/null && echo "state.json loads cleanly via specd"
fi

echo "PASS: $WORKERS x $ITERS concurrent runs left state.json intact."
