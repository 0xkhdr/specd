#!/usr/bin/env bash
# verify-progress.sh (T13.13) — enforce ADR-8 on the harness itself.
#
# Every CLI verb whose Definition of Done is "a running binary exercises it" is
# covered by the integration harness in internal/cmd (TestLifecycleE2E drives a
# built binary through init→new→check→approve→next→verify→report; the brain and
# lifecycle tests exercise the orchestration and state seams). This script runs
# that harness. A verb marked done in progress.md but not exercised here has no
# evidence and the script fails.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "== integration harness (built binary + seam tests) =="
go test ./internal/cmd -run 'TestLifecycleE2E|TestBrainDispatchesFrontierViaCLI|TestStatusNextVerifyOnRealSpec|TestApproveGatesE2E|TestMidreqDecisionAppend|TestTaskShowsDetails|TestEveryCommandHasHandler|TestUnknownCommandFailsClosed' -count=1

echo "OK: CLI seam exercised by a running binary."
