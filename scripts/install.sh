#!/bin/sh
# specd install script — downloads pre-built binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash
#        curl -fsSL ... | bash -s -- --force --version v0.1.0

set -e

REPO="0xkhdr/specd"
FORCE=false
VERSION=""
VERBOSE=false
NO_VERIFY=false
BIN_DIR="${HOME}/.local/bin"
BIN="${BIN_DIR}/specd"

# --- Colors ---
# Materialize escape sequences into the variables (via tput, falling back to
# `printf '%b'`) so the log helpers never embed raw \033 in a format string —
# POSIX /bin/sh printf does not reliably interpret backslash escapes there.
RESET="" GREEN="" RED="" YELLOW="" BLUE=""
if [ -t 1 ] && [ -z "${NO_COLOR}" ]; then
  if command -v tput >/dev/null 2>&1 && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
    RESET="$(tput sgr0)"; GREEN="$(tput setaf 2)"; RED="$(tput setaf 1)"
    YELLOW="$(tput setaf 3)"; BLUE="$(tput setaf 4)"
  else
    RESET="$(printf '%b' '\033[0m')"; GREEN="$(printf '%b' '\033[32m')"
    RED="$(printf '%b' '\033[31m')"; YELLOW="$(printf '%b' '\033[33m')"
    BLUE="$(printf '%b' '\033[34m')"
  fi
fi

log()  { printf "[specd] %s\n" "$*"; }
ok()   { printf "[specd] ${GREEN}✓${RESET} %s\n" "$*"; }
warn() { printf "[specd] ${YELLOW}⚠ %s${RESET}\n" "$*"; }
die()  { printf "[specd] ${RED}✗ %s${RESET}\n" "$*" >&2; exit 1; }

download() {
  url="$1"; dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest" || return 1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url" || return 1
  else
    die "Neither curl nor wget found. Install one and retry."
  fi
}

verify_checksum() {
  # verify_checksum <dir> <archive> <version>
  dir="$1"; archive="$2"; version="$3"
  if [ "$NO_VERIFY" = "true" ]; then
    warn "Skipping checksum verification (--no-verify)"
    return 0
  fi
  log "Verifying checksum..."
  download "https://github.com/${REPO}/releases/download/${version}/SHA256SUMS" "${dir}/SHA256SUMS" \
    || die "Could not download SHA256SUMS for ${version} (use --no-verify to override)."
  (
    cd "$dir"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum --ignore-missing -c SHA256SUMS >/dev/null 2>&1
    elif command -v shasum >/dev/null 2>&1; then
      # shasum lacks --ignore-missing; check just our archive line.
      grep " ${archive}\$" SHA256SUMS | shasum -a 256 -c - >/dev/null 2>&1
    else
      die "Neither sha256sum nor shasum found (use --no-verify to override)."
    fi
  ) || die "Checksum verification failed for ${archive}"
  ok "Checksum verified"
}

build_from_source() {
  command -v go >/dev/null 2>&1 || die "Go is required to build from source. Install Go 1.22+ and retry."

  TMPDIR="$(mktemp -d)"
  REPO_DIR="${TMPDIR}/specd"

  log "Cloning repository..."
  git clone "https://github.com/${REPO}.git" "$REPO_DIR" || die "Failed to clone repository."

  log "Building specd..."
  (
    cd "$REPO_DIR"
    go build -ldflags "-s -w -X main.version=dev" -o specd . || die "Build failed."
  )

  log "Installing..."
  mkdir -p "$BIN_DIR"
  mv "${REPO_DIR}/specd" "$BIN"
  chmod +x "$BIN"
  rm -rf "$TMPDIR"

  # --- PATH ---
  case ":${PATH}:" in
    *:"${BIN_DIR}":*) ;;
    *)
      LINE="export PATH=\"\${HOME}/.local/bin:\${PATH}\" # specd"
      for rc in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
        if [ -f "$rc" ] && ! grep -q "# specd" "$rc"; then
          printf '\n%s\n' "$LINE" >> "$rc"
        fi
      done
      warn "${BIN_DIR} added to PATH in shell configs. Run: source ~/.bashrc"
      ;;
  esac

  ok "Built and installed specd from source → ${BIN}"
  printf '[specd] %sRun '\''specd init'\'' in your project root to get started.%s\n' "${BLUE}" "${RESET}"
}

main() {
  # --- Parse args ---
  while [ $# -gt 0 ]; do
    case "$1" in
      --force)     FORCE=true;     shift ;;
      --verbose)   VERBOSE=true;   shift ;;
      --no-verify) NO_VERIFY=true; shift ;;
      --version)
        [ -z "$2" ] && die "--version requires a value"
        VERSION="$2"; shift 2 ;;
      *) die "Unknown argument: $1" ;;
    esac
  done

  # --- Platform detection ---
  OS="$(uname -s)"
  ARCH="$(uname -m)"

  case "$OS" in
    Linux)  GOOS="linux" ;;
    Darwin) GOOS="darwin" ;;
    *) die "Unsupported OS: $OS" ;;
  esac

  case "$ARCH" in
    x86_64)         GOARCH="amd64" ;;
    aarch64|arm64)  GOARCH="arm64" ;;
    *) die "Unsupported architecture: $ARCH" ;;
  esac

  # --- Resolve version ---
  if [ -z "$VERSION" ]; then
    log "Fetching latest release version..."
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | grep '"tag_name"' | head -n1 | cut -d'"' -f4)" || true

    if [ -z "$VERSION" ]; then
      warn "No releases found. Building from source..."
      build_from_source
      exit 0
    fi
  fi

  # Normalise: ensure leading 'v'
  case "$VERSION" in
    v*) ;;
    *)  VERSION="v${VERSION}" ;;
  esac

  # --- Idempotency ---
  if [ "$FORCE" = "false" ] && [ -x "$BIN" ]; then
    INSTALLED="$("$BIN" version 2>/dev/null | head -n1 | awk '{print $NF}' || true)"
    if [ "$INSTALLED" = "$VERSION" ]; then
      ok "Already installed (${VERSION})"
      exit 0
    fi
  fi

  # --- Download ---
  ARCHIVE="specd_${GOOS}_${GOARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
  TMPDIR="$(mktemp -d)"
  TMPFILE="${TMPDIR}/${ARCHIVE}"

  log "Downloading specd ${VERSION} (${GOOS}/${GOARCH})..."
  [ "$VERBOSE" = "true" ] && log "URL: ${URL}"
  download "$URL" "$TMPFILE" || {
    warn "Release binary not found. Building from source..."
    rm -rf "$TMPDIR"
    build_from_source
    exit 0
  }

  # --- Verify ---
  verify_checksum "$TMPDIR" "$ARCHIVE" "$VERSION"

  # --- Extract ---
  log "Extracting..."
  tar -xzf "$TMPFILE" -C "$TMPDIR"

  EXTRACTED_BIN="${TMPDIR}/specd"
  [ -f "$EXTRACTED_BIN" ] || die "Binary not found in archive."

  # --- Install ---
  mkdir -p "$BIN_DIR"
  mv "$EXTRACTED_BIN" "$BIN"
  chmod +x "$BIN"
  rm -rf "$TMPDIR"

  # --- PATH ---
  case ":${PATH}:" in
    *:"${BIN_DIR}":*) ;;
    *)
      LINE="export PATH=\"\${HOME}/.local/bin:\${PATH}\" # specd"
      for rc in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
        if [ -f "$rc" ] && ! grep -q "# specd" "$rc"; then
          printf '\n%s\n' "$LINE" >> "$rc"
        fi
      done
      warn "${BIN_DIR} added to PATH in shell configs. Run: source ~/.bashrc"
      ;;
  esac

  ok "Installed specd ${VERSION} → ${BIN}"
  printf '[specd] %sRun '\''specd init'\'' in your project root to get started.%s\n' "${BLUE}" "${RESET}"
}

main "$@"
