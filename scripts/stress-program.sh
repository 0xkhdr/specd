#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# Program stress is bounded race-detector orchestration coverage; limits sit well
# above normal Go test fan-out but stop runaway child/fd leaks.
. "$repo/scripts/stress-lib.sh"
stress_set_limits "stress-program"
stress_guard_begin "stress-program"

go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostProgram|TestProgramOrchestration.*(Lease|Capacity|Frontier|Escalate|Pause|Cancel|Recovery|Complete)' -race -count=3
stress_guard_end
