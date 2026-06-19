#!/usr/bin/env sh
set -eu

go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostBrainLifecycle|TestOrchestration.*(Engine|Pause|Resume|Cancel|Recovery|Retry)' -race -count=3
