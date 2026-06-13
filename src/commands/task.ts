// `specd task <slug> <id> --status <s> [--evidence|--reason]` — the evidence gate (SPEC §5.2, §3.1).
import { requireSpecdRoot } from "../core/paths.js";
import { artifactPath, loadSpec } from "../core/specFiles.js";
import { withSpecLock } from "../core/lock.js";
import { atomicWrite, readOrDefault } from "../core/io.js";
import { usageError, gateError, notFoundError } from "../core/exit.js";
import { nowIso, saveState, type TaskStatus } from "../core/state.js";
import { applyTaskAnnotation, findTask, type Annotation } from "../core/tasksParser.js";
import { nextRunnable } from "../core/dag.js";
import { dagTasks } from "../core/render.js";
import { phaseForStatus } from "../core/phases.js";
import type { Args } from "../cli.js";

/**
 * Derive the execution-phase spec status from task states. SPEC defines no explicit phase-advance
 * command, and the agent may not edit state.json, so the CLI owns this transition: once execution
 * begins it reflects task progress. Planning statuses (requirements/design/tasks) are preserved
 * until at least one task leaves `pending`. When every task is complete the spec enters the VERIFY
 * phase (`verifying`) — a spec-level verification beat — and only `specd approve` finishes it to
 * `complete`/REFLECT, mirroring the per-task evidence gate at the spec level.
 */
function deriveStatus(state: ReturnType<typeof loadSpec>["state"]): void {
  const vals = Object.values(state.tasks);
  if (vals.length === 0) return;
  const started = vals.some((t) => t.status !== "pending");
  if (!started) return; // still in planning
  if (vals.every((t) => t.status === "complete")) {
    // Don't regress a spec a human already accepted (complete) back to verifying.
    state.status = state.status === "complete" ? "complete" : "verifying";
  } else {
    const next = nextRunnable(dagTasks(state));
    state.status = next.kind === "all-blocked" ? "blocked" : "executing";
  }
  state.phase = phaseForStatus(state.status);
}

const VALID = new Set<TaskStatus>(["complete", "blocked", "running", "pending"]);

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  const id = args.pos[1];
  if (!slug || !id) throw usageError("usage: specd task <slug> <id> --status <complete|blocked|running|pending> [flags]");

  const status = args.flags.status;
  if (typeof status !== "string" || !VALID.has(status as TaskStatus)) {
    throw usageError("--status must be one of: complete, blocked, running, pending");
  }
  const newStatus = status as TaskStatus;

  // Hold the per-spec lock across load→mutate→dual-write so a parallel builder on another task of
  // the same spec can't clobber this update (§5.1). loadSpec re-enters the same lock harmlessly.
  return withSpecLock(root, slug, () => {
  const { state, doc } = loadSpec(root, slug);

  // Gate enforcement: don't mutate task status while a midreq approval is pending (override: --force).
  if (state.gate === "awaiting-approval" && args.flags.force !== true) {
    throw gateError(`spec '${slug}' is gated (awaiting-approval) — run \`specd approve ${slug}\` after the revised plan is approved, or pass --force`);
  }

  const ts = state.tasks[id];
  const docTask = findTask(doc, id);
  if (!ts || !docTask) throw notFoundError(`task '${id}' not found in spec '${slug}'`);

  const evidence = typeof args.flags.evidence === "string" ? args.flags.evidence.trim() : "";
  const reason = typeof args.flags.reason === "string" ? args.flags.reason.trim() : "";
  const stamp = nowIso();

  if (newStatus === "complete") {
    const incomplete = ts.depends.filter((d) => state.tasks[d]?.status !== "complete");
    if (incomplete.length) {
      throw gateError(`task ${id}: cannot complete — dependencies not complete: ${incomplete.join(", ")}`);
    }
    // G1 evidence gate: completion needs a deterministic verified record (specd ran the verify
    // line, exit 0) — NOT a free-text string on faith. The manual escape hatch (--unverified +
    // --evidence) covers read-only roles whose verify is N/A and genuinely-manual proofs.
    const unverified = args.flags.unverified === true;
    if (unverified) {
      if (!evidence) throw gateError(`task ${id}: --status complete --unverified requires non-empty --evidence`);
    } else {
      const verifyLine = (docTask.meta.verify ?? "").trim();
      const rec = ts.verification;
      if (!rec || !rec.verified) {
        throw gateError(`task ${id}: --status complete requires a passing \`specd verify ${slug} ${id}\` (exit 0) first — or pass --unverified with --evidence for a manual proof`);
      }
      if (rec.command !== verifyLine) {
        throw gateError(`task ${id}: verification is stale — recorded command (${rec.command}) ≠ current verify line (${verifyLine}); re-run \`specd verify ${slug} ${id}\``);
      }
    }
    ts.status = "complete";
    // Evidence string: the supplied proof, else a derived summary of the verified record.
    const derived = ts.verification && ts.verification.verified
      ? `verified: \`${ts.verification.command}\` → exit 0 @ ${ts.verification.gitHead ?? "no-git"} (${ts.verification.ranAt})`
      : "";
    ts.evidence = evidence || derived;
    ts.finishedAt = stamp;
    if (!ts.startedAt) ts.startedAt = stamp;
    ts.blocker = null;
    state.blockers = state.blockers.filter((b) => b.task !== id);
    docTask.checked = true;
    docTask.annotation = { kind: "complete", evidence: ts.evidence ?? "", ts: stamp };
  } else if (newStatus === "blocked") {
    if (!reason) throw gateError(`task ${id}: --status blocked requires --reason`);
    ts.status = "blocked";
    ts.blocker = reason;
    state.blockers = state.blockers.filter((b) => b.task !== id);
    state.blockers.push({ task: id, reason, since: `Turn ${state.turn}` });
    docTask.checked = false;
    docTask.annotation = { kind: "blocked", reason };
  } else if (newStatus === "running") {
    ts.status = "running";
    if (!ts.startedAt) ts.startedAt = stamp;
    ts.blocker = null;
    state.blockers = state.blockers.filter((b) => b.task !== id);
    docTask.checked = false;
    docTask.annotation = undefined;
  } else {
    // pending
    ts.status = "pending";
    ts.blocker = null;
    state.blockers = state.blockers.filter((b) => b.task !== id);
    docTask.checked = false;
    docTask.annotation = undefined;
  }

  // Dual write: tasks.md first (surgical line edit preserves comments/prose), then state.json
  // (source of truth) last. Each write is atomic.
  const tasksPath = artifactPath(root, slug, "tasks.md");
  const raw = readOrDefault(tasksPath, "");
  const updated = applyTaskAnnotation(raw, id, docTask.checked, docTask.annotation as Annotation | undefined);
  atomicWrite(tasksPath, updated);
  deriveStatus(state);
  saveState(root, slug, state);

  console.log(`task ${id} → ${newStatus}`);
  if (newStatus === "complete") console.log(`  evidence: ${ts.evidence}`);
  if (newStatus === "blocked") console.log(`  blocked: ${reason}`);
  return 0;
  });
}
