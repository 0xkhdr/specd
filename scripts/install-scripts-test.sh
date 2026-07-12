#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

fail() {
  printf 'install-scripts-test: %s\n' "$*" >&2
  exit 1
}

make_release() {
  rel=$1
  mkdir -p "$rel"
  bin=$tmp/specd
  cat >"$bin" <<'SCRIPT'
#!/usr/bin/env sh
if [ "${1:-}" = version ] && [ "${2:-}" = --json ]; then
  printf '{"version":"1.2.3","commit":"abc123"}\n'
  exit 0
fi
if [ "${1:-}" = handshake ] && [ "${2:-}" = bootstrap ]; then
  [ "${SPECD_TEST_SMOKE_FAIL:-0}" != 1 ] || exit 42
  printf '{"version":"1"}\n'
  exit 0
fi
printf "specd test %s\n" "$1"
SCRIPT
  chmod 755 "$bin"
  tar -czf "$rel/specd_1.2.3_linux_amd64.tar.gz" -C "$tmp" specd
  if command -v sha256sum >/dev/null 2>&1; then
    sum=$(sha256sum "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  else
    sum=$(shasum -a 256 "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  fi
  printf '%s  specd_1.2.3_linux_amd64.tar.gz\n' "$sum" >"$rel/checksums.txt"
}

make_bad_release() {
  rel=$1
  mkdir -p "$rel"
  cat >"$tmp/specd" <<'SCRIPT'
#!/usr/bin/env sh
if [ "${1:-}" = version ]; then printf '{"version":"1.2.3","commit":"bad"}\n'; exit 0; fi
exit 42
SCRIPT
  chmod 755 "$tmp/specd"
  tar -czf "$rel/specd_1.2.3_linux_amd64.tar.gz" -C "$tmp" specd
  if command -v sha256sum >/dev/null 2>&1; then
    sum=$(sha256sum "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  else
    sum=$(shasum -a 256 "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  fi
  printf '%s  specd_1.2.3_linux_amd64.tar.gz\n' "$sum" >"$rel/checksums.txt"
}

make_post_bad_release() {
  rel=$1
  mkdir -p "$rel"
  cat >"$tmp/specd" <<'SCRIPT'
#!/usr/bin/env sh
if [ "${1:-}" = version ]; then printf '{"version":"1.2.3","commit":"abc123"}\n'; exit 0; fi
if [ "${1:-}" = handshake ]; then
  case "$0" in */bin/specd) exit 42 ;; *) exit 0 ;; esac
fi
printf 'replacement\n'
SCRIPT
  chmod 755 "$tmp/specd"
  tar -czf "$rel/specd_1.2.3_linux_amd64.tar.gz" -C "$tmp" specd
  if command -v sha256sum >/dev/null 2>&1; then
    sum=$(sha256sum "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  else
    sum=$(shasum -a 256 "$rel/specd_1.2.3_linux_amd64.tar.gz" | awk '{print $1}')
  fi
  printf '%s  specd_1.2.3_linux_amd64.tar.gz\n' "$sum" >"$rel/checksums.txt"
}

release=$tmp/release
dest=$tmp/bin
make_release "$release"

SPECD_RELEASE_DIR=$release SPECD_OS=linux SPECD_ARCH=amd64 \
  sh "$ROOT/scripts/install.sh" --version v1.2.3 --install-dir "$dest"

[ -x "$dest/specd" ] || fail "installed binary missing"
"$dest/specd" ok | grep 'specd test ok' >/dev/null || fail "installed binary failed"

if SPECD_RELEASE_DIR=$release SPECD_OS=linux SPECD_ARCH=amd64 \
  sh "$ROOT/scripts/install.sh" --version v1.2.3 --install-dir "$dest" >/dev/null 2>&1; then
  fail "second install without --force succeeded"
fi

SPECD_RELEASE_DIR=$release SPECD_OS=linux SPECD_ARCH=amd64 \
  SPECD_EXPECT_COMMIT=abc123 sh "$ROOT/scripts/install.sh" --version 1.2.3 --install-dir "$dest" --update

[ ! -e "$dest/specd.previous" ] || fail "previous binary retained after successful smoke"
[ ! -e "$dest/.specd-stage-1.2.3" ] || fail "staged binary leaked"

bad=$tmp/bad-release
make_bad_release "$bad"
before=$($dest/specd ok)
if SPECD_RELEASE_DIR=$bad SPECD_OS=linux SPECD_ARCH=amd64 \
  SPECD_EXPECT_COMMIT=abc123 sh "$ROOT/scripts/install.sh" --version 1.2.3 --install-dir "$dest" --update >/dev/null 2>&1; then
  fail "failed identity smoke update succeeded"
fi
[ "$($dest/specd ok)" = "$before" ] || fail "failed update did not preserve previous binary"

post_bad=$tmp/post-bad-release
make_post_bad_release "$post_bad"
if SPECD_RELEASE_DIR=$post_bad SPECD_OS=linux SPECD_ARCH=amd64 \
  SPECD_EXPECT_COMMIT=abc123 SPECD_SMOKE_ROOT=$tmp sh "$ROOT/scripts/install.sh" \
  --version 1.2.3 --install-dir "$dest" --update >/dev/null 2>&1; then
  fail "failed post-install smoke update succeeded"
fi
[ "$($dest/specd ok)" = "$before" ] || fail "post-install failure did not restore previous binary"

sh "$ROOT/scripts/install.sh" --version v1.2.3 --install-dir "$dest" --dry-run |
  grep 'verify checksum' >/dev/null || fail "dry-run missing checksum plan"
sh "$ROOT/scripts/install.sh" --version v1.2.3 --install-dir "$dest" --dry-run |
  grep 'preview managed-asset changes' >/dev/null || fail "dry-run missing managed preview"

sh "$ROOT/scripts/uninstall.sh" --install-dir "$dest" --dry-run |
  grep "$dest/specd" >/dev/null || fail "uninstall dry-run missing target"

sh "$ROOT/scripts/uninstall.sh" --install-dir "$dest"
[ ! -e "$dest/specd" ] || fail "binary still present after uninstall"

sh "$ROOT/scripts/uninstall.sh" --install-dir "$dest" >/dev/null
if sh "$ROOT/scripts/uninstall.sh" --install-dir / --dry-run >/dev/null 2>&1; then
  fail "uninstaller accepted /"
fi

if SPECD_RELEASE_DIR=$release SPECD_OS=freebsd SPECD_ARCH=amd64 \
  sh "$ROOT/scripts/install.sh" --version v1.2.3 --install-dir "$dest" >/dev/null 2>&1; then
  fail "unsupported OS accepted"
fi

printf 'install-scripts-test: ok\n'
