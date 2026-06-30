#!/usr/bin/env bash
# stress.sh — hammer ONE spec from many concurrent specd *processes* to prove
# the advisory-lock + revision-CAS path serializes writes with no lost updates
# and no torn state.json (Stage 07 F6 / Stage 02).
#
# Each `specd midreq` call does exactly one locked SaveState (revision++ and
# state.Turn++). With a correct lock every successful call commits, so the final
# state.Turn MUST equal the number of successful invocations. A lost update
# (broken lock) shows up as Turn < successes; a corrupt write shows up as
# state.json failing to load.
#
# Usage: ./scripts/stress.sh [WORKERS] [ITERS]
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# Normal run: 16 workers x 20 short CLI calls. Limits are intentionally broad
# for shared CI hosts while still bounding process/fd runaway; override with SPECD_STRESS_*.
# Leak guard uses process/fd counts because this is a cross-process shell stress,
# not an in-process goroutine harness.
. "$repo/scripts/stress-lib.sh"
stress_set_limits "stress"
stress_guard_begin "stress"

WORKERS="${1:-16}"
ITERS="${2:-20}"

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
# Each worker fires ITERS contending writes and records how many committed
# (exit 0). midreq --impact low never gates, so every call should commit; we
# count exit codes rather than assume to stay honest if the lock ever regresses.
for w in $(seq 1 "$WORKERS"); do
  (
    ok=0
    for i in $(seq 1 "$ITERS"); do
      if "$bin" midreq stress "w${w}-i${i}" --impact low >/dev/null 2>&1; then
        ok=$((ok + 1))
      fi
    done
    echo "$ok" >"$work/ok.$w"
  ) &
done
wait

successes=0
for f in "$work"/ok.*; do
  successes=$((successes + $(cat "$f")))
done

state="$work/.specd/specs/stress/state.json"

# 1. state.json must still be valid JSON (no torn write under contention).
if command -v python3 >/dev/null 2>&1; then
  python3 -c "import json; json.load(open('$state'))" \
    || { echo "FAIL: state.json is not valid JSON — torn write detected"; exit 1; }
else
  echo "WARN: python3 not found — skipping the JSON torn-write check (the turn==successes lost-update check below still runs)." >&2
fi

# 2. Final Turn must equal the number of committed writes — no lost updates.
read_field() {
  python3 -c "import json; print(json.load(open('$state'))['$1'])" 2>/dev/null \
    || grep -o "\"$1\"[[:space:]]*:[[:space:]]*[0-9]*" "$state" | grep -o '[0-9]*$'
}
turn="$(read_field turn)"
revision="$(read_field revision)"

echo "committed writes: $successes   final turn: $turn   final revision: $revision"

if [[ "$turn" != "$successes" ]]; then
  echo "FAIL: lost update — final turn ($turn) != committed writes ($successes)"
  exit 1
fi

stress_guard_end

echo "PASS: $WORKERS x $ITERS concurrent processes, $successes committed writes, turn==successes, state.json intact."
