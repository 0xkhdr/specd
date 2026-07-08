#!/usr/bin/env sh
# regress-lint.sh (P7.2) — static smell audit of the W0–W6 verify tables.
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

for tasks in "$ROOT"/review-specs/0[0-6]-*/tasks.md; do
	[ -f "$tasks" ] || continue
	while IFS= read -r line; do
		id=$(cell "$line" 2 | sed -n 's/^\(P[0-9][0-9]*\.[0-9][0-9]*[a-z]*\)$/\1/p')
		[ -n "$id" ] || continue
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
				[ -n "$verify" ] && flag "$id" "B" "hollow-verify (asserts existence only / cannot fail): $verify" ;;
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

if [ "$smells" -eq 0 ]; then
	echo "regress-lint: clean — no smells"
else
	printf '\nregress-lint: %d smell(s) present\n' "$smells" >&2
fi
exit "$smells"
