// Program model (G4): cross-spec orchestration. A central manifest `.specd/program.json` declares
// inter-spec dependency edges; this module projects every spec as a node in a spec-level DAG and
// reuses the task-level primitives in `dag.ts` (waves / frontier / critical-path / cycle) at spec
// granularity. A spec is "complete" when its status is `complete`, "runnable" when all the specs it
// depends on are complete. Edges live in one manifest (not per-spec) so the program is reasoned
// about in a single place and each spec stays self-contained (GAPS G4 open question).
import { join } from "node:path";
import { atomicWrite, readOrNull } from "./io.js";
import { specdDir } from "./paths.js";
import { listSpecs } from "./specFiles.js";
import { loadState, type SpecStatus, type TaskStatus } from "./state.js";
import { gateError } from "./exit.js";
import { detectCycle, type DagTask } from "./dag.js";

/** program.json shape version. Bump if the manifest shape changes. */
export const PROGRAM_VERSION = 1;

export interface ProgramManifest {
  version: number;
  /** spec slug -> the slugs it depends on (must all be `complete` before it is runnable). */
  dependsOn: Record<string, string[]>;
}

export const programPath = (root: string) => join(specdDir(root), "program.json");

/** Load `.specd/program.json`, or an empty manifest if absent. De-dupes each edge list. */
export function loadProgram(root: string): ProgramManifest {
  const raw = readOrNull(programPath(root));
  if (!raw) return { version: PROGRAM_VERSION, dependsOn: {} };
  let parsed: Partial<ProgramManifest>;
  try {
    parsed = JSON.parse(raw) as Partial<ProgramManifest>;
  } catch (err) {
    throw gateError(`corrupt program.json: ${err instanceof Error ? err.message : String(err)}`);
  }
  const dependsOn: Record<string, string[]> = {};
  for (const [slug, deps] of Object.entries(parsed.dependsOn ?? {})) {
    if (Array.isArray(deps) && deps.length) dependsOn[slug] = [...new Set(deps.map(String))];
  }
  return { version: parsed.version ?? PROGRAM_VERSION, dependsOn };
}

/** Atomically persist the manifest, pruning empty edge lists and sorting for a stable diff. */
export function saveProgram(root: string, manifest: ProgramManifest): void {
  const dependsOn: Record<string, string[]> = {};
  for (const slug of Object.keys(manifest.dependsOn).sort()) {
    const deps = [...new Set(manifest.dependsOn[slug])].sort();
    if (deps.length) dependsOn[slug] = deps;
  }
  atomicWrite(programPath(root), JSON.stringify({ version: manifest.version, dependsOn }, null, 2) + "\n");
}

export interface SpecNode {
  slug: string;
  status: SpecStatus;
  dependsOn: string[];
  wave: number;
  complete: boolean;
}

export interface ProgramGraph {
  specs: SpecNode[];
  /** Each spec projected as a DagTask (id=slug) so `dag.ts` primitives apply at spec granularity. */
  dag: DagTask[];
  /** Edges pointing at a spec that does not exist (deleted or typo'd). Warn-only, like traceability. */
  orphans: { spec: string; dep: string }[];
  /** A cross-spec dependency cycle, or null. A cycle is a hard error (exit 1) for the program view. */
  cycle: string[] | null;
}

/** A complete spec maps to a complete task; a blocked spec to blocked; everything else is pending. */
const specStatusToTask = (s: SpecStatus): TaskStatus =>
  s === "complete" ? "complete" : s === "blocked" ? "blocked" : "pending";

/** Derive a wave per spec as the longest dependency chain length (roots = wave 1). */
function deriveWaves(edges: Record<string, string[]>, slugs: string[]): Map<string, number> {
  const wave = new Map<string, number>();
  const visiting = new Set<string>();
  const compute = (slug: string): number => {
    const memo = wave.get(slug);
    if (memo !== undefined) return memo;
    if (visiting.has(slug)) return 1; // cycle guard; detectCycle reports the cycle itself
    visiting.add(slug);
    let w = 1;
    for (const dep of edges[slug] ?? []) w = Math.max(w, compute(dep) + 1);
    visiting.delete(slug);
    wave.set(slug, w);
    return w;
  };
  for (const slug of slugs) compute(slug);
  return wave;
}

/**
 * Build the spec-level dependency graph from the manifest (defaults to the on-disk manifest; pass a
 * candidate manifest to test a hypothetical edge set, e.g. before committing a `link`).
 */
export function buildProgram(root: string, manifest: ProgramManifest = loadProgram(root)): ProgramGraph {
  const slugs = listSpecs(root);
  const known = new Set(slugs);

  const orphans: { spec: string; dep: string }[] = [];
  const edges: Record<string, string[]> = {};
  for (const slug of slugs) {
    const declared = manifest.dependsOn[slug] ?? [];
    for (const dep of declared) if (!known.has(dep)) orphans.push({ spec: slug, dep });
    edges[slug] = declared.filter((d) => known.has(d));
  }

  const waves = deriveWaves(edges, slugs);
  const statuses = new Map<string, SpecStatus>();
  for (const slug of slugs) statuses.set(slug, loadState(root, slug)!.status);

  const dag: DagTask[] = slugs.map((slug) => ({
    id: slug,
    wave: waves.get(slug)!,
    depends: edges[slug],
    status: specStatusToTask(statuses.get(slug)!),
  }));
  const cycle = detectCycle(dag);

  const specs: SpecNode[] = slugs
    .map((slug) => ({
      slug,
      status: statuses.get(slug)!,
      dependsOn: edges[slug],
      wave: waves.get(slug)!,
      complete: statuses.get(slug) === "complete",
    }))
    .sort((a, b) => a.wave - b.wave || a.slug.localeCompare(b.slug));

  return { specs, dag, orphans, cycle };
}
