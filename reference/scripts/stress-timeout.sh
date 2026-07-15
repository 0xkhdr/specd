#!/usr/bin/env bash
# Run a stress command with a wall-clock budget and a deterministic timeout error.
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: stress-timeout.sh <duration> <target> <command> [args...]" >&2
  exit 2
fi
budget="$1"
target="$2"
shift 2

python3 - "$budget" "$target" "$@" <<'PY'
import subprocess
import sys

def parse_seconds(raw: str) -> float:
    raw = raw.strip()
    if raw.endswith("s"):
        return float(raw[:-1])
    if raw.endswith("m"):
        return float(raw[:-1]) * 60
    return float(raw)

budget = parse_seconds(sys.argv[1])
target = sys.argv[2]
cmd = sys.argv[3:]
try:
    completed = subprocess.run(cmd, timeout=budget)
except subprocess.TimeoutExpired:
    print(f"FAIL: {target} target exceeded budget ({sys.argv[1]})", file=sys.stderr)
    raise SystemExit(124)
raise SystemExit(completed.returncode)
PY
