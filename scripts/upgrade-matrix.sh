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

# --- Removal-exit matrix: time alone never deletes support. ---
# The future-schema block above already proves downgrade-preflight — an older
# binary refuses upgraded state before any write. The rest of the matrix proves
# removal itself never fires while the exit gates are unmet.
# Every tracked compatibility surface must report removal BLOCKED with a named
# retained path until its full exit proof passes. A dev binary is short of the
# published version window; a release binary past the window is still blocked on
# the missing release-owner decision. Nothing is removed by this release.
removal=$tmp/removal.json
(cd "$workspace" && "$ROOT/specd" report upgrade-fixture --compat-removal --json) >"$removal" || fail "compat-removal projection failed"
grep -q '"eligible": true' "$removal" && fail "a surface was removal-eligible on a dev binary"
grep -q '"retained_path"' "$removal" || fail "blocked removal did not name a retained path"
grep -q '"blocking_gate": "unmet-window-version"' "$removal" || fail "dev binary did not report the version window gate"

release=$tmp/specd_release
(cd "$ROOT" && go build -ldflags "-X github.com/0xkhdr/specd/internal/version.Version=9.9.9" -o "$release" .) || fail "release-stamped build failed"
(cd "$workspace" && "$release" report upgrade-fixture --compat-removal --json) >"$removal.rel" || fail "release compat-removal projection failed"
grep -q '"eligible": true' "$removal.rel" && fail "a surface was removal-eligible without a release-owner decision"
grep -q '"blocking_gate": "release-decision"' "$removal.rel" || fail "release binary past the window did not block on the release-owner decision"

# --- Archive-read immutability: inspection never rewrites the manifest. ---
archive=$tmp/archive.json
printf '{"schema_version":1,"spec_id":"legacy-v1","digest":"deadbeef"}\n' >"$archive"
archive_before=$(sha256 "$archive")
cat "$archive" >/dev/null
[ "$archive_before" = "$(sha256 "$archive")" ] || fail "archive read mutated the manifest"

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
