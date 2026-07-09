#!/usr/bin/env sh
set -eu

DEFAULT_INSTALL_DIR="/usr/local/bin"

usage() {
  cat <<'USAGE'
Uninstall specd.

Usage:
  uninstall.sh [--install-dir <dir>] [--dry-run]

Options:
  --install-dir <dir>   Directory containing specd. Defaults to SPECD_INSTALL_DIR or /usr/local/bin.
  --dry-run             Print planned removal without writing.
  --help                Show this help.
USAGE
}

die() {
  printf 'uninstall.sh: %s\n' "$*" >&2
  exit 1
}

run_remove() {
  if [ "$DRY_RUN" = "1" ]; then
    printf '+ %s\n' "$*"
    return 0
  fi
  "$@"
}

run_privileged() {
  if [ -w "$INSTALL_DIR" ]; then
    run_remove "$@"
    return 0
  fi
  command -v sudo >/dev/null 2>&1 || die "install dir is not writable and sudo is unavailable: $INSTALL_DIR"
  if [ "$DRY_RUN" = "1" ]; then
    printf '+ sudo %s\n' "$*"
    return 0
  fi
  sudo "$@"
}

INSTALL_DIR=${SPECD_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}
DRY_RUN=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --install-dir)
      [ "$#" -ge 2 ] || die "--install-dir requires a value"
      INSTALL_DIR=$2
      shift 2
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

case "$INSTALL_DIR" in
  "" | "/" | "." | "..")
    die "refusing dangerous install dir: $INSTALL_DIR"
    ;;
esac

BIN=$INSTALL_DIR/specd
if [ ! -e "$BIN" ]; then
  printf 'specd not installed at %s\n' "$BIN"
  exit 0
fi

printf 'Removing %s\n' "$BIN"
run_privileged rm -f "$BIN"
printf 'Removed %s\n' "$BIN"
