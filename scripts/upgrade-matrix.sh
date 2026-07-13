#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
MODE=${1:-matrix}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

fail() { printf 'upgrade-matrix: %s\n' "$*" >&2; exit 1; }
sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }

make_release() {
  release=$1
  label=$2
  mkdir -p "$release"
  cat >"$tmp/specd" <<EOF
#!/usr/bin/env sh
if [ "\${1:-}" = version ]; then printf '{"version":"$label","commit":"$label"}\\n'; exit 0; fi
if [ "\${1:-}" = handshake ]; then printf '{"version":"1"}\\n'; exit 0; fi
exit 0
EOF
  chmod 755 "$tmp/specd"
  tar -czf "$release/specd_${label}_linux_amd64.tar.gz" -C "$tmp" specd
  printf '%s  %s\n' "$(sha256 "$release/specd_${label}_linux_amd64.tar.gz")" "specd_${label}_linux_amd64.tar.gz" >"$release/checksums.txt"
}

workspace=$tmp/workspace
mkdir -p "$workspace/.specd/specs/upgrade-fixture"
cp "$ROOT/testdata/upgrade/v1/state.json" "$workspace/.specd/specs/upgrade-fixture/state.json"
cp "$workspace/.specd/specs/upgrade-fixture/state.json" "$tmp/state.before"

old=$tmp/old
new=$tmp/new
dest=$tmp/bin
make_release "$old" 1.0.0
make_release "$new" 1.1.0
SPECD_RELEASE_DIR=$old SPECD_OS=linux SPECD_ARCH=amd64 sh "$ROOT/scripts/install.sh" --version 1.0.0 --install-dir "$dest" >/dev/null
SPECD_RELEASE_DIR=$new SPECD_OS=linux SPECD_ARCH=amd64 sh "$ROOT/scripts/install.sh" --version 1.1.0 --install-dir "$dest" --update >/dev/null
cmp -s "$tmp/state.before" "$workspace/.specd/specs/upgrade-fixture/state.json" || fail "N-1 to N changed state/evidence bytes"

# Current schema preflight rejects future state before writes.
mkdir -p "$tmp/future/.specd/specs/upgrade-fixture"
cp "$ROOT/testdata/upgrade/future/state.json" "$tmp/future/.specd/specs/upgrade-fixture/state.json"
before=$(sha256 "$tmp/future/.specd/specs/upgrade-fixture/state.json")
if (cd "$tmp/future" && "$ROOT/specd" status upgrade-fixture >/dev/null 2>&1); then
  fail "future schema accepted"
fi
[ "$before" = "$(sha256 "$tmp/future/.specd/specs/upgrade-fixture/state.json")" ] || fail "future schema rejection mutated state"

if [ "$MODE" = "--crash-drill" ]; then
  for point in staged backed-up swapped smoke-passed; do
    rm -rf "$dest"
    SPECD_RELEASE_DIR=$old SPECD_OS=linux SPECD_ARCH=amd64 sh "$ROOT/scripts/install.sh" --version 1.0.0 --install-dir "$dest" >/dev/null
    SPECD_RELEASE_DIR=$new SPECD_OS=linux SPECD_ARCH=amd64 SPECD_TEST_FAIL_AT=$point \
      sh "$ROOT/scripts/install.sh" --version 1.1.0 --install-dir "$dest" --update >/dev/null 2>&1 || true
    [ -x "$dest/specd" ] || fail "$point left no complete binary"
    "$dest/specd" version --json | grep -Eq '"version":"1\.(0|1)\.0"' || fail "$point left corrupt binary"
  done
fi

printf 'upgrade-matrix: ok\n'
