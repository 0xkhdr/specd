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

cat >project.yml <<'EOF'
profile: production
EOF

cat >.specd/specs/smoke/requirements.md <<'EOF'
# Requirements — smoke

## R1 — Evidence continuity

- R1.1: When one task completes, the system shall retain current passing evidence.
EOF
cat >.specd/specs/smoke/design.md <<'EOF'
# Design — smoke

- references: R1, R1.1
- boundaries: local process boundary
- interfaces: CLI command surface
- invariants: evidence pins current HEAD
- failure: local command errors fail closed
- integration: none
- alternatives: direct state mutation rejected
- disposition: harness-owned transitions
- owner: release operator

## Modules
The lifecycle commands coordinate deterministic core modules.

## On-disk contracts
State and evidence remain under .specd/specs/smoke.

## Invariants
Every completed task has passing evidence pinned to a real git HEAD.
EOF
cat >.specd/specs/smoke/tasks.md <<'EOF'
# Tasks — smoke

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | context |
|---|---|---|---|---|---|---|---|---|---|
| T1 | scout | workflow-proof.txt | - | printf ok | R1.1 | R1.1 | validation | low | fresh production fixture |
EOF
printf 'production workflow proof\n' >workflow-proof.txt
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

check_out=$RUN/check.out
if ! "$BIN" check smoke >"$check_out" 2>&1; then
	cat "$check_out" >&2
	exit 1
fi
"$BIN" approve smoke >/dev/null
"$BIN" approve smoke >/dev/null
"$BIN" approve smoke >/dev/null
"$BIN" context smoke T1 >/dev/null
"$BIN" verify smoke T1 >/dev/null
"$BIN" complete-task smoke T1 >/dev/null

# No lifecycle phase is skipped: executing advances to verifying first, then
# production completion refuses until criterion and independent review exist.
"$BIN" approve smoke >/dev/null
status_out=$RUN/status.out
"$BIN" status smoke --guide --json >"$status_out"
grep -q '"status": "verifying"' "$status_out" || { cat "$status_out" >&2; exit 1; }
quality_negative=$RUN/quality-negative.out
if "$BIN" approve smoke >"$quality_negative" 2>&1; then
	echo "production-smoke: completion without criterion/review unexpectedly succeeded" >&2
	exit 1
fi
grep -Eq 'criterion|review' "$quality_negative"

"$BIN" verify smoke --criterion 1.1 --status pass --evidence 'fresh production smoke evidence' >/dev/null
"$BIN" review smoke >/dev/null
sed -i 's/<your identity — required>/independent-auditor/; s/<approve | reject | needs-changes>/approve/; s/<Required when the verdict is reject or needs-changes: what must change and why./Reviewed scope, evidence, error handling, and rollback./; s/For an approve verdict, note what you checked.>//' .specd/specs/smoke/review_report.md
"$BIN" approve smoke >/dev/null
"$BIN" status smoke --guide --json >"$status_out"
grep -q '"status": "complete"' "$status_out" || { cat "$status_out" >&2; exit 1; }
"$BIN" submit smoke >/dev/null

test -s .specd/specs/smoke/evidence.jsonl
test -s .specd/specs/smoke/review_report.md
echo "production-smoke: installed lifecycle passed"
