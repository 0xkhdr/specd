#!/usr/bin/env bash
# coverage-check.sh — enforce a coverage floor so a refactor cannot silently
# drop test coverage on the integrity-critical paths (Stage 07 F1).
#
# Floors are ratchets, not aspirations: they sit just below the current measured
# coverage to catch regressions without breaking on noise. Raise them as
# coverage improves; never lower them to make a red build pass.
#
# The documented long-term target (TESTING.md) is higher (85% overall / 90% then
# 95% for internal/core); these gates are the regression floor on the way there.
# Floors only ratchet up. Do not lower a floor to make a red build pass; add
# tests or document an intentional coverage-shape change in the PR.
#
# Re-baselined when the `boot`/`enrich` subsystem was removed: those files
# (boot.go, boot_detectors.go, enrich.go, enrich_evidence.go) were heavily tested
# and carried internal/core's aggregate well above the rest of the package.
# Deleting that whole subsystem — and its tests — drops the internal/core average
# even though no surviving line lost coverage. The floors below sit just under the
# new measured coverage; raise them as the remaining core paths gain tests.
#
# This is a statement-coverage gate only. Onboarding performance and
# deterministic-output checks are a SEPARATE gate (`make perf-gate`,
# docs/agent-harness-baselines.md) — they assert byte-stable init receipts, not
# coverage, and CI runs both.
#
# Floors raised by the test-suite rebuild (spec.md §5). The reorg consolidated
# helpers and gap-filled dark paths — notably bringing internal/testharness from
# ~8% to >80% — so the ratchet moves up with the measured coverage. The new
# floors sit just under the post-rebuild measured values. Wave 2 adds gates for
# internal/cmd and internal/worker so orchestration glue and its process seam can
# no longer silently lose tests.
#
# Usage: ./scripts/coverage-check.sh
#   OVERALL_MIN  minimum total statement coverage          (default 71)
#   CORE_MIN     minimum internal/core coverage            (default 73)
#   CMD_MIN      minimum internal/cmd coverage             (default 61)
#   WORKER_MIN   minimum internal/worker coverage          (default 90)
#   MCP_MIN      minimum internal/mcp coverage             (default 87)
#   HARNESS_MIN  minimum internal/testharness coverage     (default 80)
set -euo pipefail

OVERALL_MIN="${OVERALL_MIN:-71}"
CORE_MIN="${CORE_MIN:-73}"
CMD_MIN="${CMD_MIN:-61}"
WORKER_MIN="${WORKER_MIN:-90}"
MCP_MIN="${MCP_MIN:-87}"
HARNESS_MIN="${HARNESS_MIN:-80}"

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

pct() { go tool cover -func="$1" | awk '/^total:/ {sub(/%/,"",$3); print $3}'; }

# Measure overall and internal/core independently so the per-package gate is a
# true weighted package coverage, not an unweighted average of -func lines.
go test ./... -coverprofile=coverage.out >/dev/null
overall="$(pct coverage.out)"

go test ./internal/core/... -coverprofile=coverage-core.out >/dev/null
core="$(pct coverage-core.out)"

go test ./internal/cmd/... -coverprofile=coverage-cmd.out >/dev/null
cmd="$(pct coverage-cmd.out)"

go test ./internal/worker/... -coverprofile=coverage-worker.out >/dev/null
worker="$(pct coverage-worker.out)"

go test ./internal/mcp/... -coverprofile=coverage-mcp.out >/dev/null
mcp="$(pct coverage-mcp.out)"

go test ./internal/testharness/... -coverprofile=coverage-harness.out >/dev/null
harness="$(pct coverage-harness.out)"

fail=0
check() {
  local name="$1" got="$2" min="$3"
  if awk -v g="$got" -v m="$min" 'BEGIN { exit !(g + 0 < m + 0) }'; then
    printf 'FAIL  %-14s %5s%% < %s%% floor\n' "$name" "$got" "$min"
    fail=1
  else
    printf 'ok    %-14s %5s%% >= %s%% floor\n' "$name" "$got" "$min"
  fi
}

check "overall" "$overall" "$OVERALL_MIN"
check "internal/core" "$core" "$CORE_MIN"
check "internal/cmd" "$cmd" "$CMD_MIN"
check "internal/worker" "$worker" "$WORKER_MIN"
check "internal/mcp" "$mcp" "$MCP_MIN"
check "internal/testharness" "$harness" "$HARNESS_MIN"

if [[ "$fail" -ne 0 ]]; then
  echo "coverage floor not met — add tests or, if intentional, lower the floor with justification." >&2
  exit 1
fi
echo "coverage floors met."
