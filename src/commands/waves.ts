// `specd waves <slug> [--json]` — the wave DAG, critical path, blockers (SPEC §5.2, §7.3.3).
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec } from "../core/specFiles.js";
import { usageError } from "../core/exit.js";
import { criticalPath, groupWaves } from "../core/dag.js";
import { dagTasks, waveGraph } from "../core/render.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd waves <slug> [--json]");
  const { state } = loadSpec(root, slug);

  if (args.flags.json === true) {
    const tasks = dagTasks(state);
    const waves = groupWaves(tasks).map((r) => ({
      wave: r.wave,
      tasks: r.tasks.map((t) => ({ id: t.id, status: t.status, depends: t.depends })),
    }));
    console.log(JSON.stringify({
      waves,
      criticalPath: criticalPath(tasks),
      blockers: state.blockers,
    }, null, 2));
    return 0;
  }

  console.log(waveGraph(state));
  if (state.blockers.length) {
    console.log("\nBlockers gating downstream waves:");
    for (const b of state.blockers) console.log(`  ⚠ ${b.task}: ${b.reason}`);
  }
  return 0;
}
