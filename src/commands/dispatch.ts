// `specd dispatch <slug> [--json]` — emit ready-to-run dispatch packets for the runnable frontier (G2).
// Where `next --all` lists the frontier, `dispatch --json` hands an orchestrator everything a fresh
// subagent needs with zero assembly: resolved role prompt + contract + files + acceptance + verify +
// the exact completion command. The whole "fan the frontier out to parallel subagents" payload.
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec, readRole } from "../core/specFiles.js";
import { usageError } from "../core/exit.js";
import { runnableFrontier, nextRunnable } from "../core/dag.js";
import { dagTasks } from "../core/render.js";
import { findTask } from "../core/tasksParser.js";
import type { Args } from "../cli.js";

interface DispatchPacket {
  id: string;
  wave: number;
  role: string;
  rolePrompt: string;
  title: string;
  why: string;
  contract: string;
  files: string;
  acceptance: string;
  verify: string;
  depends: string[];
  requirements: number[];
  completion: string;
}

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd dispatch <slug> [--json]");
  const { state, doc } = loadSpec(root, slug);
  const json = args.flags.json === true;

  // Same gate as `next`: a midreq awaiting-approval halts dispatch until `approve`.
  if (state.gate === "awaiting-approval" && args.flags.force !== true) {
    if (json) { console.log(JSON.stringify({ kind: "gated", gate: state.gate }, null, 2)); return 1; }
    console.error(`⛔ gate awaiting-approval — present the revised plan, then \`specd approve ${slug}\` (override: --force).`);
    return 1;
  }

  const frontier = runnableFrontier(dagTasks(state));

  // Empty frontier: classify the reason exactly like `next` so an orchestrator can branch on it.
  if (frontier.length === 0) {
    const r = nextRunnable(dagTasks(state));
    if (json) {
      console.log(JSON.stringify({ kind: "frontier", count: 0, reason: r.kind, packets: [] }, null, 2));
      return 0;
    }
    if (r.kind === "all-complete") console.log("✓ all tasks complete — nothing to dispatch.");
    else if (r.kind === "all-blocked") console.log(`⚠ all remaining tasks blocked: ${r.blocked.join(", ")}`);
    else if (r.kind === "waiting") console.log(`… waiting — frontier gated by incomplete deps: ${r.blocking.join(", ")}`);
    return 0;
  }

  // Load each distinct role body once.
  const roleCache = new Map<string, string>();
  const rolePromptFor = (role: string): string => {
    if (!roleCache.has(role)) roleCache.set(role, readRole(root, role) ?? "");
    return roleCache.get(role)!;
  };

  const packets: DispatchPacket[] = frontier.map((f) => {
    const t = findTask(doc, f.id);
    const ts = state.tasks[f.id];
    const role = t?.meta.role ?? ts.role;
    const m = t?.meta;
    return {
      id: f.id,
      wave: f.wave,
      role,
      rolePrompt: rolePromptFor(role),
      title: t?.title ?? ts.title,
      why: m?.why ?? "",
      contract: m?.contract ?? "",
      files: m?.files ?? "",
      acceptance: m?.acceptance ?? "",
      verify: m?.verify ?? "",
      depends: ts.depends,
      requirements: ts.requirements,
      completion: `specd task ${slug} ${f.id} --status complete --evidence "<proof>"`,
    };
  });

  if (json) {
    console.log(JSON.stringify({ kind: "frontier", count: packets.length, packets }, null, 2));
    return 0;
  }

  // Text mode: a compact dispatch summary; the full payload lives in --json.
  console.log(`=== DISPATCH FRONTIER (${packets.length}) — fan out to parallel subagents ===`);
  for (const p of packets) {
    console.log(`  ${p.id}  [wave ${p.wave}]  ${p.title}  (${p.role})`);
    console.log(`      verify: ${p.verify || "—"}`);
    console.log(`      done:   ${p.completion}`);
  }
  console.log("==============================");
  console.log(`Full packets (role prompt + contract + files + acceptance): specd dispatch ${slug} --json`);
  return 0;
}
