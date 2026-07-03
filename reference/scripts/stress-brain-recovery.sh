#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# Recovery stress repeats race-enabled crash/retry paths. Default count=3 normally
# finishes well inside the Makefile timeout; limits leave Go test fan-out headroom.
. "$repo/scripts/stress-lib.sh"
stress_set_limits "stress-brain-recovery"
stress_guard_begin "stress-brain-recovery"

# Deterministic brain recovery stress: exercises orchestration/program recovery
# and retry paths under the race detector. SPECD_STRESS_COUNT may raise/lower
# repetitions for local debugging; default mirrors existing stress jobs.
COUNT="${SPECD_STRESS_COUNT:-3}"

go test ./internal/core/... ./internal/cmd/... -run 'Test.*(Orchestration.*Recovery|ProgramOrchestration.*Recovery|Brain.*Recovery|DriveOrchestrationWorkerFailureEscalates)' -race -count="${COUNT}"
stress_guard_end
