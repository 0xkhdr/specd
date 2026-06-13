// Per-spec advisory lock: serializes the state.json read→modify→write cycle across processes so
// two agents driving the same spec in parallel (philosophy §4 "waves run in parallel") can't lose
// an update. An O_EXCL lockfile at .specd/specs/<slug>/.lock is the cross-process gate; a per-process
// reentrancy counter lets nested acquisitions (e.g. a command that wraps loadSpec, which also locks)
// pass through without deadlocking. Stale locks (orphaned by a crashed process) are reclaimed.
import { openSync, closeSync, writeSync, unlinkSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { specDir } from "./paths.js";
import { gateError } from "./exit.js";

const numEnv = (name: string, fallback: number): number => {
  const v = parseInt(process.env[name] ?? "", 10);
  return Number.isFinite(v) && v >= 0 ? v : fallback;
};

// A lock older than STALE_MS is presumed orphaned by a dead process and reclaimed. TIMEOUT_MS caps
// how long we wait for a live holder before failing loudly (exit 1). Both env-overridable for tests.
const staleMs = () => numEnv("SPECD_LOCK_STALE_MS", 30_000);
const timeoutMs = () => numEnv("SPECD_LOCK_TIMEOUT_MS", 5_000);
const RETRY_MS = 25;

/** Blocking sleep without deps — Atomics.wait parks the thread for `ms` (CLI is single-threaded). */
function sleep(ms: number): void {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

const lockPath = (root: string, slug: string): string => join(specDir(root, slug), ".lock");

/** Try to create the lockfile exclusively. Returns true on success, false if held, throws otherwise. */
function tryAcquire(path: string): boolean {
  try {
    const fd = openSync(path, "wx"); // O_CREAT | O_EXCL | O_WRONLY
    writeSync(fd, `${process.pid} ${Date.now()}\n`);
    closeSync(fd);
    return true;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "EEXIST") return false;
    throw err;
  }
}

/** True if the existing lock is older than the stale threshold (holder presumed dead). */
function isStale(path: string): boolean {
  try {
    const ts = parseInt(readFileSync(path, "utf8").trim().split(/\s+/)[1] ?? "", 10);
    const age = Date.now() - (Number.isFinite(ts) ? ts : statSync(path).mtimeMs);
    return age > staleMs();
  } catch {
    return false; // lock vanished between checks — not stale, just gone
  }
}

// Reentrancy: lockfile depth per path within this process. A nested withSpecLock on a slug we
// already hold is a no-op acquire — only the outermost frame creates and removes the lockfile.
const held = new Map<string, number>();

/**
 * Run `fn` while holding the per-spec lock. Acquires the O_EXCL lockfile (reclaiming a stale one),
 * waits up to the timeout for a live holder, and throws gateError (exit 1) on contention. The
 * lockfile is always released in `finally`. Reentrant within a single process.
 */
export function withSpecLock<T>(root: string, slug: string, fn: () => T): T {
  const path = lockPath(root, slug);

  if (held.has(path)) {
    held.set(path, held.get(path)! + 1);
    try {
      return fn();
    } finally {
      const depth = held.get(path)! - 1;
      if (depth === 0) held.delete(path);
      else held.set(path, depth);
    }
  }

  const deadline = Date.now() + timeoutMs();
  for (;;) {
    if (tryAcquire(path)) break;
    if (isStale(path)) {
      try { unlinkSync(path); } catch { /* another process reclaimed it first */ }
      continue;
    }
    if (Date.now() >= deadline) {
      throw gateError(`spec '${slug}' is locked by another specd process — retry shortly, or remove ${path} if it is stale`);
    }
    sleep(RETRY_MS);
  }

  held.set(path, 1);
  try {
    return fn();
  } finally {
    held.delete(path);
    try { unlinkSync(path); } catch { /* already released */ }
  }
}
