// Planning-phase ratchet + per-phase readiness gates (SPEC §5.2, §10).
// The Kiro spec workflow gates advancement requirements → design → tasks → executing; each
// step must pass the gate for the artifact it produces before `specd approve` lets it advance.
import { stripHtmlComments } from "./md.js";
import { lintEars } from "./ears.js";
import { parseDepends, type ParsedTasks } from "./tasksParser.js";
import { detectCycle, orphanDeps, waveViolations, type DagTask } from "./dag.js";
import type { SpecStatus, Phase } from "./state.js";

export interface Violation { gate: string; location: string; message: string; }

export const DESIGN_SECTIONS = [
  "Overview", "Architecture", "Components and interfaces", "Data models",
  "Error handling", "Verification strategy", "Risks and open questions",
];

/** Validate design.md has every mandatory section, each non-empty and TODO-free. */
export function designGate(md: string | null): Violation[] {
  const v: Violation[] = [];
  if (!md) return [{ gate: "design", location: "design.md", message: "design.md missing or empty" }];
  const lines = stripHtmlComments(md).split("\n");
  for (const sec of DESIGN_SECTIONS) {
    const idx = lines.findIndex((l) => new RegExp(`^##\\s+${sec.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}\\s*$`, "i").test(l));
    if (idx === -1) { v.push({ gate: "design", location: "design.md", message: `missing section: ## ${sec}` }); continue; }
    const body: string[] = [];
    for (let i = idx + 1; i < lines.length && !/^##\s+/.test(lines[i]); i++) body.push(lines[i]);
    const text = body.join("\n").trim();
    if (!text) v.push({ gate: "design", location: `design.md:${idx + 1}`, message: `section '${sec}' is empty` });
    else if (/\bTODO\b/.test(text)) v.push({ gate: "design", location: `design.md:${idx + 1}`, message: `section '${sec}' still contains a TODO marker` });
  }
  return v;
}

/**
 * The reasoning phase that corresponds to a spec status — the single source of truth tying the
 * six-phase architecture (reasoning.md) to persisted state. `perceive` is the pre-spec cognitive
 * beat (no spec exists yet to hold it) and is therefore never a persisted value; every other phase
 * is reachable: analyze (requirements) → plan (design+tasks) → execute → verify → reflect.
 */
export function phaseForStatus(status: SpecStatus): Phase {
  switch (status) {
    case "requirements": return "analyze";
    case "design": return "plan";
    case "tasks": return "plan";
    case "executing": return "execute";
    case "blocked": return "execute";
    case "verifying": return "verify";
    case "complete": return "reflect";
  }
}

/** The status a planning status advances to on approval. Undefined once executing. */
export const PLANNING_ADVANCE: Partial<Record<SpecStatus, { status: SpecStatus; phase: Phase }>> = {
  requirements: { status: "design", phase: phaseForStatus("design") },
  design: { status: "tasks", phase: phaseForStatus("tasks") },
  tasks: { status: "executing", phase: phaseForStatus("executing") },
};

/**
 * Problems blocking approval of the *current* planning status — only the gate for the artifact
 * that phase produces (so you can approve requirements while design/tasks are still stubs).
 * Returns located messages; empty means the phase is approvable.
 */
export function phaseReadiness(
  status: SpecStatus,
  reqMd: string | null,
  designMd: string | null,
  doc: ParsedTasks,
): string[] {
  if (status === "requirements") {
    if (!reqMd) return ["requirements.md missing or empty"];
    return lintEars(reqMd).map((i) => `requirements.md:${i.line}: ${i.message}`);
  }
  if (status === "design") {
    return designGate(designMd).map((v) => `${v.location}: ${v.message}`);
  }
  if (status === "tasks") {
    if (doc.tasks.length === 0) return ["tasks.md: no tasks defined"];
    const dag: DagTask[] = doc.tasks.map((t) => ({
      id: t.id, wave: t.wave, depends: parseDepends(t.meta.depends ?? ""), status: "pending",
    }));
    const out: string[] = [];
    for (const o of orphanDeps(dag)) out.push(`tasks.md: ${o.task} depends on missing task '${o.dep}'`);
    const cyc = detectCycle(dag);
    if (cyc) out.push(`tasks.md: dependency cycle: ${cyc.join(" → ")}`);
    for (const w of waveViolations(dag)) out.push(`tasks.md: ${w.task} depends on later-wave task '${w.dep}'`);
    return out;
  }
  return [];
}
