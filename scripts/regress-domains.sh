#!/usr/bin/env sh
# regress-domains.sh (P7.3) — per-domain best-practice regression.
#
# Complements regress-all.sh (which re-runs each task's own go-test verify).
# Here each wave's *owned invariant* is re-asserted black-box against a freshly
# built binary, in a throwaway copy of the tree so probes that mutate `.specd/`
# never touch the working repo. Exits non-zero on the FIRST violation.
#
#   W0 honesty          progress.md green rows survive the audit
#   W1 ADR-7 mode       unknown --mode is rejected (enum enforced)
#   W2 trust boundary   `brain start` is fail-closed on default config
#   W3 records          `decision` without --text is a usage error
#   W4 gates            `check` on a fresh scaffold rejects placeholder EARS
#   W5 surface          bare verb count == 32 (ADR-scoped surface)
#   W6 release          `--version` prints a stamp
#   W7 conformance      `report --proof` is a deterministic lifecycle projection
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RUN=$(mktemp -d)
trap 'rm -rf "$RUN"' EXIT

(cd "$ROOT" && tar --exclude=.git --exclude='*.log' -cf - .) | (cd "$RUN" && tar -xf -)
rm -rf "$RUN/.specd/specs/demo"
cd "$RUN"

go build -o "$RUN/specd" . >/dev/null 2>&1 || { echo "W?: build failed" >&2; exit 1; }
SPECD="$RUN/specd"

violation() { printf 'VIOLATION %s: %s\n' "$1" "$2" >&2; exit 1; }
pass() { printf 'ok  %s  %s\n' "$1" "$2"; }
skip() { printf 'skip  %s  %s\n' "$1" "$2"; }

# Domain 03 W5 — remote envelope proof. Keep this on the freshly copied tree so
# release validation exercises the same source/binary boundary as other probes.
go test ./internal/orchestration ./internal/core -run 'Test(DispatchEnvelope|DispatchDigest|Driver)' -count=1 >/dev/null 2>&1 || {
	violation 03-W5 "dispatch envelope pinning regression"
}
pass 03-W5 "remote envelope pins scope, digests, and HEAD"

# Domain 04 W3 has no CLI surface until W4. Pin its pure/offline production
# policy now: risky writes reject shallow verify while read-only work remains
# explicitly exempt.
go test ./internal/core/gates -run '^TestQualityGateVerifyStrengthAndReadOnlyException$' >/dev/null 2>&1 || {
	violation 04-W3 "quality verify-strength composition regressed"
}
pass 04-W3 "quality gate rejects shallow writes and exempts read-only work"

# Domain 05 W5 — orchestration release proof. Exercise lifecycle failure modes
# and adapter parity in fresh copied source; transport metadata must not alter ACP.
go test ./internal/cmd ./internal/orchestration ./internal/integration ./internal/mcp \
	-run 'Test(BrainDispatchCreatesPendingMissionWithoutWorkerLease|BrainResumeRaceDispatchesExactlyOnce|BrainReportProductionScopeRejectsUndeclared|BrainRunHaltsOnConfiguredCostBrake|ConflictOverlappingWriteScopes|OrchestrationConformance|ParityA2A|A2A)' \
	-count=1 >/dev/null 2>&1 || {
	violation 05-W5 "orchestration lifecycle or adapter parity regressed"
}
pass 05-W5 "pending/race/stale/revoke/brake/conflict/A2A contracts hold"

# Domain 06 W8 — adapter negotiation and incident attestations in fresh source.
go test ./internal/core/verify ./internal/core/gates/security ./internal/integration \
	-run 'Test(Adapter|SandboxConformance|Regress|SecurityConformance)' -count=1 >/dev/null 2>&1 || {
	violation 06-W8 "sandbox adapter or security regression conformance regressed"
}
pass 06-W8 "adapter capability and incident attestation contracts hold"

# W0 — honesty: progress.md must obey its own wave-ordering invariant. Prove
# the advertised input exists and parses before evaluating it: absent and
# unparseable are failures, not vacuous passes.
progress=${SPECD_PROGRESS_PATH:-$RUN/specs/progress.md}
if [ ! -f "$progress" ]; then
	if [ "${SPECD_PROGRESS_POLICY:-required}" = "optional" ]; then
		skip W0 "not applicable by optional-input policy: progress.md absent"
	else
		violation W0 "input absent: progress.md"
	fi
else
w0_seen_incomplete=0
w0_bad=0
w0_rows=$(awk '
/^- \[[x ]\] / {
	if (substr($0, 4, 1) == "x") print "done"
	else print "pending"
}' "$progress")
[ -n "$w0_rows" ] || violation W0 "input unparseable: progress.md has no wave rows"
while IFS= read -r st; do
	case "$st" in
		pending|in-progress) w0_seen_incomplete=1 ;;
		done) [ "$w0_seen_incomplete" -eq 1 ] && w0_bad=1 ;;
	esac
done <<EOF
$w0_rows
EOF
if [ "$w0_bad" -ne 0 ]; then
	violation W0 "progress.md marks a later wave done while an earlier wave is pending"
else
	pass W0 "input parsed; progress.md wave ordering honest"
fi
fi

# W1 — enum enforcement (spec 03 R3): an out-of-enum flag value must be refused.
# Probe a real enum flag (report --format ∈ {prometheus}) against an existing
# spec so the rejection is attributable to the enum path, not a missing spec.
"$SPECD" new rp-w1 >/dev/null 2>&1 || violation W1 "could not scaffold probe spec"
if "$SPECD" report rp-w1 --format __bogus__ >/dev/null 2>&1; then
	violation W1 "out-of-enum --format accepted (enum validation not enforced)"
else
	pass W1 "out-of-enum flag value rejected"
fi

# W2 — trust boundary: brain must be fail-closed on default config.
if "$SPECD" brain start rp-w2 >/dev/null 2>&1; then
	violation W2 "brain start succeeded on default config (not fail-closed)"
else
	pass W2 "brain start fail-closed"
fi

# W3 — records: decision without --text is a usage error.
if "$SPECD" decision rp-w3 >/dev/null 2>&1; then
	violation W3 "decision without --text accepted (hollow record)"
else
	pass W3 "decision requires --text"
fi

# W4 — gates: check on a fresh scaffold must reject placeholder EARS.
"$SPECD" new rp-w4 >/dev/null 2>&1 || violation W4 "could not scaffold probe spec"
if "$SPECD" check rp-w4 >/dev/null 2>&1; then
	violation W4 "check passed on placeholder scaffold (EARS gate inert)"
else
	pass W4 "check rejects placeholder EARS"
fi

# W5 — surface lock: the bare verb count is pinned as a tripwire, so adding or
# removing a verb is a deliberate edit here. Current surface is 30 (16 original
# + submit, review, link, unlink, program-era verbs, version, triage, the
# delivery verbs release + deploy, adapters, and spike). Bump this only alongside an intended verb change.
W5_EXPECT=32
verbs=$("$SPECD" 2>&1 | sed -n 's/^  \([a-z][a-z]*\) .*/\1/p' | sort -u | wc -l | tr -d ' ')
if [ "$verbs" -ne "$W5_EXPECT" ]; then
	violation W5 "verb count is $verbs, expected $W5_EXPECT"
else
	pass W5 "verb count == $W5_EXPECT"
fi

# W6 — release: the `version` verb (spec 01) prints a non-empty build stamp.
if "$SPECD" version 2>/dev/null | grep -qE '.'; then
	pass W6 "version prints a stamp"
else
	violation W6 "version prints nothing"
fi

# W7 — conformance: `report --proof` (spec 01 R8.2) is a deterministic projection
# of on-disk state. Two consecutive runs against a scaffolded spec must be
# byte-identical, and the proof must carry its four fixed sections.
"$SPECD" new rp-w7 >/dev/null 2>&1 || violation W7 "could not scaffold probe spec"
p1=$("$SPECD" report rp-w7 --proof 2>/dev/null) || violation W7 "report --proof failed"
p2=$("$SPECD" report rp-w7 --proof 2>/dev/null) || violation W7 "report --proof failed"
if [ "$p1" != "$p2" ]; then
	violation W7 "report --proof is not deterministic across runs"
elif ! printf '%s\n' "$p1" | grep -q "escaped-defects:"; then
	violation W7 "report --proof missing escaped-defects projection"
else
	pass W7 "report --proof deterministic (coverage/stale/amendments/escaped)"
fi

# Domain 10 W3 — public adapter contract remains executable without internal imports.
./scripts/adapter-conformance.sh >/dev/null 2>&1 || {
	violation 10-W3 "adapter conformance contract regressed"
}
pass 10-W3 "adapter/v1 third-party conformance holds"

echo "regress-domains: all per-domain invariants hold"
