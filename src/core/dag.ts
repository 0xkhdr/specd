// Dependency-graph engine: waves, next-runnable (SPEC §5.3), cycle & orphan detection.
import type { TaskStatus } from "./state.js";

export interface DagTask {
  id: string;
  wave: number;
  depends: string[];
  status: TaskStatus;
}

export type NextResult =
  | { kind: "task"; id: string }
  | { kind: "all-complete" }
  | { kind: "all-blocked"; blocked: string[] }
  | { kind: "waiting"; blocking: string[] };

const ordinal = (id: string): number => {
  const m = id.match(/\d+/);
  return m ? parseInt(m[0], 10) : Number.MAX_SAFE_INTEGER;
};

const byId = (tasks: DagTask[]): Map<string, DagTask> => new Map(tasks.map((t) => [t.id, t]));

/** Dep ids referenced by some task but not defined. Returns [{task, dep}]. */
export function orphanDeps(tasks: DagTask[]): { task: string; dep: string }[] {
  const ids = new Set(tasks.map((t) => t.id));
  const out: { task: string; dep: string }[] = [];
  for (const t of tasks) for (const d of t.depends) if (!ids.has(d)) out.push({ task: t.id, dep: d });
  return out;
}

/** Detect a dependency cycle. Returns the cycle path (ids) or null if acyclic. */
export function detectCycle(tasks: DagTask[]): string[] | null {
  const map = byId(tasks);
  const WHITE = 0, GREY = 1, BLACK = 2;
  const color = new Map<string, number>(tasks.map((t) => [t.id, WHITE]));
  const stack: string[] = [];

  const dfs = (id: string): string[] | null => {
    color.set(id, GREY);
    stack.push(id);
    const node = map.get(id);
    if (node) {
      for (const dep of node.depends) {
        if (!map.has(dep)) continue; // orphan handled separately
        const c = color.get(dep);
        if (c === GREY) {
          const start = stack.indexOf(dep);
          return [...stack.slice(start), dep];
        }
        if (c === WHITE) {
          const found = dfs(dep);
          if (found) return found;
        }
      }
    }
    stack.pop();
    color.set(id, BLACK);
    return null;
  };

  for (const t of tasks) {
    if (color.get(t.id) === WHITE) {
      const cycle = dfs(t.id);
      if (cycle) return cycle;
    }
  }
  return null;
}

/** Tasks whose deps live in a later wave (wave inconsistency). Returns [{task, dep}]. */
export function waveViolations(tasks: DagTask[]): { task: string; dep: string }[] {
  const map = byId(tasks);
  const out: { task: string; dep: string }[] = [];
  for (const t of tasks) {
    for (const d of t.depends) {
      const dep = map.get(d);
      if (dep && dep.wave > t.wave) out.push({ task: t.id, dep: d });
    }
  }
  return out;
}

const isRunnable = (t: DagTask, map: Map<string, DagTask>): boolean =>
  t.status === "pending" && t.depends.every((d) => map.get(d)?.status === "complete");

/** The single next runnable task per SPEC §5.3, or a classified reason when none. */
export function nextRunnable(tasks: DagTask[]): NextResult {
  const map = byId(tasks);
  const remaining = tasks.filter((t) => t.status !== "complete");
  if (remaining.length === 0) return { kind: "all-complete" };

  const runnable = remaining
    .filter((t) => isRunnable(t, map))
    .sort((a, b) => a.wave - b.wave || ordinal(a.id) - ordinal(b.id));
  if (runnable.length) return { kind: "task", id: runnable[0].id };

  const pending = remaining.filter((t) => t.status === "pending");
  const blocked = remaining.filter((t) => t.status === "blocked").map((t) => t.id);
  if (pending.length === 0 && blocked.length > 0) {
    return { kind: "all-blocked", blocked };
  }
  // waiting: deps of the pending frontier that aren't complete
  const blocking = new Set<string>();
  for (const t of pending) {
    for (const d of t.depends) {
      const dep = map.get(d);
      if (!dep || dep.status !== "complete") blocking.add(d);
    }
  }
  return { kind: "waiting", blocking: [...blocking].sort((a, b) => ordinal(a) - ordinal(b)) };
}

/**
 * Every currently-runnable task (the parallel frontier), sorted by wave then id ordinal. Where
 * `nextRunnable` returns the single focused task, this returns the whole set an orchestrator can
 * fan out concurrently — safe now that each mutation holds a per-spec lock (§5.3).
 */
export function runnableFrontier(tasks: DagTask[]): DagTask[] {
  const map = byId(tasks);
  return tasks
    .filter((t) => isRunnable(t, map))
    .sort((a, b) => a.wave - b.wave || ordinal(a.id) - ordinal(b.id));
}

export interface WaveRow {
  wave: number;
  tasks: DagTask[];
}

/** Group tasks by wave (ascending), tasks sorted by id ordinal within wave. */
export function groupWaves(tasks: DagTask[]): WaveRow[] {
  const waves = [...new Set(tasks.map((t) => t.wave))].sort((a, b) => a - b);
  return waves.map((wave) => ({
    wave,
    tasks: tasks.filter((t) => t.wave === wave).sort((a, b) => ordinal(a.id) - ordinal(b.id)),
  }));
}

/**
 * Critical path: longest dependency chain (by task count). Returns ordered ids.
 * Ties broken by lowest id ordinal.
 */
export function criticalPath(tasks: DagTask[]): string[] {
  const map = byId(tasks);
  const memo = new Map<string, string[]>();
  const longest = (id: string, seen: Set<string>): string[] => {
    if (memo.has(id)) return memo.get(id)!;
    if (seen.has(id)) return [id]; // cycle guard
    seen.add(id);
    const node = map.get(id);
    let best: string[] = [];
    if (node) {
      for (const d of node.depends) {
        if (!map.has(d)) continue;
        const path = longest(d, seen);
        if (path.length > best.length) best = path;
      }
    }
    seen.delete(id);
    const result = [...best, id];
    memo.set(id, result);
    return result;
  };
  let best: string[] = [];
  for (const t of tasks) {
    const p = longest(t.id, new Set());
    if (p.length > best.length) best = p;
  }
  return best;
}
