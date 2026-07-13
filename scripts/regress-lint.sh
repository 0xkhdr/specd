#!/usr/bin/env sh
# regress-lint.sh (P7.2) — static smell audit of active Domain 02 verify tables.
#
# Never runs a verify. Reads the tables and flags three smells:
#   A  verify targets authoring `specs/` when runtime reads `.specd/specs/`
#   B  hollow-verify (G4): a verify that passes without asserting behavior
#      (pure `test -e`/`ls` existence, `|| true`, or `:`/`true`)
#   C  stale target (G3): a `files:` or verify path that fails `test -e`
#
# Exits non-zero if any smell is present. Green means every declared target
# exists at HEAD and no verify is hollow or points at the authoring tree.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
smells=0

# Backtick-aware cell split: `|` inside a `...` span is not a delimiter.
cell() {
	printf '%s\n' "$1" | awk -v n="$2" '{
		f=1; c=""; bt=0
		for(i=1;i<=length($0);i++){ ch=substr($0,i,1)
			if(ch=="`"){bt=!bt; c=c ch; continue}
			if(ch=="|" && !bt){ if(f==n){print c; exit} f++; c=""; continue }
			c=c ch }
		if(f==n) print c
	}' | sed 's/^[ \t]*//;s/[ \t]*$//'
}
flag() { smells=$((smells+1)); printf '%-8s %-6s %s\n' "$1" "$2" "$3"; }

# Future domain tables intentionally contain planned paths. Audit current Domain 02
# release work here; each later domain owns the same audit when its wave activates.
for tasks in "$ROOT"/specs/02-*/tasks.md; do
	[ -f "$tasks" ] || continue
	while IFS= read -r line; do
		id=$(cell "$line" 2 | sed -n 's/^\(\[[ x]\] \)\{0,1\}\(T[0-9][0-9]*\)$/\2/p')
		[ -n "$id" ] || continue
		role=$(cell "$line" 3)
		files=$(cell "$line" 4)
		verify=$(cell "$line" 6 | sed -n 's/^[^`]*`\([^`]*\)`.*/\1/p' | sed 's/\\|/|/g')

		# A: verify reads authoring specs/ (mask .specd/specs/ first).
		masked=$(printf '%s\n' "$verify" | sed 's#\.specd/specs/#@@#g')
		case "$masked" in
			*[!A-Za-z0-9]specs/*|specs/*)
				flag "$id" "A" "verify reads authoring specs/ (runtime reads .specd/specs/): $verify" ;;
		esac

		# B: hollow-verify — passes without asserting behavior.
		case "$verify" in
			""|":"|"true"|"true "*|"test -e "*|"test -f "*|"[ -e "*|"[ -f "*|"ls "*|*"|| true")
				[ -n "$verify" ] && [ "$role" = "craftsman" ] && flag "$id" "B" "write-task hollow verify: $verify" ;;
		esac

		# D: compile-only commands cannot prove production-risk write behavior.
		# Read-only roles remain explicitly exempt and may use a trivial command.
		case "$verify" in
			"go build"|"go build "*)
				[ "$role" = "craftsman" ] && flag "$id" "D" "write-task compile-only verify: $verify" ;;
		esac

		# C: stale target — clean single-path tokens that don't exist.
		#    files: column (comma list) + verify args after sh/test -e/cat.
		paths=$(printf '%s' "$files" | tr ',' '\n' | sed 's/`//g;s/(.*)//;s/^[ \t]*//;s/[ \t]*$//')
		vpaths=$(printf '%s\n' "$verify" \
			| grep -oE '(sh|test -e|test -f|cat) [A-Za-z0-9._/-]+' 2>/dev/null \
			| awk '{print $NF}' || true)
		for p in $paths $vpaths; do
			# only clean paths (no brace/paren/space/prose "or"); skip ambiguous.
			case "$p" in
				""|*[!A-Za-z0-9._/-]*|or) continue ;;
			esac
			[ -e "$ROOT/$p" ] || flag "$id" "C" "stale target (test -e fails): $p"
		done
	done <"$tasks"
done

# Domain 04 R5 verify-quality lint. Inspect its authoring contract without the
# stale-target audit above: later waves intentionally declare files not created
# yet. Only write tasks are subject to shallow-verify rejection; read-only
# scout/validator/auditor rows retain their explicit trivial exception.
tasks="$ROOT/specs/04-verification-evals-and-quality/tasks.md"
while IFS= read -r line; do
	id=$(cell "$line" 2 | sed -n 's/^\(\[[ x]\] \)\{0,1\}\(T[0-9][0-9]*\)$/\2/p')
	[ -n "$id" ] || continue
	role=$(cell "$line" 3)
	verify=$(cell "$line" 6 | sed 's/^[ 	]*//;s/[ 	]*$//')
	[ "$role" = "craftsman" ] || continue
	case "$verify" in
		""|":"|"true"|"true "*|"printf ok"|"go build"|"go build "*|*"|| true")
			flag "$id" "D" "Domain 04 write-task shallow verify: $verify" ;;
	esac
done <"$tasks"

# Domain 03 W5 release-proof tripwire: envelope contract and stale-claim tests
# must remain present; absence would turn the remote proof into a hollow script.
for p in internal/orchestration/dispatch_envelope.go internal/orchestration/dispatch_envelope_test.go internal/orchestration/lease_test.go; do
	[ -f "$ROOT/$p" ] || flag 03-W5 C "missing remote-envelope proof target: $p"
done

# Domain 05 W5 release-proof tripwire: adapter implementation, parity fixture,
# and fresh-tree regression hook must remain concrete.
for p in internal/orchestration/a2a.go internal/orchestration/a2a_test.go internal/integration/orchestration_conformance_test.go; do
	[ -f "$ROOT/$p" ] || flag 05-W5 C "missing orchestration adapter proof target: $p"
done
grep -q 'violation 05-W5' "$ROOT/scripts/regress-domains.sh" || flag 05-W5 B "missing Domain 05 fresh-tree regression assertion"

for p in internal/core/verify/adapter.go internal/core/verify/adapter_test.go internal/core/gates/security/regress.go internal/core/gates/security/regress_test.go internal/core/gates/security/testdata/incidents.json internal/integration/sandbox_conformance_test.go; do
	[ -f "$ROOT/$p" ] || flag 06-W8 C "missing security release-proof target: $p"
done
grep -q 'violation 06-W8' "$ROOT/scripts/regress-domains.sh" || flag 06-W8 B "missing Domain 06 fresh-tree regression assertion"

if [ "$smells" -eq 0 ]; then
	echo "regress-lint: clean — no smells"
else
	printf '\nregress-lint: %d smell(s) present\n' "$smells" >&2
fi
exit "$smells"
