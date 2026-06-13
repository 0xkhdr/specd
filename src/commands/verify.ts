// `specd verify <slug> <id>` — deterministically RUN a task's `verify:` command and record the
// result (G1). specd makes zero judgments: it spawns the shell command the task author wrote,
// captures the OS exit code + output tail + git HEAD, and writes a VerificationRecord into
// state.json. `specd task <id> --status complete` then requires a matching exit-0 record instead
// of trusting a free-text evidence string. Zero LLM, zero new runtime deps (child_process stdlib).
import { spawnSync } from "node:child_process";
import { requireSpecdRoot } from "../core/paths.js";
import { loadSpec, readArtifact } from "../core/specFiles.js";
import { withSpecLock } from "../core/lock.js";
import { usageError, gateError, notFoundError } from "../core/exit.js";
import { nowIso, saveState, type VerificationRecord, type CriterionRecord } from "../core/state.js";
import { findTask } from "../core/tasksParser.js";
import { requirementNumbers } from "../core/render.js";
import type { Args } from "../cli.js";

/** Keep only the trailing `max` chars of captured output (head-dropped, with a marker). */
function tail(s: string, max = 2000): string {
  if (s.length <= max) return s;
  return `…(${s.length - max} chars truncated)…\n` + s.slice(s.length - max);
}

/** Timeout in ms for a single verify run (env-overridable). 0/invalid → default 600s. */
function timeoutMs(): number {
  const v = parseInt(process.env.SPECD_VERIFY_TIMEOUT_MS ?? "", 10);
  return Number.isFinite(v) && v > 0 ? v : 600_000;
}

/** Resolve the current git HEAD at `cwd`, or null if not a repo / git absent. */
function gitHead(cwd: string): string | null {
  const r = spawnSync("git", ["rev-parse", "HEAD"], { cwd, encoding: "utf8" });
  if (r.status === 0 && typeof r.stdout === "string") return r.stdout.trim() || null;
  return null;
}

export function run(args: Args): number {
  const root = requireSpecdRoot();
  const slug = args.pos[0];
  if (!slug) throw usageError(`usage: specd verify <slug> <id>  |  specd verify <slug> --criterion <req>.<n> --status pass|fail --evidence "..."`);

  // G5 criterion-recording mode: record a per-criterion acceptance proof into the spec-level ledger.
  if (args.flags.criterion !== undefined) return recordCriterion(root, slug, args);

  const id = args.pos[1];
  if (!id) throw usageError("usage: specd verify <slug> <id>");

  return withSpecLock(root, slug, () => {
    const { state, doc } = loadSpec(root, slug);
    const ts = state.tasks[id];
    const docTask = findTask(doc, id);
    if (!ts || !docTask) throw notFoundError(`task '${id}' not found in spec '${slug}'`);

    const command = (docTask.meta.verify ?? "").trim();
    if (command === "" || command.toUpperCase().startsWith("N/A")) {
      throw gateError(`task ${id}: verify is '${command || "—"}' (no runnable command) — read-only roles complete with \`specd task ${slug} ${id} --status complete --unverified --evidence "..."\``);
    }

    const startedAt = nowIso();
    const t0 = Date.now();
    const proc = spawnSync(command, {
      cwd: root,
      shell: true,
      encoding: "utf8",
      timeout: timeoutMs(),
      maxBuffer: 16 * 1024 * 1024,
    });
    const durationMs = Date.now() - t0;
    const timedOut = (proc.error as NodeJS.ErrnoException | undefined)?.code === "ETIMEDOUT";
    // spawnSync sets status=null on signal/timeout; treat any null exit as failure (124, like timeout).
    const exitCode = typeof proc.status === "number" ? proc.status : 124;

    const record: VerificationRecord = {
      command,
      exitCode,
      verified: exitCode === 0 && !timedOut,
      timedOut,
      stdoutTail: tail(proc.stdout ?? ""),
      stderrTail: tail(proc.stderr ?? ""),
      durationMs,
      ranAt: startedAt,
      gitHead: gitHead(root),
    };

    ts.verification = record;
    saveState(root, slug, state);

    const mark = record.verified ? "✓ verified" : "✗ FAILED";
    console.log(`${mark} — ${id}: \`${command}\` → exit ${exitCode}${timedOut ? " (timed out)" : ""} in ${durationMs}ms`);
    if (record.gitHead) console.log(`  gitHead: ${record.gitHead}`);
    if (!record.verified && record.stderrTail.trim()) console.log(`  stderr tail:\n${record.stderrTail}`);
    if (record.verified) console.log(`  complete with: specd task ${slug} ${id} --status complete`);
    return record.verified ? 0 : 1;
  });
}

/**
 * G5: record a per-criterion acceptance proof into `state.acceptance`. The criterion key is
 * `<requirement>.<n>`; the requirement must be defined in requirements.md. specd makes no judgment —
 * it records the pass/fail the verifier asserts, with mandatory evidence. Exit 0 for pass, 1 for fail.
 */
function recordCriterion(root: string, slug: string, args: Args): number {
  const key = String(args.flags.criterion ?? "");
  const m = key.match(/^(\d+)\.(\d+)$/);
  if (!m) throw usageError(`--criterion must be <requirement>.<n> (e.g. 1.2), got '${key}'`);
  const status = args.flags.status;
  if (status !== "pass" && status !== "fail") throw usageError(`--status must be 'pass' or 'fail'`);
  const evidence = typeof args.flags.evidence === "string" ? args.flags.evidence.trim() : "";
  if (!evidence) throw usageError(`--evidence "<proof>" is required when recording a criterion`);
  const requirement = parseInt(m[1], 10);
  const criterion = parseInt(m[2], 10);

  return withSpecLock(root, slug, () => {
    const { state } = loadSpec(root, slug);
    const reqMd = readArtifact(root, slug, "requirements.md");
    if (reqMd && !requirementNumbers(reqMd).has(requirement)) {
      throw gateError(`requirement ${requirement} is not defined in requirements.md`);
    }
    const record: CriterionRecord = { requirement, criterion, status, evidence, ranAt: nowIso() };
    if (!state.acceptance) state.acceptance = {};
    state.acceptance[key] = record;
    saveState(root, slug, state);

    console.log(`${status === "pass" ? "✓ pass" : "✗ fail"} — criterion ${key} (requirement ${requirement}) recorded.`);
    return status === "pass" ? 0 : 1;
  });
}
