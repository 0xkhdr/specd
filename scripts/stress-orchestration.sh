#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
# Orchestration stress runs race-enabled integration/core tests three times; generous
# process/fd limits protect hosts without constraining valid Go test parallelism.
. "$repo/scripts/stress-lib.sh"
stress_set_limits "stress-orchestration"
stress_guard_begin "stress-orchestration"

go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostBrainLifecycle|TestOrchestration.*(Engine|Pause|Resume|Cancel|Recovery|Retry)' -race -count=3
stress_guard_end
