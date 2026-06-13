// `specd program [status] [--json]` — the cross-spec / program view (G4). Renders a spec-level DAG
// and answers the orchestrator's question "what's runnable across the whole program right now?".
// `specd program link <spec> --on <dep>` / `unlink <spec> --on <dep>` edit the central edge manifest.
import { requireSpecdRoot } from "../core/paths.js";
import { specExists } from "../core/specFiles.js";
import { usageError, gateError, notFoundError } from "../core/exit.js";
import { buildProgram, loadProgram, saveProgram } from "../core/program.js";
import { runnableFrontier, nextRunnable, groupWaves, criticalPath } from "../core/dag.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const sub = args.pos[0];
  if (sub === "link" || sub === "unlink") return mutate(root, sub, args);
  return render(root, args.flags.json === true); // undefined or "status"
}

/** Add or remove a cross-spec edge in `.specd/program.json`. */
function mutate(root: string, sub: "link" | "unlink", args: Args): number {
  const spec = args.pos[1];
  const dep = typeof args.flags.on === "string" ? args.flags.on : undefined;
  if (!spec || !dep) throw usageError(`usage: specd program ${sub} <spec> --on <dep>`);
  if (spec === dep) throw usageError("a spec cannot depend on itself");
  if (!specExists(root, spec)) throw notFoundError(`spec '${spec}' not found under .specd/specs/`);
  if (!specExists(root, dep)) throw notFoundError(`spec '${dep}' not found under .specd/specs/`);

  const manifest = loadProgram(root);
  const current = new Set(manifest.dependsOn[spec] ?? []);

  if (sub === "link") {
    current.add(dep);
    manifest.dependsOn[spec] = [...current];
    const cycle = buildProgram(root, manifest).cycle;
    if (cycle) throw gateError(`linking ${spec} → ${dep} would create a cycle: ${cycle.join(" → ")}`);
    saveProgram(root, manifest);
    console.log(`linked: ${spec} now depends on ${dep}`);
  } else {
    current.delete(dep);
    manifest.dependsOn[spec] = [...current];
    saveProgram(root, manifest);
    console.log(`unlinked: ${spec} no longer depends on ${dep}`);
  }
  return 0;
}

/** Render the program-level DAG, runnable frontier, and any cycle/orphan problems. */
function render(root: string, json: boolean): number {
  const { specs, dag, orphans, cycle } = buildProgram(root);
  const frontier = runnableFrontier(dag).map((d) => d.id);
  const next = nextRunnable(dag);

  if (json) {
    console.log(JSON.stringify({
      kind: "program",
      count: specs.length,
      specs: specs.map((s) => ({ ...s, runnable: frontier.includes(s.slug) })),
      frontier,
      waves: groupWaves(dag).map((w) => ({ wave: w.wave, specs: w.tasks.map((t) => t.id) })),
      criticalPath: criticalPath(dag),
      next,
      cycle,
      orphans,
    }, null, 2));
    return cycle ? 1 : 0;
  }

  if (!specs.length) { console.log("no specs yet. Run `specd new <slug>`."); return 0; }

  console.log(`# Program — ${specs.length} spec(s)`);
  console.log("legend: ✓ complete · ▶ runnable · ✗ blocked · · waiting");
  console.log("");
  for (const w of groupWaves(dag)) {
    console.log(`Wave ${w.wave}:`);
    for (const t of w.tasks) {
      const s = specs.find((x) => x.slug === t.id)!;
      const mark = s.complete ? "✓" : frontier.includes(s.slug) ? "▶" : s.status === "blocked" ? "✗" : "·";
      const deps = s.dependsOn.length ? `  ← ${s.dependsOn.join(", ")}` : "";
      console.log(`  ${mark} ${s.slug}  [${s.status}]${deps}`);
    }
  }
  console.log("");

  if (frontier.length) {
    console.log(`Runnable now: ${frontier.join(", ")}`);
  } else if (next.kind === "all-complete") {
    console.log("✓ all specs complete.");
  } else if (next.kind === "all-blocked") {
    console.log(`⚠ all remaining specs blocked: ${next.blocked.join(", ")}`);
  } else if (next.kind === "waiting") {
    console.log(`… waiting on incomplete specs: ${next.blocking.join(", ")}`);
  }

  const cp = criticalPath(dag);
  if (cp.length > 1) console.log(`Critical path: ${cp.join(" → ")}`);

  if (orphans.length) {
    console.log("");
    for (const o of orphans) console.log(`⚠ ${o.spec} depends on unknown spec '${o.dep}'`);
  }
  if (cycle) {
    console.log("");
    console.log(`⛔ dependency cycle: ${cycle.join(" → ")}`);
    return 1;
  }
  return 0;
}
