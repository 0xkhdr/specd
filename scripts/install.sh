#!/usr/bin/env sh
set -eu

REPO="0xkhdr/specd"
DEFAULT_INSTALL_DIR="/usr/local/bin"

usage() {
  cat <<'USAGE'
Install specd from a GitHub release archive.

Usage:
  install.sh [--version <tag>] [--install-dir <dir>] [--update] [--force] [--dry-run]

Options:
  --version <tag>       Release tag or version. Defaults to SPECD_VERSION or latest.
  --install-dir <dir>   Directory for the specd binary. Defaults to SPECD_INSTALL_DIR or /usr/local/bin.
  --update              Replace an existing specd binary.
  --force               Replace an existing specd binary.
  --dry-run             Print planned actions without writing.
  --help                Show this help.
USAGE
}

die() {
  printf 'install.sh: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

detect_os() {
  case "${SPECD_OS:-$(uname -s)}" in
    Linux | linux) printf 'linux' ;;
    Darwin | darwin) printf 'darwin' ;;
    *) die "unsupported OS: ${SPECD_OS:-$(uname -s)}" ;;
  esac
}

detect_arch() {
  case "${SPECD_ARCH:-$(uname -m)}" in
    x86_64 | amd64) printf 'amd64' ;;
    arm64 | aarch64) printf 'arm64' ;;
    *) die "unsupported architecture: ${SPECD_ARCH:-$(uname -m)}" ;;
  esac
}

run_write() {
  if [ "$DRY_RUN" = "1" ]; then
    printf '+ %s\n' "$*"
    return 0
  fi
  "$@"
}

run_privileged() {
  if [ -w "$INSTALL_DIR" ]; then
    run_write "$@"
    return 0
  fi
  command -v sudo >/dev/null 2>&1 || die "install dir is not writable and sudo is unavailable: $INSTALL_DIR"
  if [ "$DRY_RUN" = "1" ]; then
    printf '+ sudo %s\n' "$*"
    return 0
  fi
  sudo "$@"
}

ensure_install_dir() {
  if [ "$DRY_RUN" = "1" ]; then
    printf '+ mkdir -p %s\n' "$INSTALL_DIR"
    return 0
  fi
  if [ -d "$INSTALL_DIR" ]; then
    return 0
  fi
  parent=$(dirname "$INSTALL_DIR")
  if [ -w "$parent" ]; then
    mkdir -p "$INSTALL_DIR"
    return 0
  fi
  command -v sudo >/dev/null 2>&1 || die "install dir parent is not writable and sudo is unavailable: $parent"
  sudo mkdir -p "$INSTALL_DIR"
}

download() {
  url=$1
  out=$2
  if [ -n "${SPECD_RELEASE_DIR:-}" ]; then
    src=$SPECD_RELEASE_DIR/$(basename "$url")
    [ -f "$src" ] || die "fixture not found: $src"
    cp "$src" "$out"
    return 0
  fi
  need_cmd curl
  curl -fsSL "$url" -o "$out"
}

latest_tag() {
  if [ -n "${SPECD_VERSION:-}" ]; then
    printf '%s' "$SPECD_VERSION"
    return 0
  fi
  need_cmd curl
  effective=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
  tag=${effective##*/}
  if [ -z "$tag" ] || [ "$tag" = "latest" ]; then
    die "could not resolve latest release"
  fi
  printf '%s' "$tag"
}

sha256_file() {
  file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return 0
  fi
  die "missing required command: sha256sum or shasum"
}

check_checksum() {
  sums=$1
  archive=$2
  name=$(basename "$archive")
  expected=$(awk -v n="$name" '{f=$2; sub(/^\*/, "", f); if (f == n) {print $1; exit}}' "$sums")
  [ -n "$expected" ] || die "checksum not found for $name"
  actual=$(sha256_file "$archive")
  [ "$actual" = "$expected" ] || die "checksum mismatch for $name"
}

binary_identity() {
  candidate=$1
  identity=$($candidate version --json 2>/dev/null) || return 1
  printf '%s' "$identity" | grep -q '"version"' || return 1
  if [ -n "${SPECD_EXPECT_COMMIT:-}" ]; then
    printf '%s' "$identity" | grep -q '"commit":"'"$SPECD_EXPECT_COMMIT"'"' || \
      return 1
  fi
}

smoke_binary() {
  candidate=$1
  binary_identity "$candidate" || return 1
  if [ -n "${SPECD_SMOKE_ROOT:-}" ]; then
    [ -d "$SPECD_SMOKE_ROOT" ] || return 1
    (cd "$SPECD_SMOKE_ROOT" && "$candidate" handshake bootstrap --json >/dev/null) || \
      return 1
  fi
}

managed_digest() {
  candidate=$1
  [ -n "${SPECD_SMOKE_ROOT:-}" ] || return 1
  output=$(cd "$SPECD_SMOKE_ROOT" && "$candidate" handshake bootstrap --json 2>/dev/null) || return 1
  printf '%s' "$output" | awk -F'"managed_digest":"' 'NF > 1 {split($2, a, "\""); print a[1]; exit}'
}

VERSION=${SPECD_VERSION:-}
INSTALL_DIR=${SPECD_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}
FORCE=0
DRY_RUN=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || die "--version requires a value"
      VERSION=$2
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || die "--install-dir requires a value"
      INSTALL_DIR=$2
      shift 2
      ;;
    --update | --force)
      FORCE=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --help | -h)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[ -n "$INSTALL_DIR" ] || die "install dir cannot be empty"
OS=$(detect_os)
ARCH=$(detect_arch)
if [ -z "$VERSION" ]; then
  VERSION=$(latest_tag)
fi
TAG=$VERSION
case "$TAG" in
  v*) ARCHIVE_VERSION=${TAG#v} ;;
  *) ARCHIVE_VERSION=$TAG; TAG=v$TAG ;;
esac

ARCHIVE="specd_${ARCHIVE_VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$TAG"
BIN="$INSTALL_DIR/specd"

if [ -e "$BIN" ] && [ "$FORCE" != "1" ] && [ "$DRY_RUN" != "1" ]; then
  die "specd already exists at $BIN; pass --update or --force"
fi

printf 'Installing specd %s for %s/%s to %s\n' "$TAG" "$OS" "$ARCH" "$BIN"

if [ "$DRY_RUN" = "1" ]; then
  printf '+ download %s/%s\n' "$BASE_URL" "$ARCHIVE"
  printf '+ download %s/checksums.txt\n' "$BASE_URL"
  printf '+ verify checksum\n'
  printf '+ verify version/commit identity\n'
  printf '+ preview managed-asset changes\n'
  ensure_install_dir
  printf '+ stage binary beside target and atomically rename\n'
  printf '+ retain previous binary until post-install smoke passes\n'
  exit 0
fi

need_cmd tar
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

download "$BASE_URL/$ARCHIVE" "$tmp/$ARCHIVE"
download "$BASE_URL/checksums.txt" "$tmp/checksums.txt"
check_checksum "$tmp/checksums.txt" "$tmp/$ARCHIVE"
tar -xzf "$tmp/$ARCHIVE" -C "$tmp" specd

smoke_binary "$tmp/specd" || die "staged binary failed identity/handshake smoke"

ensure_install_dir
STAGED="$INSTALL_DIR/.specd-stage-$$"
PREVIOUS="$INSTALL_DIR/specd.previous"
run_privileged install -m 0755 "$tmp/specd" "$STAGED"

old_managed=
new_managed=
if [ -e "$BIN" ]; then old_managed=$(managed_digest "$BIN" || true); fi
new_managed=$(managed_digest "$STAGED" || true)
if [ -n "$old_managed" ] && [ -n "$new_managed" ]; then
  if [ "$old_managed" = "$new_managed" ]; then
    printf 'Managed-asset changes: none (digest %s)\n' "$new_managed"
  else
    printf 'Managed-asset changes: digest %s -> %s\n' "$old_managed" "$new_managed"
  fi
else
  printf 'Managed-asset changes: preview unavailable (set SPECD_SMOKE_ROOT to a managed workspace)\n'
fi
had_previous=0
if [ -e "$BIN" ]; then
  had_previous=1
  run_privileged rm -f "$PREVIOUS"
  run_privileged mv "$BIN" "$PREVIOUS"
fi
if ! run_privileged mv "$STAGED" "$BIN"; then
  [ "$had_previous" = 0 ] || run_privileged mv "$PREVIOUS" "$BIN"
  die "atomic binary swap failed; previous binary restored"
fi
if ! smoke_binary "$BIN"; then
  run_privileged rm -f "$BIN"
  [ "$had_previous" = 0 ] || run_privileged mv "$PREVIOUS" "$BIN"
  die "post-install smoke failed; previous binary restored"
fi
run_privileged rm -f "$PREVIOUS"
printf 'Installed %s\n' "$BIN"
