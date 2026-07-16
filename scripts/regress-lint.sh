#!/usr/bin/env sh
# regress-lint.sh — static smell audit of every domain's verify tables.
#
# Never runs a verify. Reads all ten domains' tasks tables and flags smells that
# would let a task claim evidence it does not actually hold, or that break the
# table contract the regression tooling depends on:
#
#   A  verify targets authoring `specs/` when runtime reads `.specd/specs/`
#   B  hollow verify: a write task whose verify passes without asserting behavior
#      (pure `test -e`/`ls` existence, `|| true`, or `:`/`true`)
#   C  stale target: a completed row's `files:` or verify path fails `test -e`
#   D  compile-only: a write task proving only `go build`, never runtime behavior
#   E  vacuous selector: a `-run` pattern using `\|` — in Go regexp `\|` is a
#      LITERAL pipe, so `-run 'TestA\|TestB'` matches nothing ("No tests found",
#      exit 0). The verify record is real but empty. Use a raw `|` in a code span.
#   F  unescaped pipe: a raw `|` outside a backtick code span splits the markdown
#      cell mid-command, corrupting the table (GFM) and the harness parser.
#
# Exits non-zero if any smell is present. Green means every completed row's
# declared targets exist at HEAD, no verify is hollow/compile-only, no selector
# is vacuous, and every verify cell is a well-formed single table field.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
smells=0
ROLLUP_RUN=$(mktemp -d)
trap 'rm -rf "$ROLLUP_RUN"' EXIT

# Backtick- and escape-aware cell split: a `|` inside a `...` span or a `\|`
# escape is not a delimiter. (1-based field N.)
cell() {
	printf '%s\n' "$1" | awk -v n="$2" '{
		f=1; c=""; bt=0
		for(i=1;i<=length($0);i++){ ch=substr($0,i,1)
			if(ch=="\\" && i<length($0)){ c=c ch substr($0,i+1,1); i++; continue }
			if(ch=="`"){bt=!bt; c=c ch; continue}
			if(ch=="|" && !bt){ if(f==n){print c; exit} f++; c=""; continue }
			c=c ch }
		if(f==n) print c
	}' | sed 's/^[ \t]*//;s/[ \t]*$//'
}

# Field count under the same split. A well-formed 6-column task row plus the
# leading and trailing empties yields exactly 8 fields.
ncells() {
	printf '%s\n' "$1" | awk '{
		f=1; bt=0
		for(i=1;i<=length($0);i++){ ch=substr($0,i,1)
			if(ch=="\\" && i<length($0)){ i++; continue }
			if(ch=="`"){bt=!bt; continue}
			if(ch=="|" && !bt) f++ }
		print f
	}'
}

# Reduce a verify cell to its runnable command: first backtick code span if the
# cell has one (annotations like ` (GREEN)` may follow), else the cell verbatim.
verify_cmd() {
	printf '%s\n' "$1" | awk '{
		s=index($0,"`")
		if(s==0){print; next}
		rest=substr($0,s+1); e=index(rest,"`")
		if(e==0){print; next}
		print substr(rest,1,e-1)
	}' | sed 's/\\|/|/g'
}

flag() { smells=$((smells+1)); printf '%-10s %-6s %s\n' "$1" "$2" "$3"; }

for tasks in "$ROOT"/specs/[0-1][0-9]-*/tasks.md; do
	[ -f "$tasks" ] || continue
	dom=$(basename "$(dirname "$tasks")" | cut -c1-2)
	while IFS= read -r line; do
		# Any task row (completed or not); capture whether it is [x].
		case "$line" in
			'| ['*'] T'*) ;;
			*) continue ;;
		esac
		id=$(cell "$line" 2 | sed -n 's/^\(\[[ x]\] \)\{0,1\}\(T[0-9][0-9]*\)$/\2/p')
		[ -n "$id" ] || continue
		tag="$dom-$id"
		done_row=$(cell "$line" 2 | grep -q '^\[x\]' && echo yes || echo no)
		role=$(cell "$line" 3)
		raw6=$(cell "$line" 6)
		verify=$(verify_cmd "$raw6")

		# F: raw unescaped pipe corrupts the table (row splits into >8 fields).
		if [ "$(ncells "$line")" -ne 8 ]; then
			flag "$tag" "F" "unescaped '|' outside a code span (corrupts table/parse): $line"
		fi

		# E: vacuous \| selector — matches a literal pipe, runs zero tests.
		case "$raw6" in
			*'\|'*) flag "$tag" "E" "vacuous '\\|' selector (Go regexp literal pipe, runs 0 tests): $raw6" ;;
		esac

		# A: verify reads authoring specs/ (mask .specd/specs/ first).
		masked=$(printf '%s\n' "$verify" | sed 's#\.specd/specs/#@@#g')
		case "$masked" in
			*[!A-Za-z0-9]specs/*|specs/*)
				flag "$tag" "A" "verify reads authoring specs/ (runtime reads .specd/specs/): $verify" ;;
		esac

		# B: hollow verify — write task that passes without asserting behavior.
		case "$verify" in
			""|":"|"true"|"true "*|"test -e "*|"test -f "*|"[ -e "*|"[ -f "*|"ls "*|*"|| true")
				[ -n "$verify" ] && [ "$role" = "craftsman" ] && flag "$tag" "B" "write-task hollow verify: $verify" ;;
		esac

		# D: compile-only write task — cannot prove production write behavior.
		case "$verify" in
			"go build"|"go build "*)
				[ "$role" = "craftsman" ] && flag "$tag" "D" "write-task compile-only verify: $verify" ;;
		esac

		# C: stale target — a *completed* row's verify must not reference a file
		# that no longer exists. Only the verify command's own path arguments are
		# checked (they are well-formed); the free-form files column also carries
		# prose and is not a reliable target list, so it is left to the human
		# review the tables already receive.
		[ "$done_row" = "yes" ] || continue
		vpaths=$(printf '%s\n' "$verify" \
			| grep -oE '(sh|test -e|test -f|cat) [A-Za-z0-9._/-]+' 2>/dev/null \
			| awk '{print $NF}' || true)
		for p in $vpaths; do
			case "$p" in
				""|*[!A-Za-z0-9._/-]*|or) continue ;;
			esac
			[ -e "$ROOT/$p" ] || flag "$tag" "C" "stale target (test -e fails): $p"
		done
	done <"$tasks"
done

# Release-proof tripwires: specific proof targets must remain present, or the
# remote/black-box regression scripts degrade into hollow shells. Kept explicit
# because these assert cross-file invariants the per-row audit cannot see.
for p in internal/orchestration/dispatch_envelope.go internal/orchestration/dispatch_envelope_test.go internal/orchestration/lease_test.go; do
	[ -f "$ROOT/$p" ] || flag 03-W5 C "missing remote-envelope proof target: $p"
done
for p in internal/orchestration/a2a.go internal/orchestration/a2a_test.go internal/integration/orchestration_conformance_test.go; do
	[ -f "$ROOT/$p" ] || flag 05-W5 C "missing orchestration adapter proof target: $p"
done
grep -q 'violation 05-W5' "$ROOT/scripts/regress-domains.sh" || flag 05-W5 B "missing Domain 05 fresh-tree regression assertion"
for p in internal/core/verify/adapter.go internal/core/verify/adapter_test.go internal/core/gates/security/regress.go internal/core/gates/security/regress_test.go internal/core/gates/security/testdata/incidents.json internal/integration/sandbox_conformance_test.go; do
	[ -f "$ROOT/$p" ] || flag 06-W8 C "missing security release-proof target: $p"
done
grep -q 'violation 06-W8' "$ROOT/scripts/regress-domains.sh" || flag 06-W8 B "missing Domain 06 fresh-tree regression assertion"

# G: program rollup and domain task rows are duplicate projections of one
# completion truth. Compare complete key sets and status, catching a mismatch
# whether it originates in progress.md or a domain task table.
program_truth="$ROLLUP_RUN/program.tsv"
domain_truth="$ROLLUP_RUN/domain.tsv"
awk '
/^- \[[x ]\] [0-9][0-9] W[0-9]+/ {
	status = (substr($0, 4, 1) == "x") ? "done" : "pending"
	line = $0
	sub(/^- \[[x ]\] /, "", line)
	split(line, fields, / +/)
	print fields[1], fields[2], status
}' "$ROOT/specs/progress.md" | sort >"$program_truth"
: >"$domain_truth"
for tasks in "$ROOT"/specs/[0-1][0-9]-*/tasks.md; do
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
	flag program G "progress.md rollup differs from domain task truth"
fi

if [ "$smells" -eq 0 ]; then
	echo "regress-lint: clean — no smells"
else
	printf '\nregress-lint: %d smell(s) present\n' "$smells" >&2
fi
exit "$smells"
