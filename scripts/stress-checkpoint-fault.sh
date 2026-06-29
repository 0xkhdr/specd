#!/usr/bin/env sh
set -eu

# Checkpoint fault-injection stress (spec A3): re-runs the crash-injection test,
# which re-execs a child, SIGKILL-emulates it (os.Exit) at each window inside
# RecordCheckpoint, and asserts no double-claim and no orphaned lease on resume.
#
# The injected window plus the iteration index are the reproducible "seed": a
# failure prints the failing window and -count run, and SPECD_FAULT_CHECKPOINT
# is honored only when set, so the production path is untouched. SPECD_STRESS_COUNT
# raises/lowers repetitions; default mirrors the existing stress jobs. Kept bounded
# and -race so CI fails fast with the seed rather than hanging.
COUNT="${SPECD_STRESS_COUNT:-5}"

echo "checkpoint fault-injection stress: ${COUNT} iterations across crash windows"
go test ./internal/core/ -run '^TestCheckpointFaultInjection$' -race -count="${COUNT}" -v
