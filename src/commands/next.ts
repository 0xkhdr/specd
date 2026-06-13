// `specd next <slug> [--json]` — the single next runnable task as a focused prompt (SPEC §5.2, §5.3).
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec } from "../core/specFiles.js";
import { usageError } from "../core/exit.js";
import { nextRunnable, runnableFrontier } from "../core/dag.js";
import { dagTasks } from "../core/render.js";
import { findTask } from "../core/tasksParser.js";
import type { Args } from "../cli.js";

/** Shape one task for JSON output, preferring tasks.md metadata, falling back to state. */
function taskJson(doc: ReturnType<typeof loadSpec>["doc"], state: ReturnType<typeof loadSpec>["state"], id: string) {
  const t = findTask(doc, id);
  const ts = state.tasks[id];
  return t
    ? { id: t.id, title: t.title, wave: t.wave, ...t.meta }
    : { id, title: ts.title, wave: ts.wave, role: ts.role, depends: ts.depends.join(", ") };
}

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd next <slug> [--json]");
  const { state, doc } = loadSpec(root, slug);
  const json = args.flags.json === true;

  // Gate enforcement: a midreq raised awaiting-approval → stop handing out work until `approve`.
  if (state.gate === "awaiting-approval" && args.flags.force !== true) {
    if (json) { console.log(JSON.stringify({ kind: "gated", gate: state.gate }, null, 2)); return 1; }
    console.error(`⛔ gate awaiting-approval — present the revised plan, then \`specd approve ${slug}\` (override: --force).`);
    return 1;
  }

  // --all: the whole runnable frontier for parallel dispatch (§5.3), not the single focused task.
  if (args.flags.all === true) {
    const frontier = runnableFrontier(dagTasks(state));
    if (json) {
      console.log(JSON.stringify({ kind: "frontier", count: frontier.length,
        tasks: frontier.map((f) => taskJson(doc, state, f.id)) }, null, 2));
      return 0;
    }
    if (frontier.length === 0) {
      // No runnable task: reuse the single-task classifier for a precise reason.
      const r = nextRunnable(dagTasks(state));
      if (r.kind === "all-complete") console.log("✓ all tasks complete — nothing runnable.");
      else if (r.kind === "all-blocked") console.log(`⚠ all remaining tasks blocked: ${r.blocked.join(", ")}`);
      else if (r.kind === "waiting") console.log(`… waiting — frontier gated by incomplete deps: ${r.blocking.join(", ")}`);
      return 0;
    }
    console.log(`=== RUNNABLE FRONTIER (${frontier.length}) — dispatch in parallel ===`);
    for (const f of frontier) {
      const t = findTask(doc, f.id);
      console.log(`  ${f.id}  [wave ${f.wave}]  ${t?.title ?? state.tasks[f.id].title}  (${t?.meta.role ?? state.tasks[f.id].role})`);
    }
    console.log("==============================");
    console.log(`Each: specd next ${slug} (focused) or complete with specd task ${slug} <id> --status complete --evidence "<proof>"`);
    return 0;
  }

  const result = nextRunnable(dagTasks(state));

  if (json) {
    if (result.kind === "task") {
      console.log(JSON.stringify({ ...result, task: taskJson(doc, state, result.id) }, null, 2));
    } else {
      console.log(JSON.stringify(result, null, 2));
    }
    return 0;
  }

  switch (result.kind) {
    case "all-complete":
      if (state.status === "verifying") {
        console.log(`✓ all tasks complete — VERIFY the spec, then \`specd approve ${slug}\` to accept and finish (→ REFLECT).`);
      } else {
        console.log("✓ all tasks complete — nothing runnable. Move to REFLECT.");
      }
      return 0;
    case "all-blocked":
      console.log(`⚠ all remaining tasks blocked: ${result.blocked.join(", ")}`);
      for (const b of state.blockers) console.log(`  ${b.task}: ${b.reason}`);
      return 0;
    case "waiting":
      console.log(`… waiting — frontier gated by incomplete deps: ${result.blocking.join(", ")}`);
      return 0;
    case "task": {
      const t = findTask(doc, result.id);
      const m = t?.meta;
      console.log(`=== NEXT TASK: ${result.id} ===`);
      console.log(`title:        ${t?.title ?? state.tasks[result.id].title}`);
      console.log(`role:         ${m?.role ?? state.tasks[result.id].role}`);
      console.log(`why:          ${m?.why ?? ""}`);
      console.log(`files:        ${m?.files ?? ""}`);
      console.log(`contract:     ${m?.contract ?? ""}`);
      console.log(`acceptance:   ${m?.acceptance ?? ""}`);
      console.log(`verify:       ${m?.verify ?? ""}`);
      console.log(`depends:      ${m?.depends ?? "—"}`);
      if (m && "requirements" in m) console.log(`requirements: ${m.requirements}`);
      console.log("==============================");
      console.log(`When done: specd task ${slug} ${result.id} --status complete --evidence "<proof>"`);
      return 0;
    }
  }
}
