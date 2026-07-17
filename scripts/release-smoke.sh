#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
BIN=${1:-}
if [ -z "$BIN" ]; then
  BIN=$tmp/specd
  (cd "$ROOT" && go build -o "$BIN" .)
fi

case "$BIN" in
  /*) ;;
  *) BIN=$(CDPATH='' cd -- "$(dirname -- "$BIN")" && pwd)/$(basename -- "$BIN") ;;
esac

[ -x "$BIN" ] || {
  printf 'release-smoke: binary missing: %s\n' "$BIN" >&2
  exit 1
}

identity=$($BIN version --json)
printf '%s' "$identity" | grep -q '"version"' || {
  printf 'release-smoke: version identity missing\n' >&2
  exit 1
}
if [ -n "${SPECD_EXPECT_COMMIT:-}" ]; then
  # Normalize whitespace so both compact and indented JSON output match.
  printf '%s' "$identity" | tr -d ' \n\t' | grep -q '"commit":"'"$SPECD_EXPECT_COMMIT"'"' || {
    printf 'release-smoke: commit identity mismatch (want %s, got: %s)\n' "$SPECD_EXPECT_COMMIT" "$identity" >&2
    exit 1
  }
fi

workspace=$tmp/workspace
mkdir -p "$workspace"
git -C "$workspace" init -q
(cd "$workspace" && "$BIN" init >/dev/null && "$BIN" handshake bootstrap --json >/dev/null)
printf 'release-smoke: ok\n'
