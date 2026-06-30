#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# ACP stress is three race-detector runs over bounded unit suites; 2048 procs / 4096 fds
# leave broad Go test/CI headroom while catching fork/fd runaway failures.
. "$repo/scripts/stress-lib.sh"
stress_set_limits "stress-acp"
stress_guard_begin "stress-acp"

go test ./internal/core/... -run 'TestACP(Store|Lease|Archive|Security|Pinky.*Claim)' -race -count=3
stress_guard_end
