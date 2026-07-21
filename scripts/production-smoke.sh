#!/usr/bin/env sh
# Installed-binary production preflight. This CLI-only lane declares no host
# sandbox, so policy must refuse before init or task work and name the recovery.
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
printf 'profile: production\n' >project.yml
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"driver_capabilities":{}}}' |
	"$BIN" mcp >preflight.json

grep -Fq '"capability":"sandbox","status":"refused"' preflight.json
grep -Fq '"recovery_action":"declare sandbox support or use read-only operations"' preflight.json
test "$(cat project.yml)" = 'profile: production'
test ! -e .specd

echo "production-smoke: refused before work; recovery: declare sandbox support or use read-only operations"
