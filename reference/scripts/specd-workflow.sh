#!/bin/sh
# Optional UX wrappers for specd slash workflows.
# Thin glue only: mutations delegate to native `specd`.

_specd_workflow_script_dir() {
  case $0 in
    */*) dirname -- "$0" ;;
    *) pwd ;;
  esac
}

_specd_workflow_py() {
  if [ -n "${SPECD_WORKFLOW_PY:-}" ]; then
    printf '%s\n' "$SPECD_WORKFLOW_PY"
    return 0
  fi
  # When sourced, ${BASH_SOURCE:-} or zsh ${(%):-%x} may be unavailable in POSIX sh.
  # Prefer script-relative path; fallback to repository scripts path from cwd/parents.
  candidate="$(_specd_workflow_script_dir)/specd-workflow.py"
  if [ -f "$candidate" ]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  d=$PWD
  while :; do
    candidate="$d/scripts/specd-workflow.py"
    if [ -f "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
    [ "$d" = / ] && break
    d=$(dirname -- "$d")
  done
  return 1
}

specd_workflow() {
  py=$(_specd_workflow_py) || {
    printf '%s\n' 'specd-workflow.py not found; set SPECD_WORKFLOW_PY' >&2
    return 3
  }
  if command -v python3 >/dev/null 2>&1; then
    python3 "$py" "$@"
    return $?
  fi
  printf '%s\n' 'python3 not found; use native specd commands directly' >&2
  return 3
}

# Portable command names.
specd_workflow_init() { specd_workflow init "$@"; }
specd_workflow_steer() { specd_workflow steer "$@"; }
specd_workflow_spec() { specd_workflow spec "$@"; }
specd_workflow_pinky_brain() { specd_workflow pinky-brain "$@"; }

# Slash-command hosts can map /init, /steer, /spec, and /pinky-brain to:
#   specd_workflow init|steer|spec|pinky-brain
# POSIX shells do not portably allow function or alias names containing '/'.
