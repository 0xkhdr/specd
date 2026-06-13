// `specd status [<slug>] [--json]` — the durable ledger / "where am I" board (SPEC §5.2).
import { requireSpecdRoot } from "../core/paths.js";
import { listSpecs, loadSpec } from "../core/specFiles.js";
import { loadState } from "../core/state.js";
import { counts, nextSummary, waveGraph } from "../core/render.js";
import { nextRunnable } from "../core/dag.js";
import { dagTasks } from "../core/render.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const json = args.flags.json === true;
  const slug = args.pos[0];

  if (!slug) {
    const specs = listSpecs(root);
    if (json) {
      const rows = specs.map((s) => {
        const st = loadState(root, s)!;
        return { spec: s, status: st.status, phase: st.phase, gate: st.gate, ...counts(st) };
      });
      console.log(JSON.stringify(rows, null, 2));
      return 0;
    }
    if (!specs.length) { console.log("no specs yet. Run `specd new <slug>`."); return 0; }
    for (const s of specs) {
      const st = loadState(root, s)!;
      const c = counts(st);
      const gate = st.gate === "none" ? "" : `  ⛔ ${st.gate}`;
      console.log(`${s}  [${st.status}]  ${c.complete}/${c.total} done · next: ${nextSummary(st)}${gate}`);
    }
    return 0;
  }

  const { state } = loadSpec(root, slug);
  const c = counts(state);
  if (json) {
    console.log(JSON.stringify({
      ...state,
      counts: c,
      next: nextRunnable(dagTasks(state)),
    }, null, 2));
    return 0;
  }

  console.log(`# ${state.title} (${state.spec})`);
  console.log(`status: ${state.status} · phase: ${state.phase} · gate: ${state.gate} · turn: ${state.turn}`);
  console.log(`tasks: ${c.complete} complete · ${c.running} running · ${c.pending} pending · ${c.blocked} blocked · ${c.total} total`);
  console.log("");
  console.log(waveGraph(state));
  if (state.blockers.length) {
    console.log("");
    console.log("Blockers:");
    for (const b of state.blockers) console.log(`  ⚠ ${b.task}: ${b.reason} (since ${b.since})`);
  }
  console.log("");
  console.log(`Next: ${nextSummary(state)}`);
  if (state.gate !== "none") console.log(`\n⛔ GATE: ${state.gate} — stop and get approval.`);
  return 0;
}
