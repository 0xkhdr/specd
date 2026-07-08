#!/usr/bin/env sh
# perf-gate.sh (SPEC-01 T-01-02) — minimal, runnable A4 perf gate.
#
# Replaces the former `make perf-gate` invocation (no root Makefile exists;
# single-script mechanism preferred). Asserts invariant A4: a disabled-mode
# context-manifest budget check does no work — CheckBudget short-circuits
# before computing or enforcing the manifest cost. Returns non-zero if the
# disabled path does any budget work (i.e. starts rejecting).
#
# This is the minimal gate SPEC-01 owns; SPEC-03 ratchets it to a measured
# O(0)/allocation envelope. Runs the behavioural pin in internal/context.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

go test ./internal/context/ -run TestCheckBudgetDisabledDoesNoWork -count=1

echo "perf-gate: ok (A4 — disabled-mode context budget does no work)"
