// Accessors for the six spec artifacts + config, and tasks.md <-> state.json reconciliation.
import { existsSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { readOrNull } from "./io.js";
import { configPath, rolesDir, specDir } from "./paths.js";
import { notFoundError } from "./exit.js";
import type { State, TaskState } from "./state.js";
import { parseDepends, parseRequirements, parseTasks, type ParsedTasks } from "./tasksParser.js";
import { loadState, saveState } from "./state.js";
import { withSpecLock } from "./lock.js";

export interface Config {
  version: number;
  defaultVerify: string;
  report: { format: "md" | "html"; autoRefreshSeconds: number };
  roles: { subagentMode: "inline" | "delegate" };
  promotionThreshold: number;
  /**
   * VERIFY-time gate policy (G5). `traceability` promotes forward-traceability (a requirement with
   * no covering task) from a warning to a hard violation. `acceptance` requires a per-criterion
   * proof map before `approve` advances `verifying → complete`. Both default to the legacy
   * behavior so existing repos are unchanged.
   */
  gates: { traceability: "warn" | "error"; acceptance: "off" | "required" };
}

export const DEFAULT_CONFIG: Config = {
  version: 1,
  defaultVerify: "npm test",
  report: { format: "md", autoRefreshSeconds: 0 },
  roles: { subagentMode: "inline" },
  promotionThreshold: 3,
  gates: { traceability: "warn", acceptance: "off" },
};

export function loadConfig(root: string): Config {
  const raw = readOrNull(configPath(root));
  if (!raw) return DEFAULT_CONFIG;
  const partial = JSON.parse(raw) as Partial<Config>;
  // Deep-merge nested objects so a partial `gates`/`report`/`roles` keeps the other defaults.
  return {
    ...DEFAULT_CONFIG,
    ...partial,
    report: { ...DEFAULT_CONFIG.report, ...(partial.report ?? {}) },
    roles: { ...DEFAULT_CONFIG.roles, ...(partial.roles ?? {}) },
    gates: { ...DEFAULT_CONFIG.gates, ...(partial.gates ?? {}) },
  };
}

export const ARTIFACTS = [
  "requirements.md",
  "design.md",
  "tasks.md",
  "decisions.md",
  "memory.md",
  "mid-requirements.md",
] as const;
export type Artifact = (typeof ARTIFACTS)[number];

export const artifactPath = (root: string, slug: string, name: Artifact) => join(specDir(root, slug), name);

export function readArtifact(root: string, slug: string, name: Artifact): string | null {
  return readOrNull(artifactPath(root, slug, name));
}

/** Read a role prompt body from `.specd/roles/<role>.md`, or null if absent. */
export function readRole(root: string, role: string): string | null {
  return readOrNull(join(rolesDir(root), `${role}.md`));
}

export function specExists(root: string, slug: string): boolean {
  return existsSync(join(specDir(root, slug), "state.json"));
}

export function requireSpec(root: string, slug: string): void {
  if (!specExists(root, slug)) {
    throw notFoundError(`spec '${slug}' not found under .specd/specs/`);
  }
}

/** List spec slugs (dirs with a state.json). */
export function listSpecs(root: string): string[] {
  const dir = join(root, ".specd", "specs");
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { withFileTypes: true })
    .filter((d) => d.isDirectory() && existsSync(join(dir, d.name, "state.json")))
    .map((d) => d.name)
    .sort();
}

/**
 * Reconcile state.json task entries against the structure declared in tasks.md.
 * Structural fields (title/role/wave/depends/requirements) come from tasks.md;
 * status/evidence/timestamps/blocker are preserved from state.json (status source of truth).
 * Tasks absent from tasks.md are dropped. Mutates and returns `state`.
 */
export function reconcile(state: State, doc: ParsedTasks): State {
  const next: Record<string, TaskState> = {};
  for (const t of doc.tasks) {
    const prev = state.tasks[t.id];
    const depends = parseDepends(t.meta.depends ?? "");
    const requirements = "requirements" in t.meta ? parseRequirements(t.meta.requirements) : (prev?.requirements ?? []);
    next[t.id] = {
      id: t.id,
      title: t.title,
      role: t.meta.role ?? prev?.role ?? "builder",
      wave: t.wave,
      depends,
      requirements,
      status: prev?.status ?? "pending",
      startedAt: prev?.startedAt ?? null,
      finishedAt: prev?.finishedAt ?? null,
      evidence: prev?.evidence ?? null,
      verification: prev?.verification,
      blocker: prev?.blocker ?? null,
    };
  }
  state.tasks = next;
  // prune blockers for tasks that no longer exist
  state.blockers = state.blockers.filter((b) => b.task in next);
  return state;
}

/** Parse tasks.md for a spec (empty doc if file missing). May throw SpecdError(1) on bad format. */
export function parseTasksMd(root: string, slug: string): ParsedTasks {
  const raw = readArtifact(root, slug, "tasks.md");
  if (raw === null || raw.trim() === "") return { title: slug, tasks: [] };
  return parseTasks(raw);
}

/**
 * Load state.json, reconcile it against tasks.md, persist if structure changed, and return both.
 * This is the canonical "load a spec for read/mutate" entry point.
 */
export function loadSpec(root: string, slug: string): { state: State; doc: ParsedTasks } {
  requireSpec(root, slug); // before locking: the lockfile lives inside the spec dir, which must exist
  return withSpecLock(root, slug, () => {
    const state = loadState(root, slug)!;
    const doc = parseTasksMd(root, slug);
    const before = JSON.stringify(state.tasks);
    reconcile(state, doc);
    if (JSON.stringify(state.tasks) !== before) saveState(root, slug, state);
    return { state, doc };
  });
}
