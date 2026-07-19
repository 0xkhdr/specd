#!/usr/bin/env bash
# stress.sh (SPEC-01 T-01-04) — cross-process contention stress harness.
#
# One parameterized entry point: `stress.sh <domain>`. Each domain builds a
# fresh binary in a throwaway tree, drives concurrent racers, then asserts a
# domain-specific concurrency invariant. Exits non-zero on the first violation.
#
# Domains:
#   default          general state.json CAS: records landed == final revision
#   acp              ACP + run ledger append integrity under concurrent append
#   orchestration    session-revision CAS: one winner, no stale lease
#   program          per-spec isolation across M crashed specs (no cross bleed)
#   brain-recovery   crashed-checkpoint reclaim: exactly one re-dispatch
#   checkpoint-fault crash mid-checkpoint + stale lease: one dispatch, reclaimed
set -euo pipefail

domain=${1:-}
valid="default acp orchestration program brain-recovery checkpoint-fault"
ok=0
for d in $valid; do [ "$domain" = "$d" ] && ok=1; done
if [ "$ok" -ne 1 ]; then
	echo "usage: stress.sh <domain>   valid domains: $valid" >&2
	exit 2
fi

# --- shared setup (runs for every domain) ---
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=$(mktemp -d)/specd
go build -o "$bin" "$root"

tree=$(mktemp -d)
trap 'rm -rf "$tree"' EXIT
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
racers=8

# scaffold a single crashed "demo" spec and race N `brain resume` at it.
# $1 = JSON contents of the session's "leases" array (may be empty).
scaffold_and_resume() {
	spec="$tree/.specd/specs/demo"
	mkdir -p "$spec"
	printf '{"id":"demo","revision":1,"state":"running","leases":[%s]}\n' "$1" > "$spec/session.json"
	printf '{"session_id":"demo","step":1,"mission_id":"demo.s1.T1","task_id":"T1","time":"%s"}\n' "$now" > "$spec/checkpoint.json"
	: > "$spec/acp.jsonl"
	i=0
	while [ "$i" -lt "$racers" ]; do
		( cd "$tree" && "$bin" brain resume demo >/dev/null 2>&1 || true ) &
		i=$((i + 1))
	done
	wait
}

case "$domain" in
default)
	# Races N concurrent `specd decision` writers at one spec's state.json. Each
	# mutation serialises through the reentrant per-spec lock and a CAS on the
	# revision counter, written atomically. Invariant: revision starts at 0 and
	# every landed decision bumps it by exactly one, so the count of landed
	# records must equal the final revision. A lost update or double-count breaks
	# the equality. Each writer stamps a unique "stressmark-<i>".
	cd "$tree"
	"$bin" init >/dev/null 2>&1
	"$bin" new demo >/dev/null 2>&1

	writers=12
	i=0
	while [ "$i" -lt "$writers" ]; do
		( "$bin" decision demo --text "stressmark-$i" >/dev/null 2>&1 || true ) &
		i=$((i + 1))
	done
	wait

	state="$tree/.specd/specs/demo/state.json"
	[ -s "$state" ] || { echo "stress: state.json missing/empty after contention" >&2; exit 1; }

	records=$(grep -o 'stressmark-' "$state" | wc -l | tr -d ' ')
	revision=$(sed -n 's/.*"revision"[ ]*:[ ]*\([0-9][0-9]*\).*/\1/p' "$state" | head -n1)
	if [ "$records" != "$revision" ]; then
		echo "stress: landed records=$records != revision=$revision (lost update or double-count)" >&2
		exit 1
	fi

	echo "stress: ok ($writers racing writers, records==revision==$records, no lost update)"
	;;

acp)
	# Races N concurrent `brain resume` appending to one spec's acp.jsonl. Focus
	# is ledger LINE integrity under concurrent append (not the dispatch count,
	# which brain-recovery owns): the append path must never leave a torn or
	# blank line, and every event's `seq` must be unique.
	scaffold_and_resume ""
	spec="$tree/.specd/specs/demo"

	ledger="$spec/acp.jsonl"
	lines=0
	while IFS= read -r line; do
		[ -n "$line" ] || { echo "stress-acp: blank line in ledger (torn append)" >&2; exit 1; }
		case "$line" in
			\{*\}) ;;
			*) echo "stress-acp: malformed ledger line: $line" >&2; exit 1 ;;
		esac
		case "$line" in
			*'"seq":'*) ;;
			*) echo "stress-acp: ledger line missing seq: $line" >&2; exit 1 ;;
		esac
		lines=$((lines + 1))
	done < "$ledger"

	seqs=$(grep -o '"seq":[0-9][0-9]*' "$ledger" | sed 's/.*://')
	uniq=$(printf '%s\n' "$seqs" | sort -u | grep -c '.' || true)
	total=$(printf '%s\n' "$seqs" | grep -c '.' || true)
	if [ "$uniq" != "$total" ]; then
		echo "stress-acp: duplicate seq values in ledger ($total events, $uniq distinct)" >&2
		exit 1
	fi

	# Run ledger (spec 07 R2.4): racing writers must not duplicate an attempt, and a
	# crash mid-append must leave no partial line. Race concurrent `specd verify`,
	# each of which allocates one run/attempt under the spec lock, then assert the
	# ledger is well-formed with a single run_id and no duplicate attempt.
	printf '{"schema_version":1,"slug":"demo","mode":"default","status":"tasks","phase":"plan","revision":0,"records":{}}\n' > "$spec/state.json"
	printf '| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n' > "$spec/tasks.md"

	i=0
	while [ "$i" -lt "$racers" ]; do
		( cd "$tree" && "$bin" verify demo T1 >/dev/null 2>&1 || true ) &
		i=$((i + 1))
	done
	wait

	runs="$spec/runs.jsonl"
	runlines=0
	while IFS= read -r line; do
		[ -n "$line" ] || { echo "stress-acp: blank line in run ledger (torn append)" >&2; exit 1; }
		case "$line" in
			\{*\}) ;;
			*) echo "stress-acp: malformed run ledger line: $line" >&2; exit 1 ;;
		esac
		case "$line" in
			*'"run_id":"'*'"'*) ;;
			*) echo "stress-acp: run ledger line missing run_id: $line" >&2; exit 1 ;;
		esac
		runlines=$((runlines + 1))
	done < "$runs"

	attempts=$(grep -o '"attempt":[0-9][0-9]*' "$runs" | sed 's/.*://')
	runiq=$(printf '%s\n' "$attempts" | sort -u | grep -c '.' || true)
	runtotal=$(printf '%s\n' "$attempts" | grep -c '.' || true)
	if [ "$runiq" != "$runtotal" ]; then
		echo "stress-acp: duplicate attempt in run ledger ($runtotal runs, $runiq distinct)" >&2
		exit 1
	fi
	runidcount=$(grep -o '"run_id":"[^"]*"' "$runs" | sort -u | grep -c '.' || true)
	if [ "$runidcount" != "1" ]; then
		echo "stress-acp: expected one run chain, found $runidcount run_id(s)" >&2
		exit 1
	fi

	echo "stress-acp: ok ($racers racing appends, $lines well-formed line(s), $uniq distinct seq; $runlines run(s), $runiq distinct attempt(s), 1 chain)"
	;;

orchestration)
	# Races N concurrent `brain resume` at one session and inspects the controller
	# state file (not the ledger). Focus is the session-revision CAS: concurrent
	# controllers must not corrupt session.json, and exactly one may win the
	# transition (advance the revision past its scaffolded value).
	scaffold_and_resume ""
	spec="$tree/.specd/specs/demo"

	session="$spec/session.json"
	body=$(cat "$session")
	case "$body" in
		\{*\}*) ;;
		*) echo "stress-orchestration: session.json is not a JSON object after contention" >&2; exit 1 ;;
	esac

	rev=$(sed -n 's/.*"revision"[ ]*:[ ]*\([0-9][0-9]*\).*/\1/p' "$session" | head -n1)
	[ -n "$rev" ] || { echo "stress-orchestration: session.json lost its revision field" >&2; exit 1; }
	if [ "$rev" -le 1 ]; then
		echo "stress-orchestration: session revision is $rev, expected an advance past scaffolded 1" >&2
		exit 1
	fi

	dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
	if [ "$dispatches" != "1" ]; then
		echo "stress-orchestration: expected exactly one dispatch, got $dispatches" >&2
		exit 1
	fi

	# No stale live leases (gap 5.4): the winning resume reclaims orphaned leases and
	# clears them from session.json, so after contention settles no lease object may
	# linger. A retained lease would show as a phantom live worker in `brain status`.
	if grep -q '"worker_id"' "$session"; then
		echo "stress-orchestration: session.json retains a stale lease after resume" >&2
		exit 1
	fi

	echo "stress-orchestration: ok ($racers racers, session revision advanced 1->$rev once, one dispatch, no stale leases)"
	;;

program)
	# Scaffolds M distinct crashed specs and races concurrent `brain resume`
	# across ALL of them at once. Focus is per-spec isolation: the per-spec lock
	# must keep concurrent recovery of one spec from bleeding into another. Each
	# spec must converge independently to exactly one dispatch of its own mission.
	specs="alpha bravo charlie"
	for s in $specs; do
		spec="$tree/.specd/specs/$s"
		mkdir -p "$spec"
		printf '{"id":"%s","revision":1,"state":"running","leases":[]}\n' "$s" > "$spec/session.json"
		printf '{"session_id":"%s","step":1,"mission_id":"%s.s1.T1","task_id":"T1","time":"%s"}\n' "$s" "$s" "$now" > "$spec/checkpoint.json"
		: > "$spec/acp.jsonl"
	done

	# Race resumes across every spec at once: each spec gets several racers,
	# interleaved with the others.
	i=0
	while [ "$i" -lt 4 ]; do
		for s in $specs; do
			( cd "$tree" && "$bin" brain resume "$s" >/dev/null 2>&1 || true ) &
		done
		i=$((i + 1))
	done
	wait

	for s in $specs; do
		ledger="$tree/.specd/specs/$s/acp.jsonl"
		dispatches=$(grep -c "\"mission_id\":\"$s.s1.T1\"" "$ledger" || true)
		if [ "$dispatches" != "1" ]; then
			echo "stress-program: spec $s expected exactly one dispatch, got $dispatches" >&2
			exit 1
		fi
		# No other spec's mission should have leaked into this ledger.
		foreign=$(grep -c '"mission_id"' "$ledger" || true)
		if [ "$foreign" != "1" ]; then
			echo "stress-program: spec $s ledger has $foreign mission events (cross-spec bleed)" >&2
			exit 1
		fi
	done

	echo "stress-program: ok (3 specs recovered concurrently, one isolated dispatch each)"
	;;

brain-recovery)
	# Scaffolds a crashed controller state (a write-ahead checkpoint whose mission
	# never reached the ledger), then races N concurrent `brain resume` at it.
	# Invariant: exactly one resume re-issues the mission and the ledger carries
	# that dispatch exactly once — no double-dispatch under a race; one holder
	# wins the session-revision CAS.
	scaffold_and_resume ""
	spec="$tree/.specd/specs/demo"

	dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
	if [ "$dispatches" != "1" ]; then
		echo "stress-brain-recovery: expected exactly one dispatch of demo.s1.T1, got $dispatches" >&2
		exit 1
	fi

	echo "stress-brain-recovery: ok ($racers racing resumes converged on one dispatch)"
	;;

checkpoint-fault)
	# Injects a fault: a write-ahead checkpoint whose mission never reached the
	# ledger, PLUS a stale lease left behind by the crashed worker. Then races N
	# concurrent `brain resume` at it. Recovery must reconcile the crash cleanly:
	# no double-claim (one dispatch) and no orphaned lease (reclaimed).
	scaffold_and_resume '{"task_id":"T1","mission_id":"demo.s1.T1","holder":"pinky-crashed"}'
	spec="$tree/.specd/specs/demo"

	dispatches=$(grep -c '"mission_id":"demo.s1.T1"' "$spec/acp.jsonl" || true)
	if [ "$dispatches" != "1" ]; then
		echo "stress-checkpoint-fault: double-claim — expected one dispatch, got $dispatches" >&2
		exit 1
	fi

	if grep -q '"holder"' "$spec/session.json"; then
		echo "stress-checkpoint-fault: orphaned lease survived recovery:" >&2
		cat "$spec/session.json" >&2
		exit 1
	fi

	echo "stress-checkpoint-fault: ok ($racers racers, one dispatch, stale lease reclaimed)"
	;;
esac
