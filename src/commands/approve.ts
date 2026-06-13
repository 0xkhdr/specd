// `specd approve <slug> [--json]` — the human/orchestrator approval primitive (SPEC §5.2, §7.6).
// Two jobs, dispatched on state: (1) clear a `midreq`-set awaiting-approval gate so work resumes;
// (2) advance the planning ratchet requirements → design → tasks → executing, but only once the
// gate for the artifact that phase produced is green. This is the inverse of `midreq` and the
// only sanctioned way to clear the gate or advance a planning phase (state.json stays CLI-owned).
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec, readArtifact, loadConfig } from "../core/specFiles.js";
import { withSpecLock } from "../core/lock.js";
import { saveState } from "../core/state.js";
import { PLANNING_ADVANCE, phaseReadiness, phaseForStatus } from "../core/phases.js";
import { acceptanceGaps } from "../core/render.js";
import { usageError, gateError } from "../core/exit.js";
import type { Args } from "../cli.js";

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError("usage: specd approve <slug> [--json]");
  const json = args.flags.json === true;
  // Lock across load→advance→save so a concurrent approve/task can't race the planning ratchet.
  return withSpecLock(root, slug, () => {
  const { state, doc } = loadSpec(root, slug);

  // Case 1: a midreq raised the gate — clear it, leave status/phase where execution paused.
  if (state.gate === "awaiting-approval") {
    state.gate = "none";
    saveState(root, slug, state);
    if (json) { console.log(JSON.stringify({ ok: true, action: "gate-cleared", status: state.status, phase: state.phase }, null, 2)); return 0; }
    console.log(`approve: gate cleared — resume at status '${state.status}' (phase ${state.phase}).`);
    return 0;
  }

  // Case 2: spec-level VERIFY → REFLECT. Every task is complete (status `verifying`); this is the
  // human acceptance of the spec-level verification. Advance to `complete`/reflect.
  if (state.status === "verifying") {
    // G5: when the acceptance gate is `required`, refuse while any requirement lacks a passing
    // criterion or any criterion is recorded as fail — the per-criterion proof must be complete.
    if (loadConfig(root).gates.acceptance === "required") {
      const { unmet, failed } = acceptanceGaps(state, readArtifact(root, slug, "requirements.md"));
      if (unmet.length || failed.length) {
        const problems = [
          ...unmet.map((n) => `requirement ${n}: no passing acceptance criterion`),
          ...failed.map((k) => `criterion ${k}: recorded as fail`),
        ];
        if (json) { console.log(JSON.stringify({ ok: false, action: "blocked", status: state.status, problems }, null, 2)); return 1; }
        for (const p of problems) console.error(`fail  ${p}`);
        console.error(`\n✗ cannot approve verification — ${problems.length} unmet acceptance criterion/criteria. Record with \`specd verify ${slug} --criterion <r>.<n> --status pass --evidence "..."\`.`);
        return 1;
      }
    }
    const from = state.status;
    state.status = "complete";
    state.phase = phaseForStatus("complete");
    saveState(root, slug, state);
    if (json) { console.log(JSON.stringify({ ok: true, action: "verified", from, status: state.status, phase: state.phase }, null, 2)); return 0; }
    console.log(`approve: verification accepted → status 'complete' (phase ${state.phase}).`);
    return 0;
  }

  // Case 3: advance the planning ratchet.
  const advance = PLANNING_ADVANCE[state.status];
  if (!advance) {
    throw gateError(`approve: nothing to approve — spec '${slug}' is '${state.status}'.`);
  }

  const problems = phaseReadiness(
    state.status,
    readArtifact(root, slug, "requirements.md"),
    readArtifact(root, slug, "design.md"),
    doc,
  );
  if (problems.length) {
    if (json) { console.log(JSON.stringify({ ok: false, action: "blocked", status: state.status, problems }, null, 2)); return 1; }
    for (const p of problems) console.error(`fail  ${p}`);
    console.error(`\n✗ cannot approve '${state.status}' — ${problems.length} gate violation(s). Fix and retry.`);
    return 1;
  }

  const from = state.status;
  state.status = advance.status;
  state.phase = advance.phase;
  saveState(root, slug, state);

  if (json) { console.log(JSON.stringify({ ok: true, action: "advanced", from, status: state.status, phase: state.phase }, null, 2)); return 0; }
  console.log(`approve: '${from}' approved → status '${state.status}' (phase ${state.phase}).`);
  return 0;
  });
}
