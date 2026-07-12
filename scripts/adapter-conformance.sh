#!/bin/sh
# Public adapter/v1 contract. Optional arg: third-party adapter executable.
# No Go package, jq, network, or provider SDK required.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
FIXTURES=$ROOT/internal/adapter/testdata
ADAPTER=${1:-$FIXTURES/third-party-adapter.sh}

fail() { printf 'adapter-conformance: FAIL: %s\n' "$1" >&2; exit 1; }
pass() { printf 'adapter-conformance: ok: %s\n' "$1"; }

[ -x "$ADAPTER" ] || fail "adapter is not executable: $ADAPTER"
[ -f "$FIXTURES/conformance-cases.json" ] || fail "scenario manifest missing"
[ -f "$FIXTURES/request_v1.json" ] || fail "request fixture missing"
[ -f "$FIXTURES/result_v1.json" ] || fail "result fixture missing"

# Certification boundary: adapter source cannot depend on this repo's private packages.
if grep -nE '(^|[[:space:]/])internal/' "$ADAPTER" >/dev/null 2>&1; then
	fail "adapter references private internal/ package"
fi
pass "third-party adapter has no internal/ import"

# Manifest must name every R9.2 validation scenario.
for case in no-adapters wrong-version malformed-output oversized-output timeout \
	crash stale-result restricted-export provider-outage a2a-mcp third-party-adapter; do
	grep -q '"id": "'$case'"' "$FIXTURES/conformance-cases.json" || \
		fail "scenario missing from manifest: $case"
done
pass "all R9.2 scenarios present"

# Public protocol proof: request on stdin; result on stdout; identity preserved.
output=$($ADAPTER <"$FIXTURES/request_v1.json") || fail "adapter exited non-zero"
case "$output" in
	*'"schema_version": "adapter/v1"'*) ;;
	*) fail "result does not declare adapter/v1" ;;
esac
for field in request_id correlation_id spec_slug task_id git_head adapter_name status exit_class; do
	printf '%s\n' "$output" | grep -q '"'$field'"' || fail "result missing field: $field"
done
case "$output" in
	*'"request_id": "req-1"'*'"correlation_id": "corr-1"'*) ;;
	*) fail "result does not preserve request identity" ;;
esac
case "$output" in
	*'"status": "succeeded"'*'"exit_class": "ok"'*) ;;
	*) fail "successful result has wrong status classes" ;;
esac
pass "third-party adapter request/result round trip"

# Restricted data must not appear in published default result fixture.
if grep -nE '"class": "(secret|source-content|prompt)"|"(secret|prompt|source_content)"[[:space:]]*:' \
	"$FIXTURES/result_v1.json" >/dev/null 2>&1; then
	fail "restricted data present in default result fixture"
fi
pass "default result excludes restricted data"
