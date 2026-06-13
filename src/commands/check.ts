// `specd check <slug> [--json]` — run all seven validation gates (SPEC §10).
import { requireSpecdRoot } from "../core/paths.js";
import { artifactPath, readArtifact, requireSpec, loadConfig } from "../core/specFiles.js";
import { loadState } from "../core/state.js";
import { lintEars } from "../core/ears.js";
import { detectCycle, orphanDeps, waveViolations, type DagTask } from "../core/dag.js";
import {
  parseTasks, parseDepends, parseRequirements, READONLY_ROLES, VALID_ROLES,
  type ParsedTasks,
} from "../core/tasksParser.js";
import { SpecdError, usageError } from "../core/exit.js";
import { requirementNumbers } from "../core/render.js";
import { designGate, type Violation } from "../core/phases.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd check <slug> [--json]");
  requireSpec(root, slug);
  const json = args.flags.json === true;

  const violations: Violation[] = [];
  const warnings: Violation[] = [];

  // Gate 1: EARS
  const reqMd = readArtifact(root, slug, "requirements.md");
  if (reqMd) for (const i of lintEars(reqMd)) violations.push({ gate: "ears", location: `requirements.md:${i.line}`, message: i.message });
  else violations.push({ gate: "ears", location: "requirements.md", message: "requirements.md missing" });

  // Gate 2: design
  violations.push(...designGate(readArtifact(root, slug, "design.md")));

  // Gate 3 + 4: parse tasks, schema, DAG
  const tasksMd = readArtifact(root, slug, "tasks.md") ?? "";
  let doc: ParsedTasks | null = null;
  try {
    doc = parseTasks(tasksMd);
  } catch (err) {
    if (err instanceof SpecdError) violations.push({ gate: "task-schema", location: "tasks.md", message: err.message });
    else throw err;
  }

  const state = loadState(root, slug)!;

  if (doc) {
    if (doc.tasks.length === 0) {
      violations.push({ gate: "task-schema", location: "tasks.md", message: "no tasks defined" });
    }
    // schema: role valid; verify command unless read-only role
    for (const t of doc.tasks) {
      const role = t.meta.role;
      if (!(VALID_ROLES as readonly string[]).includes(role)) {
        violations.push({ gate: "task-schema", location: `tasks.md:${t.line}`, message: `${t.id}: invalid role '${role}'` });
      }
      const verify = (t.meta.verify ?? "").trim();
      const isNA = verify === "" || verify.toUpperCase().startsWith("N/A");
      if (isNA && !(READONLY_ROLES as readonly string[]).includes(role)) {
        violations.push({ gate: "task-schema", location: `tasks.md:${t.line}`, message: `${t.id}: verify N/A only allowed for read-only roles (got '${role}')` });
      }
    }
    // DAG
    const dag: DagTask[] = doc.tasks.map((t) => ({
      id: t.id, wave: t.wave, depends: parseDepends(t.meta.depends ?? ""),
      status: state.tasks[t.id]?.status ?? "pending",
    }));
    for (const o of orphanDeps(dag)) violations.push({ gate: "dag", location: "tasks.md", message: `${o.task} depends on missing task '${o.dep}'` });
    const cycle = detectCycle(dag);
    if (cycle) violations.push({ gate: "dag", location: "tasks.md", message: `dependency cycle: ${cycle.join(" → ")}` });
    for (const w of waveViolations(dag)) violations.push({ gate: "dag", location: "tasks.md", message: `${w.task} depends on '${w.dep}' which is in a later wave` });

    // Gate 6: sync — checkbox/annotation vs state status
    for (const t of doc.tasks) {
      const st = state.tasks[t.id]?.status;
      if (st === undefined) continue;
      if (t.checked !== (st === "complete")) {
        violations.push({ gate: "sync", location: `tasks.md:${t.line}`, message: `${t.id}: checkbox/state drift (checkbox=${t.checked ? "[x]" : "[ ]"}, state=${st})` });
      }
      const annotBlocked = t.annotation?.kind === "blocked";
      if (annotBlocked !== (st === "blocked")) {
        violations.push({ gate: "sync", location: `tasks.md:${t.line}`, message: `${t.id}: blocked-annotation/state drift (state=${st})` });
      }
    }

    // Gate 7: traceability — bidirectional. Forward (warn): every requirement referenced by ≥1
    // task. Backward (fail): no task references a requirement number absent from requirements.md.
    const referenced = new Set<number>();
    for (const t of doc.tasks) if ("requirements" in t.meta) for (const n of parseRequirements(t.meta.requirements)) referenced.add(n);
    if (reqMd) {
      // Forward-traceability severity is configurable (G5): warn (default) or error.
      const forwardSink = loadConfig(root).gates.traceability === "error" ? violations : warnings;
      const reqNums = requirementNumbers(reqMd);
      for (const n of reqNums) if (!referenced.has(n)) forwardSink.push({ gate: "traceability", location: "requirements.md", message: `requirement ${n} not referenced by any task` });
      for (const t of doc.tasks) {
        if (!("requirements" in t.meta)) continue;
        for (const n of parseRequirements(t.meta.requirements)) {
          if (!reqNums.has(n)) violations.push({ gate: "traceability", location: `tasks.md:${t.line}`, message: `${t.id}: references requirement ${n} which is not defined in requirements.md` });
        }
      }
    }
  }

  // Gate 5: evidence (G1) — a complete task must carry evidence, and a build-role task must carry
  // a *verified* record (specd actually ran its verify line, exit 0). Read-only roles
  // (investigator/reviewer) are exempt — their verify is legitimately N/A, proof is manual.
  for (const t of Object.values(state.tasks)) {
    if (t.status !== "complete") continue;
    if (!t.evidence || !t.evidence.trim()) {
      violations.push({ gate: "evidence", location: "state.json", message: `${t.id}: complete without evidence` });
      continue;
    }
    const readonly = (READONLY_ROLES as readonly string[]).includes(t.role);
    if (!readonly && !(t.verification && t.verification.verified)) {
      violations.push({ gate: "evidence", location: "state.json", message: `${t.id}: complete without a verified record (role '${t.role}') — run \`specd verify ${slug} ${t.id}\`` });
    }
  }

  if (json) {
    console.log(JSON.stringify({ ok: violations.length === 0, violations, warnings }, null, 2));
    return violations.length === 0 ? 0 : 1;
  }

  for (const w of warnings) console.log(`warn  ${w.location}: ${w.message} (${w.gate})`);
  if (violations.length === 0) {
    console.log(`✓ check passed — all gates green for '${slug}'${warnings.length ? ` (${warnings.length} warning(s))` : ""}`);
    return 0;
  }
  for (const v of violations) console.error(`fail  ${v.location}: ${v.message} (${v.gate})`);
  console.error(`\n✗ ${violations.length} violation(s) across gates.`);
  return 1;
}
