#!/usr/bin/env bash
# Shared guards for stress scripts. POSIX-only, no runtime deps beyond common
# shell tools. Bounds are intentionally generous: they sit far above normal CI
# use but catch runaway process/fd leaks before they exhaust the host.

stress_set_limits() {
  target="${1:-stress}"
  proc_limit="${SPECD_STRESS_PROC_LIMIT:-2048}"
  fd_limit="${SPECD_STRESS_FD_LIMIT:-4096}"

  if ulimit -u "$proc_limit" 2>/dev/null; then
    :
  else
    echo "WARN: ${target}: process-count ulimit unavailable on this shell/platform" >&2
  fi
  if ulimit -n "$fd_limit" 2>/dev/null; then
    :
  else
    echo "WARN: ${target}: open-file ulimit unavailable on this shell/platform" >&2
  fi
}

stress_fd_count() {
  if [ -d "/proc/$$/fd" ]; then
    find "/proc/$$/fd" -maxdepth 1 2>/dev/null | wc -l | tr -d ' '
  elif command -v lsof >/dev/null 2>&1; then
    lsof -p "$$" 2>/dev/null | awk 'NR > 1 {n++} END {print n + 0}'
  else
    echo 0
  fi
}

stress_jobs_count() {
  jobs -p 2>/dev/null | wc -l | tr -d ' '
}

stress_guard_begin() {
  STRESS_GUARD_TARGET="${1:-stress}"
  STRESS_GUARD_FD_BEFORE="$(stress_fd_count)"
  STRESS_GUARD_JOBS_BEFORE="$(stress_jobs_count)"
  export STRESS_GUARD_TARGET STRESS_GUARD_FD_BEFORE STRESS_GUARD_JOBS_BEFORE
}

stress_guard_end() {
  target="${STRESS_GUARD_TARGET:-stress}"
  fd_tolerance="${SPECD_STRESS_FD_TOLERANCE:-4}"
  # One completed job entry may remain in non-interactive shells until final reap;
  # anything above that indicates an unexpected child-process leak.
  jobs_tolerance="${SPECD_STRESS_JOB_TOLERANCE:-1}"

  # Let short-lived child cleanup and Go test process teardown settle.
  sleep "${SPECD_STRESS_SETTLE_SECONDS:-1}"

  fd_after="$(stress_fd_count)"
  jobs_after="$(stress_jobs_count)"
  fd_before="${STRESS_GUARD_FD_BEFORE:-0}"
  jobs_before="${STRESS_GUARD_JOBS_BEFORE:-0}"
  fd_growth=$((fd_after - fd_before))
  jobs_growth=$((jobs_after - jobs_before))

  if [ "$fd_growth" -gt "$fd_tolerance" ]; then
    echo "FAIL: ${target}: open-file leak detected — before=${fd_before} after=${fd_after} tolerance=${fd_tolerance}" >&2
    return 1
  fi
  if [ "$jobs_growth" -gt "$jobs_tolerance" ]; then
    echo "FAIL: ${target}: child-process leak detected — before=${jobs_before} after=${jobs_after} tolerance=${jobs_tolerance}" >&2
    return 1
  fi
}
