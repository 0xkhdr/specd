#!/usr/bin/env sh
set -eu

go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostProgram|TestProgramOrchestration.*(Lease|Capacity|Frontier|Escalate|Pause|Cancel|Recovery|Complete)' -race -count=3
