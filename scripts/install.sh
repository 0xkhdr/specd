#!/bin/sh
# specd install script — downloads pre-built binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh | bash
#        curl -fsSL ... | bash -s -- --force --version v0.3.0

set -e

REPO="0xkhdr/specd"
FORCE=false
VERSION=""
VERBOSE=false
BIN_DIR="${HOME}/.local/bin"
BIN="${BIN_DIR}/specd"

# --- Colors ---
RESET="" GREEN="" RED="" YELLOW="" BLUE=""
if [ -t 1 ] && [ -z "${NO_COLOR}" ]; then
  RESET="\033[0m" GREEN="\033[32m" RED="\033[31m" YELLOW="\033[33m" BLUE="\033[34m"
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
  printf "[specd] ${BLUE}Run 'specd init' in your project root to get started.${RESET}\n"
}

main() {
  # --- Parse args ---
  while [ $# -gt 0 ]; do
    case "$1" in
      --force)   FORCE=true;  shift ;;
      --verbose) VERBOSE=true; shift ;;
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
  printf "[specd] ${BLUE}Run 'specd init' in your project root to get started.${RESET}\n"
}

main "$@"
