#!/bin/sh
# specd install script — POSIX-compliant, zero-dependency bootstrap
# Usage: curl -fsSL ... | bash
#        curl -fsSL ... | bash -s -- --force --version 0.2.0

set -e

# --- Configuration ---
REPO_URL="https://github.com/0xkhdr/specd"
RAW_URL="https://raw.githubusercontent.com/0xkhdr/specd"
FORCE=false
VERSION="main"
VERBOSE=false
ALLOW_ROOT=false

# --- UI Helpers (respect NO_COLOR) ---
RESET=""
BOLD=""
RED=""
GREEN=""
YELLOW=""
BLUE=""

if [ -t 1 ] && { [ -z "${NO_COLOR}" ] || [ "${NO_COLOR}" = "0" ]; }; then
  RESET="\033[0m"
  BOLD="\033[1m"
  RED="\033[31m"
  GREEN="\033[32m"
  YELLOW="\033[33m"
  BLUE="\033[34m"
fi

log_step() {
  label="$1"
  status="$2"
  case "$status" in
    pending)
      printf "[specd] 🔍 %-30s" "$label..."
      ;;
    done)
      printf " ${GREEN}✓${RESET}\n"
      ;;
    done_val)
      val="$3"
      printf " ${GREEN}✓${RESET} %s\n" "$val"
      ;;
    failed)
      printf " ${RED}❌${RESET}\n"
      ;;
  esac
}

log_error() {
  printf "[specd] ${RED}❌ Error: %s${RESET}\n" "$1" >&2
}

log_warn() {
  printf "[specd] ${YELLOW}⚠️ Warning: %s${RESET}\n" "$1"
}

# --- Download Helper ---
download_file() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ "$VERBOSE" = "true" ]; then
      curl -fsSL "$url" -o "$dest"
    else
      curl -fsSL "$url" -o "$dest" >/dev/null 2>&1
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ "$VERBOSE" = "true" ]; then
      wget -qO "$dest" "$url"
    else
      wget -qO "$dest" "$url" >/dev/null 2>&1
    fi
  else
    return 1
  fi
}

main() {
  # --- Parse args ---
  while [ $# -gt 0 ]; do
    case "$1" in
      --force)
        FORCE=true
        shift
        ;;
      --verbose)
        VERBOSE=true
        shift
        ;;
      --allow-root)
        ALLOW_ROOT=true
        shift
        ;;
      --version)
        if [ -z "$2" ]; then
          log_error "Missing value for --version"
          exit 2
        fi
        VERSION="$2"
        shift 2
        ;;
      *)
        log_error "Unknown argument: $1"
        exit 2
        ;;
    esac
  done

  # --- Check root ---
  if [ "$(id -u)" -eq 0 ] && [ "$ALLOW_ROOT" = "false" ]; then
    log_error "Running as root is not recommended. Use --allow-root if you really want to proceed."
    exit 1
  fi

  # --- Platform Detection ---
  OS="$(uname -s)"
  ARCH="$(uname -m)"
  DETECTED_OS=""
  case "$OS" in
    Linux)
      if grep -qEi "(Microsoft|WSL)" /proc/version 2>/dev/null; then
        DETECTED_OS="Windows/WSL"
      else
        DETECTED_OS="Linux"
      fi
      ;;
    Darwin) DETECTED_OS="macOS" ;;
    *) DETECTED_OS="Unsupported ($OS)" ;;
  esac

  case "$DETECTED_OS" in
    Unsupported*)
      log_warn "Unsupported OS detected: $OS. Installation may fail."
      ;;
  esac

  # --- Set Install Dirs ---
  if [ "$DETECTED_OS" = "Linux" ] || [ "$DETECTED_OS" = "Windows/WSL" ]; then
    INSTALL_DIR="${HOME}/.local/share/specd"
  else
    INSTALL_DIR="${HOME}/.specd-repo"
  fi
  BIN_DIR="${HOME}/.local/bin"
  BIN_LINK="${BIN_DIR}/specd"

  # --- Node.js Check ---
  log_step "Checking Node.js" "pending"
  if ! command -v node >/dev/null 2>&1; then
    log_step "Checking Node.js" "failed"
    echo ""
    log_error "Node.js is not installed."
    echo "Please install Node.js >= 18:"
    echo "  - macOS: brew install node"
    echo "  - Debian/Ubuntu: curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt-get install -y nodejs"
    echo "  - Windows/WSL: install nvm-windows or nvm inside WSL."
    exit 1
  fi

  NODE_VER_RAW="$(node -v)"
  NODE_VER="$(echo "$NODE_VER_RAW" | tr -d 'v')"
  NODE_MAJOR="$(echo "$NODE_VER" | cut -d. -f1)"
  if [ "$NODE_MAJOR" -lt 18 ]; then
    log_step "Checking Node.js" "failed"
    echo ""
    log_error "Node.js version $NODE_VER_RAW is too old. Node.js >= 18 is required."
    exit 1
  fi
  log_step "Checking Node.js" "done_val" "$NODE_VER_RAW"

  # --- Idempotency Check ---
  TARGET_VERSION="$VERSION"
  if [ "$VERSION" = "main" ]; then
    # Fetch LATEST_VERSION to check if we need to download anything
    LATEST_VER_STR="$(curl -fsSL ${RAW_URL}/main/LATEST_VERSION 2>/dev/null | tr -d 'v' | tr -d '\n' || true)"
    if [ -n "$LATEST_VER_STR" ]; then
      TARGET_VERSION="$LATEST_VER_STR"
    fi
  fi

  CLEAN_TARGET_VER="$(echo "$TARGET_VERSION" | tr -d 'v')"
  CLEAN_INSTALLED_VER=""
  if [ -f "$INSTALL_DIR/package.json" ]; then
    CLEAN_INSTALLED_VER="$(grep '"version"' "$INSTALL_DIR/package.json" | head -n 1 | cut -d'"' -f4 | tr -d 'v' || true)"
  fi

  if [ "$CLEAN_TARGET_VER" = "$CLEAN_INSTALLED_VER" ] && [ -x "$BIN_LINK" ] && [ "$FORCE" = "false" ]; then
    printf "[specd] Already installed (v%s)\n" "$CLEAN_INSTALLED_VER"
    exit 0
  fi

  # --- Warning for Force ---
  if [ "$FORCE" = "true" ]; then
    log_warn "Force flag passed. Existing tool binary at $INSTALL_DIR will be replaced. Project data (.specd/ directories) will be preserved."
  fi

  # --- Clone/Download repository ---
  log_step "Cloning repository" "pending"
  if [ -d "$INSTALL_DIR" ]; then
    rm -rf "$INSTALL_DIR"
  fi

  if command -v git >/dev/null 2>&1; then
    # Git is available
    if [ "$VERSION" = "main" ]; then
      if [ "$VERBOSE" = "true" ]; then
        git clone --depth 1 "$REPO_URL" "$INSTALL_DIR"
      else
        git clone --depth 1 "$REPO_URL" "$INSTALL_DIR" >/dev/null 2>&1
      fi
    else
      # Pinned version/tag
      # Ensure tag has 'v' prefix
      TAG_REF="$VERSION"
      case "$VERSION" in
        v*) ;;
        *) TAG_REF="v$VERSION" ;;
      esac
      if [ "$VERBOSE" = "true" ]; then
        git clone --depth 1 --branch "$TAG_REF" "$REPO_URL" "$INSTALL_DIR"
      else
        git clone --depth 1 --branch "$TAG_REF" "$REPO_URL" "$INSTALL_DIR" >/dev/null 2>&1
      fi
    fi
    log_step "Cloning repository" "done"
  else
    # Tarball fallback
    log_step "Cloning repository" "failed"
    log_warn "git is not available. Falling back to source tarball."
    log_step "Downloading source tarball" "pending"

    if [ "$VERSION" = "main" ]; then
      TARBALL_URL="${REPO_URL}/archive/refs/heads/main.tar.gz"
    else
      TAG_REF="$VERSION"
      case "$VERSION" in
        v*) ;;
        *) TAG_REF="v$VERSION" ;;
      esac
      TARBALL_URL="${REPO_URL}/archive/refs/tags/${TAG_REF}.tar.gz"
    fi

    TEMP_TAR="/tmp/specd-source.tar.gz"
    if ! download_file "$TARBALL_URL" "$TEMP_TAR"; then
      log_step "Downloading source tarball" "failed"
      log_error "Could not download tarball from $TARBALL_URL"
      exit 1
    fi
    log_step "Downloading source tarball" "done"

    log_step "Extracting source tarball" "pending"
    TEMP_EXTRACT_DIR="/tmp/specd-extract-$$"
    mkdir -p "$TEMP_EXTRACT_DIR"
    tar -xzf "$TEMP_TAR" -C "$TEMP_EXTRACT_DIR"
    
    EXTRACTED_DIR="$(find "$TEMP_EXTRACT_DIR" -maxdepth 1 -mindepth 1 -type d | head -n 1)"
    mkdir -p "$INSTALL_DIR"
    cp -r "$EXTRACTED_DIR"/* "$INSTALL_DIR"/
    
    rm -rf "$TEMP_EXTRACT_DIR" "$TEMP_TAR"
    log_step "Extracting source tarball" "done"
  fi

  # --- Build step ---
  log_step "Building from source" "pending"
  if [ "$VERBOSE" = "true" ]; then
    (cd "$INSTALL_DIR" && npm install && npm run build)
  else
    (cd "$INSTALL_DIR" && npm install >/dev/null 2>&1 && npm run build >/dev/null 2>&1)
  fi
  log_step "Building from source" "done"

  # --- Link Binary ---
  log_step "Linking binary" "pending"
  mkdir -p "$BIN_DIR"
  rm -f "$BIN_LINK"
  ln -s "$INSTALL_DIR/dist/cli.js" "$BIN_LINK"
  chmod +x "$INSTALL_DIR/dist/cli.js"
  log_step "Linking binary" "done"

  # --- PATH Management ---
  case ":$PATH:" in
    *:"$BIN_DIR":*) ;;
    *)
      APPEND_LINE="export PATH=\"\$HOME/.local/bin:\$PATH\" # specd PATH"
      UPDATED=false
      for shell_config in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
        if [ -f "$shell_config" ]; then
          if ! grep -q "# specd PATH" "$shell_config"; then
            echo "" >> "$shell_config"
            echo "$APPEND_LINE" >> "$shell_config"
            UPDATED=true
          fi
        fi
      done
      if [ "$UPDATED" = "true" ]; then
        log_warn "Added $BIN_DIR to PATH in shell configs. Please restart your shell or run: source ~/.bashrc"
      fi
      ;;
  esac

  # --- Verify ---
  INSTALLED_VER="unknown"
  if [ -f "$INSTALL_DIR/package.json" ]; then
    INSTALLED_VER="v$(grep '"version"' "$INSTALL_DIR/package.json" | head -n 1 | cut -d'"' -f4 || echo "unknown")"
  fi

  printf "[specd] ${GREEN}✅ Installation complete!${RESET}     specd %s\n" "$INSTALLED_VER"
  printf "[specd] ${BLUE}💡 Run 'specd --help' to get started.${RESET}\n"
}

main "$@"
