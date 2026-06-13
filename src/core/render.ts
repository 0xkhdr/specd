// Shared deterministic text renderers used by status, waves, report, context, and check.
import { criticalPath, groupWaves, nextRunnable, type DagTask } from "./dag.js";
import type { State, TaskStatus } from "./state.js";
import { readArtifact } from "./specFiles.js";
import { stripHtmlComments } from "./md.js";

export const GLYPH: Record<TaskStatus, string> = {
  complete: "✓",
  running: "◐",
  pending: "○",
  blocked: "⚠",
};

export interface Counts {
  pending: number;
  running: number;
  complete: number;
  blocked: number;
  total: number;
}

export function counts(state: State): Counts {
  const c: Counts = { pending: 0, running: 0, complete: 0, blocked: 0, total: 0 };
  for (const t of Object.values(state.tasks)) {
    c[t.status]++;
    c.total++;
  }
  return c;
}

export function dagTasks(state: State): DagTask[] {
  return Object.values(state.tasks).map((t) => ({
    id: t.id,
    wave: t.wave,
    depends: t.depends,
    status: t.status,
  }));
}

/** Render the §7.3.3 wave graph: tasks grouped by wave with status glyphs + critical path. */
export function waveGraph(state: State): string {
  const tasks = dagTasks(state);
  if (tasks.length === 0) return "(no tasks yet)";
  const lines: string[] = [];
  for (const row of groupWaves(tasks)) {
    lines.push(`Wave ${row.wave}`);
    for (const t of row.tasks) {
      const full = state.tasks[t.id];
      let line = `  ${GLYPH[t.status]} ${t.id}  ${full.title}`;
      if (t.status === "blocked" && full.blocker) line += `  (blocked: ${full.blocker})`;
      lines.push(line);
    }
  }
  const cp = criticalPath(tasks);
  if (cp.length) lines.push("", `Critical path: ${cp.join(" → ")}`);
  return lines.join("\n");
}

/** One-line human summary of next-runnable result. */
export function nextSummary(state: State): string {
  const r = nextRunnable(dagTasks(state));
  switch (r.kind) {
    case "task": return `${r.id} — ${state.tasks[r.id].title}`;
    case "all-complete": return "all tasks complete";
    case "all-blocked": return `all remaining blocked: ${r.blocked.join(", ")}`;
    case "waiting": return `waiting on: ${r.blocking.join(", ")}`;
  }
}

// --- Live-signal renderers for `specd context` (G3) --------------------------------------------

/** Blocker one-liners, e.g. "T4: api key missing". Empty array if none recorded. */
export function blockerLines(state: State): string[] {
  return state.blockers.map((b) => `${b.task}: ${b.reason}`);
}

export interface MidreqSummary { turn: number; impact: string; input: string; }

/**
 * Parse the most recent `## Turn N — … — impact: X` block from mid-requirements.md so `context`
 * can show *what* changed when a midreq raised the awaiting-approval gate. Returns null if the
 * ledger is absent or has no parseable entry. Deterministic string parse — no deps.
 */
export function latestMidreq(root: string, slug: string): MidreqSummary | null {
  const raw = readArtifact(root, slug, "mid-requirements.md");
  if (!raw) return null;
  const idx = raw.lastIndexOf("## Turn ");
  if (idx === -1) return null;
  const block = raw.slice(idx);
  const header = block.match(/^##\s+Turn\s+(\d+).*?impact:\s*(\w+)/i);
  if (!header) return null;
  const im = block.match(/\*\*User input \(verbatim\):\*\*\s*"?(.*?)"?\s*$/im);
  let input = im ? im[1].trim() : "";
  if (input.length > 120) input = input.slice(0, 117) + "...";
  return { turn: parseInt(header[1], 10), impact: header[2].toLowerCase(), input };
}

/** Requirement numbers declared in requirements.md (`## Requirement N`). Shared with check gate 7. */
export function requirementNumbers(reqMd: string): Set<number> {
  return new Set(
    [...stripHtmlComments(reqMd).matchAll(/^##\s+Requirement\s+(\d+)/gim)].map((m) => parseInt(m[1], 10)),
  );
}

/**
 * Requirement numbers defined in requirements.md but referenced by zero tasks (state-side
 * coverage). Sorted ascending. Used by `context` at the VERIFY gate to list what still needs
 * confirming. Empty if reqMd is absent or every requirement is covered.
 */
export function uncoveredRequirements(state: State, reqMd: string | null): number[] {
  if (!reqMd) return [];
  const referenced = new Set<number>();
  for (const t of Object.values(state.tasks)) for (const n of t.requirements) referenced.add(n);
  return [...requirementNumbers(reqMd)].filter((n) => !referenced.has(n)).sort((a, b) => a - b);
}

/**
 * Evaluate the G5 acceptance ledger against the requirements: `unmet` is every defined requirement
 * with no passing criterion, `failed` is every criterion key recorded as `fail`. Both sorted.
 * Shared by `approve` (the VERIFY refusal when `gates.acceptance: required`) and any reporting.
 */
export function acceptanceGaps(state: State, reqMd: string | null): { unmet: number[]; failed: string[] } {
  const reqs = reqMd ? [...requirementNumbers(reqMd)] : [];
  const map = state.acceptance ?? {};
  const passed = new Set<number>();
  const failed: string[] = [];
  for (const [key, rec] of Object.entries(map)) {
    if (rec.status === "pass") passed.add(rec.requirement);
    else failed.push(key);
  }
  return {
    unmet: reqs.filter((n) => !passed.has(n)).sort((a, b) => a - b),
    failed: failed.sort((a, b) => a.localeCompare(b, undefined, { numeric: true })),
  };
}
