#!/usr/bin/env sh
set -eu

# Deterministic brain recovery stress: exercises orchestration/program recovery
# and retry paths under the race detector. SPECD_STRESS_COUNT may raise/lower
# repetitions for local debugging; default mirrors existing stress jobs.
COUNT="${SPECD_STRESS_COUNT:-3}"

go test ./internal/core/... ./internal/cmd/... -run 'Test.*(Orchestration.*Recovery|ProgramOrchestration.*Recovery|Brain.*Recovery|DriveOrchestrationWorkerFailureEscalates)' -race -count="${COUNT}"
