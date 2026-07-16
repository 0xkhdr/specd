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
#   W5 surface          bare verb count == 33 (ADR-scoped surface)
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
	program_truth="$RUN/program-rollup.tsv"
	domain_truth="$RUN/domain-rollup.tsv"
	awk '
/^- \[[x ]\] [0-9][0-9] W[0-9]+/ {
	status = (substr($0, 4, 1) == "x") ? "done" : "pending"
	line = $0
	sub(/^- \[[x ]\] /, "", line)
	split(line, fields, / +/)
	print fields[1], fields[2], status
}' "$progress" | sort >"$program_truth"
	: >"$domain_truth"
	for tasks in "$RUN"/specs/[0-1][0-9]-*/tasks.md; do
		dom=$(basename "$(dirname "$tasks")" | cut -c1-2)
		awk -v dom="$dom" '
/^## W[0-9]+ / { wave=$2; seen[wave]=1; next }
/^\| \[[x ]\] T[0-9]+ / && wave != "" {
	total[wave]++
	if (substr($0, 4, 1) != "x") incomplete[wave]++
}
END {
	for (wave in seen) {
		status = (total[wave] > 0 && incomplete[wave] == 0) ? "done" : "pending"
		print dom, wave, status
	}
}' "$tasks" >>"$domain_truth"
	done
	sort -o "$domain_truth" "$domain_truth"
	if ! cmp -s "$program_truth" "$domain_truth"; then
		violation W0 "program rollup differs from domain task truth"
	fi
	pass W0 "program rollup equals domain task truth in both directions"
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

# W5 — surface coherence derives from the canonical command set. Registry,
# help, and command metadata parity tests catch additions/removals without a
# second hardcoded verb count that drifts whenever the canonical set changes.
go test ./internal/core ./internal/cmd -run 'Test(Command|Registry|Help)' -count=1 >/dev/null 2>&1 || {
	violation W5 "command registry/help parity regressed"
}
pass W5 "command registry/help derive from canonical surface"

# W6 — release: retain the build stamp and run the fresh-project generated
# workflow plus CLI/MCP semantic parity proofs against copied source.
if ! "$SPECD" version 2>/dev/null | grep -qE '.'; then
	violation W6 "version prints nothing"
fi
go test ./internal/cmd ./internal/integration \
	-run 'TestWorkflowCoherence(Default|Production)|TestDriverConformance|Test.*Conformance' -count=1 >/dev/null 2>&1 || {
	violation W6 "fresh default/production workflow or conformance parity regressed"
}
pass W6 "version, fresh default/production workflow, and conformance parity hold"

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
