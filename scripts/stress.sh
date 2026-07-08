#!/usr/bin/env sh
# stress.sh (SPEC-01 T-01-04) — general cross-process contention on one spec.
#
# Races N concurrent `specd decision` writers at a single spec's state.json.
# The state layer serialises every mutation through the reentrant per-spec lock
# and a compare-and-swap on the revision counter, writing atomically. Under a
# race a writer either lands cleanly (bumps revision, appends its record) or
# fails without clobbering — never a torn write, never a lost update.
#
# Invariant: on a fresh spec the revision starts at 0 and every recorded
# decision bumps it by exactly one, so the count of landed decision records
# must equal the final revision. A lost update or double-count breaks the
# equality. Each writer stamps a unique "stressmark-<i>" so records are counted
# without a JSON parser. Runs in a throwaway tree; exits non-zero on violation.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=$(mktemp -d)/specd
go build -o "$bin" "$root"

tree=$(mktemp -d)
trap 'rm -rf "$tree"' EXIT
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
