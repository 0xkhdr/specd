#!/bin/sh
# install_test.sh — unit tests for the handoff/PATH logic in install.sh.
#
# Sources install.sh as a library (SPECD_INSTALL_LIB=1) and exercises its
# functions against fake binaries and a sandbox HOME. No network, no real
# install, and no mutation of the directory the tests run from.
#
# Run: sh scripts/install_test.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Load install.sh functions without triggering main(). NO_COLOR keeps assertions
# free of escape sequences. Source FIRST: install.sh defines log/ok/warn/die that
# finish() depends on; our test helpers below use distinct names to avoid clobber.
NO_COLOR=1
export NO_COLOR
SPECD_INSTALL_LIB=1
export SPECD_INSTALL_LIB
# shellcheck source=scripts/install.sh
. "${SCRIPT_DIR}/install.sh"
# install.sh runs under `set -e`; assertions manage status explicitly.
set +e

PASS=0
FAIL=0

tpass() { PASS=$((PASS + 1)); }
tfail() { FAIL=$((FAIL + 1)); printf 'FAIL: %s\n' "$*"; }

# assert_contains <haystack> <needle> <label>
assert_contains() {
  case "$1" in
    *"$2"*) tpass ;;
    *) tfail "$3: expected to contain '$2'
--- got ---
$1" ;;
  esac
}

# assert_missing <haystack> <needle> <label>
assert_missing() {
  case "$1" in
    *"$2"*) tfail "$3: expected NOT to contain '$2'" ;;
    *) tpass ;;
  esac
}

# assert_eq <got> <want> <label>
assert_eq() {
  if [ "$1" = "$2" ]; then tpass; else tfail "$3: got '$1' want '$2'"; fi
}

# make_fake_specd <dir> <version-line> — install a runnable fake at <dir>/specd.
make_fake_specd() {
  mkdir -p "$1"
  cat > "$1/specd" <<EOF
#!/bin/sh
[ "\$1" = "version" ] && echo "$2"
exit 0
EOF
  chmod +x "$1/specd"
}

# --- setup_path: PATH absent → rc files updated, and idempotent (existing install) ---
SANDBOX="$(mktemp -d)"
HOME="${SANDBOX}/home"
mkdir -p "$HOME"
: > "${HOME}/.bashrc"
BIN_DIR="${HOME}/.local/bin"
BIN="${BIN_DIR}/specd"
PATH_SAVED="$PATH"
PATH="/usr/bin:/bin"
setup_path >/dev/null 2>&1
count="$(grep -c '# specd' "${HOME}/.bashrc")"
assert_eq "$count" "1" "setup_path adds PATH line when absent"
# Re-run must not duplicate (existing-install idempotency).
setup_path >/dev/null 2>&1
count="$(grep -c '# specd' "${HOME}/.bashrc")"
assert_eq "$count" "1" "setup_path is idempotent on re-run"
PATH="$PATH_SAVED"
rm -rf "$SANDBOX"

# --- setup_path: PATH present → rc files untouched ---
SANDBOX="$(mktemp -d)"
HOME="${SANDBOX}/home"
mkdir -p "$HOME"
: > "${HOME}/.bashrc"
BIN_DIR="${HOME}/.local/bin"
PATH_SAVED="$PATH"
PATH="${BIN_DIR}:/usr/bin:/bin"
setup_path >/dev/null 2>&1
count="$(grep -c '# specd' "${HOME}/.bashrc" || true)"
assert_eq "$count" "0" "setup_path leaves rc files alone when BIN_DIR on PATH"
PATH="$PATH_SAVED"
rm -rf "$SANDBOX"

# --- finish: PATH present → verifies binary and recommends auto onboarding ---
SANDBOX="$(mktemp -d)"
BIN_DIR="${SANDBOX}/bin"
BIN="${BIN_DIR}/specd"
make_fake_specd "$BIN_DIR" "specd 9.9.9"
PATH_SAVED="$PATH"
PATH="${BIN_DIR}:/usr/bin:/bin"
OUT="$(finish "Installed specd 9.9.9" 2>&1)"
RC=$?
PATH="$PATH_SAVED"
assert_eq "$RC" "0" "finish exits 0 with a runnable binary"
assert_contains "$OUT" "$BIN" "finish prints absolute binary path"
assert_contains "$OUT" "Verified: specd 9.9.9" "finish runs the binary version"
assert_contains "$OUT" "specd init --agent auto" "finish recommends auto onboarding"
assert_missing "$OUT" "not on PATH" "finish skips PATH warning when on PATH"
rm -rf "$SANDBOX"

# --- finish: PATH absent → warns and offers full-path command ---
SANDBOX="$(mktemp -d)"
BIN_DIR="${SANDBOX}/bin"
BIN="${BIN_DIR}/specd"
make_fake_specd "$BIN_DIR" "specd 9.9.9"
PATH_SAVED="$PATH"
PATH="/usr/bin:/bin"
OUT="$(finish "Installed specd 9.9.9" 2>&1)"
PATH="$PATH_SAVED"
assert_contains "$OUT" "not on PATH" "finish warns when BIN_DIR absent from PATH"
assert_contains "$OUT" "${BIN} init --agent auto" "finish offers full-path command"
rm -rf "$SANDBOX"

# --- finish: missing binary → fails closed ---
SANDBOX="$(mktemp -d)"
BIN="${SANDBOX}/bin/specd"  # never created
OUT="$(finish "Installed" 2>&1)"
RC=$?
assert_eq "$RC" "1" "finish fails closed when binary cannot run"
assert_contains "$OUT" "did not run" "finish reports the verification failure"
rm -rf "$SANDBOX"

# --- no project mutation: running from a project dir leaves it untouched ---
SANDBOX="$(mktemp -d)"
PROJECT="${SANDBOX}/project"
mkdir -p "$PROJECT"
echo "user code" > "${PROJECT}/main.go"
BEFORE="$(ls -A "$PROJECT")"
HOME="${SANDBOX}/home"
mkdir -p "$HOME"
BIN_DIR="${HOME}/.local/bin"
BIN="${BIN_DIR}/specd"
make_fake_specd "$BIN_DIR" "specd 9.9.9"
PATH_SAVED="$PATH"
PATH="/usr/bin:/bin"
( cd "$PROJECT" && setup_path >/dev/null 2>&1 && finish "Installed" >/dev/null 2>&1 )
PATH="$PATH_SAVED"
AFTER="$(ls -A "$PROJECT")"
assert_eq "$AFTER" "$BEFORE" "installer does not mutate the project directory"
assert_missing "$AFTER" ".specd" "installer does not initialize the project"
rm -rf "$SANDBOX"

# --- verify_checksum: rejects mismatch, accepts match, --no-verify skips it ---
SANDBOX="$(mktemp -d)"
ARCHIVE="specd_test_archive.tar.gz"
echo "fake release bytes" > "${SANDBOX}/${ARCHIVE}"
WANT_HASH="$(cd "$SANDBOX" && sha256sum "$ARCHIVE" 2>/dev/null | awk '{print $1}')"
if [ -z "$WANT_HASH" ]; then
  WANT_HASH="$(cd "$SANDBOX" && shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
fi
WRONG_HASH="0000000000000000000000000000000000000000000000000000000000000000"

# download() normally fetches SHA256SUMS from GitHub; stub it so verify_checksum's
# own pass/fail logic is exercised against a local fixture instead of the network.
download() { cp "${SANDBOX}/SHA256SUMS.fixture" "$2"; }

printf '%s  %s\n' "$WRONG_HASH" "$ARCHIVE" > "${SANDBOX}/SHA256SUMS.fixture"
( verify_checksum "$SANDBOX" "$ARCHIVE" "v9.9.9" >/dev/null 2>&1 )
assert_eq "$?" "1" "verify_checksum exits non-zero on checksum mismatch"

printf '%s  %s\n' "$WANT_HASH" "$ARCHIVE" > "${SANDBOX}/SHA256SUMS.fixture"
( verify_checksum "$SANDBOX" "$ARCHIVE" "v9.9.9" >/dev/null 2>&1 )
assert_eq "$?" "0" "verify_checksum succeeds on matching checksum"

printf '%s  %s\n' "$WRONG_HASH" "$ARCHIVE" > "${SANDBOX}/SHA256SUMS.fixture"
NO_VERIFY=true
( verify_checksum "$SANDBOX" "$ARCHIVE" "v9.9.9" >/dev/null 2>&1 )
assert_eq "$?" "0" "--no-verify skips checksum verification even on mismatch"
NO_VERIFY=false

unset -f download
rm -rf "$SANDBOX"

# --- source fallback also verifies + hands off (static wiring check) ---
SRC="$(cat "${SCRIPT_DIR}/install.sh")"
case "$SRC" in
  *"setup_path"*"finish "*) ok ;;
  *) bad "build_from_source must call setup_path and finish" ;;
esac

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
