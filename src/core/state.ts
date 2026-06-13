// state.json model: the machine durable ledger, CLI-owned single writer (SPEC §7.7).
import { join } from "node:path";
import { atomicWrite, readOrNull } from "./io.js";
import { specDir } from "./paths.js";
import { gateError, SpecdError } from "./exit.js";

/** state.json shape version. Bump + add a migration branch in `migrate()` when the shape changes. */
export const SCHEMA_VERSION = 4;

export type SpecStatus =
  | "requirements" | "design" | "tasks" | "executing" | "verifying" | "complete" | "blocked";
// The six reasoning phases (reasoning.md). `perceive` is the pre-spec cognitive beat — it happens
// before `specd new` creates any state — so it is never a persisted value; the persisted phase
// always derives from status via phaseForStatus(): analyze → plan → execute → verify → reflect.
export type Phase = "perceive" | "analyze" | "plan" | "execute" | "verify" | "reflect";
export type Gate = "none" | "awaiting-approval";
export type TaskStatus = "pending" | "running" | "complete" | "blocked";

/**
 * A deterministic verification record (G1): the captured result of specd *running* a task's
 * `verify:` command. specd makes no judgment — it records the OS exit code of the command the
 * task author wrote. `verified` is exactly `exitCode === 0`. The `command` is snapshotted so a
 * later edit to the task's `verify:` line can be detected as stale.
 */
export interface VerificationRecord {
  command: string;
  exitCode: number;
  verified: boolean; // exitCode === 0 && !timedOut
  timedOut: boolean;
  stdoutTail: string;
  stderrTail: string;
  durationMs: number;
  ranAt: string;
  gitHead: string | null;
}

/**
 * A per-criterion acceptance proof (G5): the spec-level VERIFY ledger. Recorded by
 * `specd verify <slug> --criterion <req>.<n> --status pass|fail --evidence "..."`. Keyed in
 * `State.acceptance` by the `<requirement>.<criterion>` string (e.g. "1.2"). When the
 * `gates.acceptance` config is `required`, `approve` of `verifying → complete` refuses while any
 * defined requirement lacks a passing criterion or any criterion is recorded as `fail`.
 */
export interface CriterionRecord {
  requirement: number;
  criterion: number;
  status: "pass" | "fail";
  evidence: string;
  ranAt: string;
}

export interface TaskState {
  id: string;
  title: string;
  role: string;
  wave: number;
  depends: string[];
  requirements: number[];
  status: TaskStatus;
  startedAt?: string | null;
  finishedAt?: string | null;
  evidence?: string | null;
  verification?: VerificationRecord;
  blocker?: string | null;
}

export interface Blocker {
  task: string;
  reason: string;
  since: string;
}

export interface State {
  schemaVersion: number;
  /** Optimistic-concurrency counter: bumped on every saveState. Compare-and-swap guard (§5.1). */
  revision: number;
  spec: string;
  title: string;
  status: SpecStatus;
  phase: Phase;
  gate: Gate;
  turn: number;
  createdAt: string;
  updatedAt: string;
  tasks: Record<string, TaskState>;
  blockers: Blocker[];
  /** Spec-level VERIFY acceptance ledger (G5), keyed by `<requirement>.<criterion>`. */
  acceptance?: Record<string, CriterionRecord>;
}

export const nowIso = (): string => new Date().toISOString();

export function initialState(spec: string, title: string): State {
  const ts = nowIso();
  return {
    schemaVersion: SCHEMA_VERSION,
    revision: 0,
    spec,
    title,
    status: "requirements",
    phase: "analyze",
    gate: "none",
    turn: 0,
    createdAt: ts,
    updatedAt: ts,
    tasks: {},
    blockers: [],
  };
}

/**
 * Migrate a parsed state object forward to SCHEMA_VERSION. Pre-versioned files (no schemaVersion)
 * are treated as v0 and stamped v1 — their shape is already compatible. Add a branch per bump.
 */
function migrate(raw: Partial<State>): State {
  if (raw.schemaVersion === undefined) raw.schemaVersion = 1; // v0 -> v1: shape-compatible
  if (raw.schemaVersion === 1) {
    raw.revision = raw.revision ?? 0; // v1 -> v2: add optimistic-concurrency revision
    raw.schemaVersion = 2;
  }
  if (raw.schemaVersion === 2) {
    // v2 -> v3: add per-task `verification` (G1). Shape-compatible — defaults to undefined.
    raw.schemaVersion = 3;
  }
  if (raw.schemaVersion === 3) {
    // v3 -> v4: add spec-level `acceptance` criterion map (G5). Shape-compatible, defaults undefined.
    raw.schemaVersion = 4;
  }
  if (raw.schemaVersion > SCHEMA_VERSION) {
    throw gateError(`state.json schemaVersion ${raw.schemaVersion} is newer than this specd (${SCHEMA_VERSION}) — upgrade specd`);
  }
  return raw as State;
}

const statePath = (root: string, slug: string) => join(specDir(root, slug), "state.json");

/** Load state.json for a spec, or null if absent. Throws a located error on corrupt JSON. */
export function loadState(root: string, slug: string): State | null {
  const raw = readOrNull(statePath(root, slug));
  if (raw === null) return null;
  let parsed: Partial<State>;
  try {
    parsed = JSON.parse(raw) as Partial<State>;
  } catch (err) {
    throw gateError(`corrupt state.json for spec '${slug}': ${err instanceof Error ? err.message : String(err)}`);
  }
  if (typeof parsed !== "object" || parsed === null || !parsed.spec) {
    throw gateError(`malformed state.json for spec '${slug}': missing required fields`);
  }
  return migrate(parsed);
}

/**
 * Save state.json atomically, touching updatedAt and bumping the revision counter. Before writing,
 * compare-and-swap: if the on-disk revision no longer matches the one we loaded, another writer
 * slipped in (a missing lock) — abort with exit 1 rather than silently clobber. Defense-in-depth
 * behind `withSpecLock`; in correctly-locked usage the revisions always match.
 */
export function saveState(root: string, slug: string, state: State): void {
  const disk = readOrNull(statePath(root, slug));
  if (disk !== null) {
    try {
      const onDisk = JSON.parse(disk) as Partial<State>;
      if (typeof onDisk.revision === "number" && onDisk.revision !== state.revision) {
        throw gateError(`state.json for '${slug}' changed underfoot (on-disk revision ${onDisk.revision} ≠ expected ${state.revision}) — concurrent write detected, reload and retry`);
      }
    } catch (err) {
      if (err instanceof SpecdError) throw err; // re-throw the CAS gate error
      // unparseable on-disk JSON: let the write proceed (atomicWrite repairs it)
    }
  }
  state.revision = (state.revision ?? 0) + 1;
  state.updatedAt = nowIso();
  atomicWrite(statePath(root, slug), JSON.stringify(state, null, 2) + "\n");
}
