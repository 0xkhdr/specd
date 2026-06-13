// Atomic file IO: write-to-temp + rename so a crash never leaves a partial file (SPEC §12).
import { appendFileSync, closeSync, fsyncSync, openSync, readFileSync, renameSync, writeFileSync, existsSync, mkdirSync, unlinkSync } from "node:fs";
import { dirname, join } from "node:path";

/** Read a UTF-8 file, or return `fallback` if it does not exist. */
export function readOrDefault(path: string, fallback: string): string {
  if (!existsSync(path)) return fallback;
  return readFileSync(path, "utf8");
}

/** Read a UTF-8 file or null if absent. */
export function readOrNull(path: string): string | null {
  if (!existsSync(path)) return null;
  return readFileSync(path, "utf8");
}

/**
 * Atomically write `data` to `path`. Writes to a sibling temp file, fsyncs, then renames.
 * rename(2) is atomic on POSIX, so readers see either the old or the new file, never a partial.
 */
export function atomicWrite(path: string, data: string): void {
  const dir = dirname(path);
  mkdirSync(dir, { recursive: true });
  const tmp = join(dir, `.${process.pid}.${Date.now()}.${Math.random().toString(36).slice(2)}.tmp`);
  const fd = openSync(tmp, "w");
  try {
    writeFileSync(fd, data, "utf8");
    fsyncSync(fd);
  } finally {
    closeSync(fd);
  }
  try {
    renameSync(tmp, path);
  } catch (err) {
    try { unlinkSync(tmp); } catch { /* ignore */ }
    throw err;
  }
}

/**
 * Append a block to a file, creating it if needed. Uses a single O_APPEND write (`flag: "a"`) so
 * concurrent appends to a ledger never lose an entry — the kernel serializes appends to one fd, so
 * there is no read-modify-write window like the old `read + atomicWrite(existing + data)` had (§5.2).
 */
export function appendFile(path: string, data: string): void {
  mkdirSync(dirname(path), { recursive: true });
  appendFileSync(path, data, { encoding: "utf8" });
}
