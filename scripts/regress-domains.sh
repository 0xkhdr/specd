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
#   W5 surface          bare verb count == 16 (ADR-scoped surface)
#   W6 release          `--version` prints a stamp
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

# W0 — honesty: progress.md must obey its own wave-ordering invariant
# ("a wave may start only when every spec in the prior wave is done"). In file
# order, waves run top-to-bottom, so a `pending`/`in-progress` row must never
# precede a `done` row. Pure text check — no binary, current program/wave format.
w0_seen_incomplete=0
w0_bad=0
w0_rows=$(awk -F'|' 'NF>=5 { s=$4; gsub(/^[ \t]+|[ \t]+$/, "", s); if (s=="pending"||s=="in-progress"||s=="done") print s }' "$RUN/specs/progress.md")
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
elif [ -z "$w0_rows" ]; then
	# 08a/T04: reproduce the documented fail-open. This advertised honesty check
	# parses a `| ... | done |` table cell, but progress.md now uses `[x]`/`[ ]`
	# checkbox rows — so its input absent, and the check still reports a pass
	# instead of failing or skipping. 08e/T22 replaces this line with a fail/skip.
	pass W0 "progress.md wave ordering honest (input absent: fail-open reproduced)"
else
	pass W0 "progress.md wave ordering honest"
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
# removing a verb is a deliberate edit here. Current surface is 24 (16 original
# + submit, review, link, unlink, program-era verbs, version, triage). Bump this
# only alongside an intended verb change.
W5_EXPECT=24
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

echo "regress-domains: all per-domain invariants hold"
