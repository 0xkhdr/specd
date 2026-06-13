// `specd midreq <slug> "<input>" --impact <..>` — append feedback, bump turn, maybe gate (§5.2, §7.6).
import { requireSpecdRoot } from "../core/paths.js";
import { artifactPath, requireSpec } from "../core/specFiles.js";
import { appendFile } from "../core/io.js";
import { usageError } from "../core/exit.js";
import { loadState, saveState } from "../core/state.js";
import { withSpecLock } from "../core/lock.js";
import type { Args } from "../cli.js";

const IMPACTS = new Set(["low", "medium", "high", "critical"]);

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  const input = args.pos[1];
  if (!slug || !input) throw usageError('usage: specd midreq <slug> "<verbatim input>" --impact <low|medium|high|critical> [--interpretation ..] [--changes ..]');
  requireSpec(root, slug);

  const impact = args.flags.impact;
  if (typeof impact !== "string" || !IMPACTS.has(impact)) {
    throw usageError("--impact must be one of: low, medium, high, critical");
  }
  const interpretation = typeof args.flags.interpretation === "string" ? args.flags.interpretation : "TODO";
  const changes = typeof args.flags.changes === "string" ? args.flags.changes : "TODO";

  const gated = impact === "high" || impact === "critical";
  return withSpecLock(root, slug, () => {
    const state = loadState(root, slug)!;
    state.turn += 1;
    if (gated) state.gate = "awaiting-approval";
    saveState(root, slug, state);

    const stamp = new Date().toISOString().slice(0, 16); // YYYY-MM-DDTHH:MM
    const entry = [
      "",
      `## Turn ${state.turn} — ${stamp} — impact: ${impact}`,
      `**User input (verbatim):** "${input}"`,
      `**Interpretation:** ${interpretation}`,
      `**Impact:** ${impact}`,
      `**Changes made:** ${changes}`,
      `**Notes / open questions:** TODO`,
      "",
    ].join("\n");
    appendFile(artifactPath(root, slug, "mid-requirements.md"), entry);

    console.log(`midreq: logged Turn ${state.turn} (impact: ${impact})`);
    if (gated) console.log("⛔ gate set to awaiting-approval — stop, present the revised plan, wait for approval.");
    return 0;
  });
}
