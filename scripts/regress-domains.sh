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

# W0 — honesty: progress.md green claims must survive the audit.
if sh "$RUN/scripts/audit-progress.sh" >/dev/null 2>&1; then
	pass W0 "progress.md green rows honest"
else
	violation W0 "audit-progress.sh reports a falsified green row"
fi

# W1 — ADR-7 mode enum: an unknown mode must be refused.
if "$SPECD" new __rp_w1 --mode __bogus__ >/dev/null 2>&1; then
	violation W1 "unknown --mode accepted (mode enum not enforced)"
else
	pass W1 "unknown --mode rejected"
fi

# W2 — trust boundary: brain must be fail-closed on default config.
if "$SPECD" brain start __rp_w2 >/dev/null 2>&1; then
	violation W2 "brain start succeeded on default config (not fail-closed)"
else
	pass W2 "brain start fail-closed"
fi

# W3 — records: decision without --text is a usage error.
if "$SPECD" decision __rp_w3 >/dev/null 2>&1; then
	violation W3 "decision without --text accepted (hollow record)"
else
	pass W3 "decision requires --text"
fi

# W4 — gates: check on a fresh scaffold must reject placeholder EARS.
"$SPECD" new __rp_w4 >/dev/null 2>&1 || violation W4 "could not scaffold probe spec"
if "$SPECD" check __rp_w4 >/dev/null 2>&1; then
	violation W4 "check passed on placeholder scaffold (EARS gate inert)"
else
	pass W4 "check rejects placeholder EARS"
fi

# W5 — surface: bare verb count must equal the ADR-scoped 16.
verbs=$("$SPECD" 2>&1 | sed -n 's/^  \([a-z][a-z]*\) .*/\1/p' | sort -u | wc -l | tr -d ' ')
if [ "$verbs" -ne 16 ]; then
	violation W5 "verb count is $verbs, expected 16"
else
	pass W5 "verb count == 16"
fi

# W6 — release: --version prints a non-empty stamp.
if "$SPECD" --version 2>/dev/null | grep -qE '.'; then
	pass W6 "--version prints a stamp"
else
	violation W6 "--version prints nothing"
fi

echo "regress-domains: all per-domain invariants hold"
