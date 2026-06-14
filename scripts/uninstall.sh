#!/bin/sh
# specd uninstall script — POSIX-compliant, zero-dependency cleanup
# Usage: curl -fsSL ... | bash

set -e

# --- UI Helpers (respect NO_COLOR) ---
RESET=""
RED=""
GREEN=""
YELLOW=""

if [ -t 1 ] && { [ -z "${NO_COLOR}" ] || [ "${NO_COLOR}" = "0" ]; }; then
  RESET="\033[0m"
  RED="\033[31m"
  GREEN="\033[32m"
  YELLOW="\033[33m"
fi

log_step() {
  label="$1"
  status="$2"
  case "$status" in
    pending)
      printf "[specd] %-30s" "$label..."
      ;;
    done)
      printf ' %s✓%s\n' "${GREEN}" "${RESET}"
      ;;
    failed)
      printf ' %s❌%s\n' "${RED}" "${RESET}"
      ;;
  esac
}

log_warn() {
  printf '[specd] %s⚠️ Warning: %s%s\n' "${YELLOW}" "$1" "${RESET}"
}

main() {
  # --- Detect Platforms & Paths ---
  OS="$(uname -s)"
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

  if [ "$DETECTED_OS" = "Linux" ] || [ "$DETECTED_OS" = "Windows/WSL" ]; then
    INSTALL_DIR="${HOME}/.local/share/specd"
  else
    INSTALL_DIR="${HOME}/.specd-repo"
  fi
  BIN_LINK="${HOME}/.local/bin/specd"

  # --- Check if installed ---
  PATH_ENTRIES_FOUND=false
  for shell_config in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
    if [ -f "$shell_config" ] && grep -q "# specd PATH" "$shell_config"; then
      PATH_ENTRIES_FOUND=true
    fi
  done

  if [ ! -d "$INSTALL_DIR" ] && [ ! -h "$BIN_LINK" ] && [ "$PATH_ENTRIES_FOUND" = "false" ]; then
    printf "[specd] Nothing to uninstall.\n"
    exit 0
  fi

  # --- Remove Artifacts ---
  log_step "🗑️  Removing installation" "pending"
  rm -rf "$INSTALL_DIR"
  rm -f "$BIN_LINK"
  log_step "🗑️  Removing installation" "done"

  # --- PATH Cleanup ---
  log_step "🧹 Cleaning PATH entries" "pending"
  for shell_config in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
    if [ -f "$shell_config" ] && grep -q "# specd PATH" "$shell_config"; then
      # Backup
      cp "$shell_config" "${shell_config}.specd.bak"
      # Remove lines (using portable grep -v fallback to avoid sed incompatibility)
      temp_file="${shell_config}.tmp"
      grep -v "# specd PATH" "$shell_config" > "$temp_file" || true
      mv "$temp_file" "$shell_config"
    fi
  done
  log_step "🧹 Cleaning PATH entries" "done"

  # --- Data Preservation Warning ---
  printf '[specd] %s✅ Uninstallation complete.%s\n' "${GREEN}" "${RESET}"
  log_warn "Any local project-specific '.specd/' directories have been preserved."
}

main "$@"
