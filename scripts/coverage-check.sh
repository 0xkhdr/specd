#!/usr/bin/env bash
# coverage-check.sh — enforce a coverage floor so a refactor cannot silently
# drop test coverage on the integrity-critical paths (Stage 07 F1).
#
# Floors are ratchets, not aspirations: they sit just below the current measured
# coverage to catch regressions without breaking on noise. Raise them as
# coverage improves; never lower them to make a red build pass.
#
# The documented long-term target (TESTING.md) is higher (85% overall / 95% for
# internal/core); these gates are the regression floor on the way there.
#
# Re-baselined when the `boot`/`enrich` subsystem was removed: those files
# (boot.go, boot_detectors.go, enrich.go, enrich_evidence.go) were heavily tested
# and carried internal/core's aggregate well above the rest of the package.
# Deleting that whole subsystem — and its tests — drops the internal/core average
# even though no surviving line lost coverage. The floors below sit just under the
# new measured coverage; raise them as the remaining core paths gain tests.
#
# Usage: ./scripts/coverage-check.sh
#   OVERALL_MIN  minimum total statement coverage   (default 59)
#   CORE_MIN     minimum internal/core coverage     (default 49)
set -euo pipefail

OVERALL_MIN="${OVERALL_MIN:-59}"
CORE_MIN="${CORE_MIN:-49}"

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

pct() { go tool cover -func="$1" | awk '/^total:/ {sub(/%/,"",$3); print $3}'; }

# Measure overall and internal/core independently so the per-package gate is a
# true weighted package coverage, not an unweighted average of -func lines.
go test ./... -coverprofile=coverage.out >/dev/null
overall="$(pct coverage.out)"

go test ./internal/core/... -coverprofile=coverage-core.out >/dev/null
core="$(pct coverage-core.out)"

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

if [[ "$fail" -ne 0 ]]; then
  echo "coverage floor not met — add tests or, if intentional, lower the floor with justification." >&2
  exit 1
fi
echo "coverage floors met."
