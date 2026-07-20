#!/usr/bin/env sh
# regress-domains.sh — per-domain best-practice regression.
#
# Each wave's *owned invariant* is re-asserted black-box against a freshly
# built binary, in a throwaway copy of the tree so probes that mutate `.specd/`
# never touch the working repo. Exits non-zero on the FIRST violation.
#
#   W1 ADR-7 mode       unknown --mode is rejected (enum enforced)
#   W2 trust boundary   `brain start` is fail-closed on default config
#   W3 records          `decision` without --text is a usage error
#   W4 gates            `check` on a fresh scaffold rejects placeholder EARS
#   W5 surface          bare verb count == 33 (ADR-scoped surface)
#   W6 release          `--version` prints a stamp
#   W7 conformance      `report --proof` is a deterministic lifecycle projection
#   AD driveability     authoring gates, evidence close path, actionable waits/help/MCP
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RUN=$(mktemp -d)
trap 'rm -rf "$RUN"' EXIT

(cd "$ROOT" && tar --exclude=.git --exclude='*.log' -cf - .) | (cd "$RUN" && tar -xf -)
rm -rf "$RUN/.specd/specs/demo"
cd "$RUN"

go build -o "$RUN/specd" . >/dev/null 2>&1 || { echo "W?: build failed" >&2; exit 1; }
SPECD="$RUN/specd"
go build -o "$RUN/evalstatusassert" ./internal/testutil/evalstatusassert >/dev/null 2>&1 || { echo "AD-R2: eval status assertion helper build failed" >&2; exit 1; }
EVAL_STATUS_ASSERT="$RUN/evalstatusassert"

violation() { printf 'VIOLATION %s: %s\n' "$1" "$2" >&2; exit 1; }
pass() { printf 'ok  %s  %s\n' "$1" "$2"; }

# Agent-driveability probes use only public CLI/MCP surfaces. Fixtures are
# authored through `new` plus normal spec artifacts; lifecycle state advances
# only through `approve`/`mode` (never by editing state.json).
author_ad_spec() {
	slug=$1
	task_row=$2
	"$SPECD" new "$slug" >/dev/null 2>&1 || violation AD "could not scaffold $slug"
	cat >".specd/specs/$slug/requirements.md" <<EOF
# Requirements — $slug

## R1 — Driveability probe

- owner: test
- priority: must
- risk: low

- R1.1: When a probe runs, the system shall expose deterministic guidance.
EOF
	cat >".specd/specs/$slug/design.md" <<EOF
# Design — $slug

- references: R1, R1.1

## Modules
The probe uses public commands.

## On-disk contracts
Spec artifacts remain authoritative.

## Invariants
Output is deterministic.
EOF
	cat >".specd/specs/$slug/tasks.md" <<EOF
# Tasks — $slug

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | evidence |
|---|---|---|---|---|---|---|---|---|---|---|
$task_row
EOF
}

approve_ad_to_tasks() {
	slug=$1
	"$SPECD" approve "$slug" >/dev/null 2>&1 || violation AD "requirements approval failed for $slug"
	"$SPECD" approve "$slug" >/dev/null 2>&1 || violation AD "design approval failed for $slug"
}

git init -q
git config user.email specd-regress@example.test
git config user.name specd-regress
git add .
git commit -qm 'regression fixture'

# R1 — malformed quality declarations fail during `check`, with enum + format
# guidance available before execution/completion.
author_ad_spec ad-quality '| T1 | scout | requirements.md | - | true | R1.1 | R1, R1.1 | feature | low | low | tests |'
if quality_out=$("$SPECD" check ad-quality 2>&1); then
	violation AD-R1 "check accepted malformed evidence declaration"
fi
for expected in 'QUALITY_DECLARATION_INVALID' 'test, output_eval, trajectory_eval, review' 'class/check-id'; do
	printf '%s\n' "$quality_out" | grep -Fq "$expected" || violation AD-R1 "quality refusal missing $expected"
done
pass AD-R1 "check rejects malformed evidence with enum/format guidance"

# R2 — normal verify stamps test-class EvidenceEnvelopeV1 at current HEAD and
# that envelope closes completion without an external eval import.
author_ad_spec ad-evidence '| T1 | scout | requirements.md | - | true | R1.1 | R1, R1.1 | feature | low | low | test/unit |'
approve_ad_to_tasks ad-evidence
"$SPECD" approve ad-evidence >/dev/null 2>&1 || violation AD-R2 "tasks approval failed for evidence probe"
verify_out=$("$SPECD" verify ad-evidence T1 2>&1) || violation AD-R2 "passing task verify failed"
printf '%s\n' "$verify_out" | grep -Fq 'specd complete-task ad-evidence T1' || violation AD-R2 "satisfied test contract lacked completion hint"
eval_line=$("$SPECD" eval status ad-evidence --json 2>&1) || violation AD-R2 "eval status failed"
printf '%s\n' "$eval_line" | "$EVAL_STATUS_ASSERT" ad-evidence T1 unit || violation AD-R2 "stamped envelope JSON contract invalid"
"$SPECD" complete-task ad-evidence T1 >/dev/null 2>&1 || violation AD-R2 "stamped test envelope did not permit completion"
pass AD-R2 "passing test/* verify stamps envelope and completes"

# R3 — controller wait reasons are mutually distinct and each names its remedy.
cat >project.yml <<'EOF'
version: 1
agent: claude
verify:
  timeout_seconds: 600
orchestration:
  enabled: true
EOF
author_ad_spec ad-wait-auth '| T1 | craftsman | requirements.md | - | go test ./... | R1.1 | R1, R1.1 | feature | low | low |  |'
approve_ad_to_tasks ad-wait-auth
"$SPECD" mode ad-wait-auth orchestrated >/dev/null 2>&1 || violation AD-R3 "could not enable auth-wait orchestration"
"$SPECD" brain start ad-wait-auth >/dev/null 2>&1 || violation AD-R3 "could not start auth-wait brain"
auth_wait=$("$SPECD" brain step ad-wait-auth 2>&1) || violation AD-R3 "authority wait step failed"
printf '%s\n' "$auth_wait" | grep -Fq 'dispatch authority absent' || violation AD-R3 "authority wait reason conflated"
printf '%s\n' "$auth_wait" | grep -Fq 'specd brain run <slug> --authority' || violation AD-R3 "authority wait remedy absent"

author_ad_spec ad-wait-frontier ''
approve_ad_to_tasks ad-wait-frontier
"$SPECD" mode ad-wait-frontier orchestrated >/dev/null 2>&1 || violation AD-R3 "could not enable frontier-wait orchestration"
"$SPECD" brain start ad-wait-frontier >/dev/null 2>&1 || violation AD-R3 "could not start frontier-wait brain"
frontier_wait=$("$SPECD" brain step ad-wait-frontier --authority 2>&1) || violation AD-R3 "frontier wait step failed"
printf '%s\n' "$frontier_wait" | grep -Fq 'frontier empty' || violation AD-R3 "frontier wait reason conflated"
printf '%s\n' "$frontier_wait" | grep -Fq 'specd status <slug> --guide' || violation AD-R3 "frontier wait remedy absent"

author_ad_spec ad-wait-worker '| T1 | craftsman | requirements.md | - | go test ./... | R1.1 | R1, R1.1 | feature | low | low |  |'
approve_ad_to_tasks ad-wait-worker
"$SPECD" mode ad-wait-worker orchestrated >/dev/null 2>&1 || violation AD-R3 "could not enable worker-wait orchestration"
"$SPECD" brain start ad-wait-worker >/dev/null 2>&1 || violation AD-R3 "could not start worker-wait brain"
mv .claude/agents/pinky-craftsman.md .claude/agents/pinky-craftsman.md.regress
worker_wait=$("$SPECD" brain step ad-wait-worker --authority 2>&1) || violation AD-R3 "worker wait step failed"
mv .claude/agents/pinky-craftsman.md.regress .claude/agents/pinky-craftsman.md
printf '%s\n' "$worker_wait" | grep -Fq 'no worker definition for active harness' || violation AD-R3 "worker wait reason conflated"
printf '%s\n' "$worker_wait" | grep -Fq 'specd init --repair' || violation AD-R3 "worker wait remedy absent"
pass AD-R3 "brain wait reasons distinguish authority/frontier/worker"

# R4 — multi-operation help is discoverable and exits successfully.
brain_help=$("$SPECD" brain --help 2>&1) || violation AD-R4 "brain --help failed"
for expected in 'brain.start' 'brain.run' 'brain.status' 'brain.report'; do
	printf '%s\n' "$brain_help" | grep -Fq "$expected" || violation AD-R4 "brain help missing $expected"
done
pass AD-R4 "brain --help exits 0 with operation palette"

# R5 — executing-phase coverage refusal teaches the exact refs-column contract.
author_ad_spec ad-coverage '| T1 | scout | requirements.md | - | true | unrelated | R1 | feature | low | low |  |'
approve_ad_to_tasks ad-coverage
if coverage_out=$("$SPECD" approve ad-coverage 2>&1); then
	violation AD-R5 "executing approval accepted uncovered R1.1"
fi
for expected in 'tasks.md `refs` column' 'R1.1' "add each id to an implementing task's \`refs\` column" 'kind: deferred'; do
	printf '%s\n' "$coverage_out" | grep -Fq "$expected" || violation AD-R5 "coverage refusal missing $expected"
done
pass AD-R5 "coverage refusal names refs, gaps, and both fixes"

# R7 — MCP rejects flag-shaped positional args and teaches property spelling.
mcp_request='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"status","arguments":{"args":["ad-quality","--guide"]}}}'
mcp_out=$(printf '%s\n' "$mcp_request" | "$SPECD" mcp 2>&1) || violation AD-R7 "MCP flag-guard probe failed"
for expected in 'invalid `args` element \"--guide\"' '`guide: true`' 'not inside `args`'; do
	printf '%s\n' "$mcp_out" | grep -Fq "$expected" || violation AD-R7 "MCP refusal missing $expected"
done
pass AD-R7 "MCP rejects --flag in args with property remedy"

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

# AD-R8 — diff-scope is a core invariant (agent-driver-protocol R4.5). The check
# must run on the DEFAULT profile: it previously lived behind
# `lifecycle.profile = production`, so a probe that enables production would pass
# while the invariant this asserts was absent. No project.yml profile is set here.
#
# A baseline is what arms the check, and `session open` is what pins one, so the
# probe opens a session and then completes with an undeclared file present.
author_ad_spec ad-diffscope '| T1 | craftsman | ad-diffscope-declared.txt | - | test -f ad-diffscope-declared.txt | R1.1 | R1, R1.1 | feature | low | low | test/unit |'
approve_ad_to_tasks ad-diffscope
"$SPECD" approve ad-diffscope >/dev/null 2>&1 || violation AD-R8 "tasks approval failed for diff-scope probe"

printf 'declared\n' >ad-diffscope-declared.txt
git add -A >/dev/null 2>&1
git commit -qm 'diff-scope probe baseline' >/dev/null 2>&1 || violation AD-R8 "could not commit probe baseline"
"$SPECD" session open ad-diffscope --driver regress >/dev/null 2>&1 || violation AD-R8 "session open failed"

# An open session makes the protocol bindings mandatory, so the probe drives the
# real chain: ack the context, mint a nonce, then complete. The declared file
# alone must complete cleanly, or the violation probe below proves nothing.
printf 'edited\n' >ad-diffscope-declared.txt
"$SPECD" verify ad-diffscope T1 >/dev/null 2>&1 || violation AD-R8 "declared-only verify failed"
if "$SPECD" complete-task ad-diffscope T1 >/dev/null 2>&1; then
	violation AD-R8 "an open session accepted a completion carrying no bindings"
fi
"$SPECD" session ack ad-diffscope T1 --tokens 100 >/dev/null 2>&1 || violation AD-R8 "context ack failed"
ds_id=$("$SPECD" session show ad-diffscope --json | sed -n 's/.*"id": "\([^"]*\)".*/\1/p' | head -1)
ds_nonce=$("$SPECD" session action ad-diffscope --json | sed -n 's/.*"nonce": "\([^"]*\)".*/\1/p' | head -1)
[ -n "$ds_id" ] && [ -n "$ds_nonce" ] || violation AD-R8 "session action did not mint bindings"
"$SPECD" complete-task ad-diffscope T1 --session "$ds_id" --nonce "$ds_nonce" >/dev/null 2>&1 || violation AD-R8 "declared-only change was refused"
if "$SPECD" complete-task ad-diffscope T1 --session "$ds_id" --nonce "$ds_nonce" >/dev/null 2>&1; then
	violation AD-R8 "a spent nonce was accepted a second time"
fi

# Now the real assertion, on a second spec: an undeclared sibling must refuse.
author_ad_spec ad-diffscope-violation '| T1 | craftsman | ad-diffscope-declared.txt | - | test -f ad-diffscope-declared.txt | R1.1 | R1, R1.1 | feature | low | low | test/unit |'
approve_ad_to_tasks ad-diffscope-violation
"$SPECD" approve ad-diffscope-violation >/dev/null 2>&1 || violation AD-R8 "tasks approval failed for violation probe"
git add -A >/dev/null 2>&1
git commit -qm 'violation probe baseline' >/dev/null 2>&1 || violation AD-R8 "could not commit violation baseline"
"$SPECD" session open ad-diffscope-violation --driver regress >/dev/null 2>&1 || violation AD-R8 "session open failed"

printf 'edited again\n' >ad-diffscope-declared.txt
printf 'undeclared\n' >ad-diffscope-undeclared.txt
"$SPECD" verify ad-diffscope-violation T1 >/dev/null 2>&1 || violation AD-R8 "violation probe verify failed"
"$SPECD" session ack ad-diffscope-violation T1 --tokens 100 >/dev/null 2>&1 || violation AD-R8 "context ack failed"
vs_id=$("$SPECD" session show ad-diffscope-violation --json | sed -n 's/.*"id": "\([^"]*\)".*/\1/p' | head -1)
vs_nonce=$("$SPECD" session action ad-diffscope-violation --json | sed -n 's/.*"nonce": "\([^"]*\)".*/\1/p' | head -1)
if scope_out=$("$SPECD" complete-task ad-diffscope-violation T1 --session "$vs_id" --nonce "$vs_nonce" 2>&1); then
	violation AD-R8 "completion accepted an undeclared file on the default profile"
fi
printf '%s\n' "$scope_out" | grep -Fq 'OUTSIDE_SCOPE' || violation AD-R8 "scope refusal is not typed OUTSIDE_SCOPE"
printf '%s\n' "$scope_out" | grep -Fq 'ad-diffscope-undeclared.txt' || violation AD-R8 "scope refusal does not name the undeclared file"

# R4.3 — harness-owned artifacts are refused even though .specd/ runtime state
# (evidence, session, state.json) is written by specd during the same task.
printf '\nhand-edited\n' >>.specd/specs/ad-diffscope-violation/tasks.md
vs_nonce2=$("$SPECD" session action ad-diffscope-violation --json | sed -n 's/.*"nonce": "\([^"]*\)".*/\1/p' | head -1)
if marker_out=$("$SPECD" complete-task ad-diffscope-violation T1 --session "$vs_id" --nonce "$vs_nonce2" 2>&1); then
	violation AD-R8 "completion accepted a hand-edited tasks.md"
fi
printf '%s\n' "$marker_out" | grep -Fq 'harness-owned' || violation AD-R8 "task-marker refusal does not name the harness-owned rule"
pass AD-R8 "diff-scope + session bindings enforce on the default profile (undeclared, harness-owned, unbound, replayed)"

# Domain 10 W3 — public adapter contract remains executable without internal imports.
./scripts/adapter-conformance.sh >/dev/null 2>&1 || {
	violation 10-W3 "adapter conformance contract regressed"
}
pass 10-W3 "adapter/v1 third-party conformance holds"

echo "regress-domains: all per-domain invariants hold"
