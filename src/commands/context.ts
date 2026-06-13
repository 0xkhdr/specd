// `specd context <slug> [--json]` — the context-engineering primitive. Emits the MINIMAL,
// phase-scoped briefing an agent needs right now: orientation + exactly which files to load for the
// current phase + the single next action. The opposite of "read all steering every turn" — it
// curates context to the phase so the agent's window holds signal, not the whole repo of docs.
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec, loadConfig, readArtifact } from "../core/specFiles.js";
import { usageError } from "../core/exit.js";
import { counts, nextSummary, blockerLines, latestMidreq, uncoveredRequirements } from "../core/render.js";
import type { SpecStatus, State } from "../core/state.js";
import type { Args } from "../cli.js";

// Always-on steering (the thinking architecture + the lifecycle). Everything else is phase-scoped.
const BASE_STEERING = [".specd/steering/reasoning.md", ".specd/steering/workflow.md"];

interface Brief {
  phaseLabel: string;
  purpose: string;
  load: string[]; // minimal files to pull into context now
  focus: string;
  next: string;
}

function brief(state: State, slug: string, defaultVerify: string): Brief {
  const sp = (...f: string[]) => f.map((n) => `.specd/specs/${slug}/${n}`);
  const map: Record<SpecStatus, Brief> = {
    requirements: {
      phaseLabel: "ANALYZE", purpose: "Pin down what must be true, in EARS.",
      load: [...sp("requirements.md"), ".specd/steering/product.md"],
      focus: "Write/refine requirements.md acceptance criteria in EARS form.",
      next: `specd check ${slug}  →  specd approve ${slug}`,
    },
    design: {
      phaseLabel: "PLAN (design)", purpose: "Decide how the requirements get satisfied.",
      load: [...sp("requirements.md", "design.md"), ".specd/steering/tech.md", ".specd/steering/structure.md"],
      focus: "Fill every design.md section (overview…risks); no TODOs, no empty sections.",
      next: `specd check ${slug}  →  specd approve ${slug}`,
    },
    tasks: {
      phaseLabel: "PLAN (tasks)", purpose: "Decompose the design into an ordered wave DAG.",
      load: sp("design.md", "tasks.md"),
      focus: "Author tasks.md: each task carries why/role/files/contract/acceptance/verify/depends/requirements.",
      next: `specd check ${slug}  →  specd approve ${slug}`,
    },
    executing: {
      phaseLabel: "EXECUTE", purpose: "Build one task at a time, evidence-gated.",
      load: sp("tasks.md", "memory.md"),
      focus: `Run the next runnable task only: ${nextSummary(state)}`,
      next: `specd next ${slug}`,
    },
    blocked: {
      phaseLabel: "EXECUTE (blocked)", purpose: "Frontier is stuck — surface and resolve.",
      load: sp("tasks.md"),
      focus: state.blockers.length
        ? "Resolve the blockers listed under SIGNALS."
        : "All remaining tasks blocked.",
      next: `specd status ${slug}`,
    },
    verifying: {
      phaseLabel: "VERIFY", purpose: "All tasks done — confirm the spec actually works.",
      load: sp("tasks.md", "requirements.md"),
      focus: `Run the spec-level verification (config defaultVerify: \`${defaultVerify}\`) and confirm acceptance criteria hold.`,
      next: `specd approve ${slug}   (accepts verification → REFLECT)`,
    },
    complete: {
      phaseLabel: "REFLECT", purpose: "Capture what was learned; promote durable patterns.",
      load: sp("memory.md", "decisions.md"),
      focus: "Record learnings in memory.md and any deviations in decisions.md.",
      next: `specd memory ${slug} promote --key <pattern>`,
    },
  };
  return map[state.status];
}

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd context <slug> [--json]");
  const { state } = loadSpec(root, slug);
  const cfg = loadConfig(root);
  const json = args.flags.json === true;

  const b = brief(state, slug, cfg.defaultVerify);
  const c = counts(state);
  const load = [...BASE_STEERING, ...b.load];
  const gated = state.gate === "awaiting-approval";

  // Live signals folded into the briefing so one `context` call is enough to act:
  // blockers (any phase), the latest midreq (only when gated), and uncovered requirements (VERIFY).
  const reqMd = readArtifact(root, slug, "requirements.md");
  const blockers = blockerLines(state);
  const midreq = gated ? latestMidreq(root, slug) : null;
  const uncovered = state.status === "verifying" ? uncoveredRequirements(state, reqMd) : [];

  if (json) {
    console.log(JSON.stringify({
      spec: slug, title: state.title, status: state.status, phase: state.phase,
      gate: state.gate, turn: state.turn, counts: c,
      phaseLabel: b.phaseLabel, purpose: b.purpose, load,
      focus: gated ? "GATE awaiting-approval — present the revised plan, do not hand out work." : b.focus,
      next: gated ? `specd approve ${slug}` : b.next,
      signals: { blockers, latestMidreq: midreq, uncoveredRequirements: uncovered },
    }, null, 2));
    return 0;
  }

  console.log(`=== CONTEXT: ${slug} ===`);
  console.log(`${state.title} · status ${state.status} · phase ${state.phase} · turn ${state.turn}`);
  console.log(`tasks: ${c.complete}/${c.total} done · next: ${nextSummary(state)}`);
  console.log("");
  console.log(`PHASE ${b.phaseLabel} — ${b.purpose}`);
  console.log("");
  console.log("LOAD NOW (minimal — don't dump the rest):");
  for (const f of load) console.log(`  - ${f}`);
  console.log("");

  // SIGNALS: only what applies right now — never a dump. Omitted entirely when nothing is live.
  const signals: string[] = [];
  for (const bl of blockers) signals.push(`! blocker ${bl}`);
  if (uncovered.length) signals.push(`! uncovered requirements (no covering task): ${uncovered.join(", ")}`);
  if (signals.length) {
    console.log("SIGNALS:");
    for (const s of signals) console.log(`  ${s}`);
    console.log("");
  }

  if (gated) {
    console.log("⛔ GATE awaiting-approval — present the revised plan; work is frozen.");
    if (midreq) console.log(`   ↳ midreq Turn ${midreq.turn} (${midreq.impact}): "${midreq.input}"`);
    console.log(`NEXT: specd approve ${slug}`);
    return 0;
  }
  console.log(`FOCUS: ${b.focus}`);
  console.log(`NEXT:  ${b.next}`);
  return 0;
}
