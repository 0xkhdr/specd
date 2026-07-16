#!/usr/bin/env sh
# Installed-binary lifecycle smoke. All harness state transitions use advertised
# CLI commands; fixture authoring models normal user edits between approvals.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RUN=$(mktemp -d)
trap 'rm -rf "$RUN"' EXIT HUP INT TERM

if [ -n "${SPECD_BIN:-}" ]; then
	BIN=$SPECD_BIN
else
	BIN=$RUN/specd
	(cd "$ROOT" && go build -o "$BIN" .)
fi
BIN=$(CDPATH= cd -- "$(dirname -- "$BIN")" && pwd)/$(basename -- "$BIN")

cd "$RUN"
git init -q
git config user.email smoke@specd.invalid
git config user.name specd-smoke
git commit -q --allow-empty -m root

"$BIN" init >/dev/null
"$BIN" new smoke >/dev/null

cat >.specd/specs/smoke/requirements.md <<'EOF'
# Requirements — smoke

- **R1** When a user runs the lifecycle, the system shall preserve evidence.
EOF
cat >.specd/specs/smoke/design.md <<'EOF'
# Design — smoke

## Modules
The lifecycle commands coordinate deterministic core modules.

## On-disk contracts
State and evidence remain under .specd/specs/smoke.

## Invariants
Every completed task has passing evidence pinned to a real git HEAD.
EOF
cat >.specd/specs/smoke/tasks.md <<'EOF'
# Tasks — smoke

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | requirements.md | - | printf ok | R1 |
EOF
git add .
git commit -q -m "author smoke spec"

# Deliberately invalid completion must fail closed and name missing verify evidence.
negative=$RUN/negative.out
if "$BIN" complete-task smoke T1 >"$negative" 2>&1; then
	echo "production-smoke: completion without verify unexpectedly succeeded" >&2
	exit 1
fi
if ! grep -Eq 'verify|passing evidence' "$negative"; then
	echo "production-smoke: invalid step omitted documented next action" >&2
	cat "$negative" >&2
	exit 1
fi
if [ "${1:-}" = "--negative" ]; then
	echo "production-smoke: invalid step failed closed with verify next action"
	exit 0
fi

"$BIN" check smoke >/dev/null
"$BIN" approve smoke >/dev/null
"$BIN" approve smoke >/dev/null
"$BIN" approve smoke >/dev/null
"$BIN" context smoke T1 >/dev/null
"$BIN" verify smoke T1 >/dev/null
"$BIN" complete-task smoke T1 >/dev/null
"$BIN" review smoke >/dev/null
"$BIN" submit smoke >/dev/null

test -s .specd/specs/smoke/evidence.jsonl
test -s .specd/specs/smoke/review_report.md
echo "production-smoke: installed lifecycle passed"
