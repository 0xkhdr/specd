// Direct tests for the integrity core: the evidence gate, dependency gate, awaiting-approval
// gate, dual-write sync (tasks.md <-> state.json), and corrupt-state handling. CLAUDE.md calls
// task.ts "the integrity core" and lists its rules as non-negotiable invariants (§5.2, §3.1).
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const TASKS = `# Tasks — Integrity

## Wave 1
- [ ] T1 — first
  - why: groundwork
  - role: builder
  - files: a.ts
  - contract: add a
  - acceptance: a exists
  - verify: true
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T2 — second
  - why: depends on first
  - role: builder
  - files: b.ts
  - contract: add b
  - acceptance: b exists
  - verify: true
  - depends: T1
  - requirements: 1
`;

/** Scaffold a spec with the TASKS DAG above and return its dir + spec dir. */
async function fixture(): Promise<{ dir: string; s: string }> {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "ig");
  await run(dir, "init");
  await run(dir, "new", "ig");
  writeFileSync(join(s, "tasks.md"), TASKS);
  return { dir, s };
}

const readState = (s: string) => JSON.parse(readFileSync(join(s, "state.json"), "utf8"));

test("evidence gate: complete without a verified record is rejected (exit 1, no mutation)", async () => {
  const { dir, s } = await fixture();
  const r = await run(dir, "task", "ig", "T1", "--status", "complete");
  assert.equal(r.rc, 1);
  assert.match(r.err, /requires a passing `specd verify/);
  assert.equal(readState(s).tasks.T1.status, "pending"); // untouched
});

test("evidence gate: --unverified with whitespace-only --evidence is rejected", async () => {
  const { dir, s } = await fixture();
  const r = await run(dir, "task", "ig", "T1", "--status", "complete", "--unverified", "--evidence", "   ");
  assert.equal(r.rc, 1);
  assert.match(r.err, /requires non-empty --evidence/);
  assert.equal(readState(s).tasks.T1.status, "pending");
});

test("dependency gate: completing T2 before T1 is rejected", async () => {
  const { dir, s } = await fixture();
  const r = await run(dir, "task", "ig", "T2", "--status", "complete", "--evidence", "x");
  assert.equal(r.rc, 1);
  assert.match(r.err, /dependencies not complete: T1/);
  assert.equal(readState(s).tasks.T2.status, "pending");
});

test("happy path: verify then complete records the verified record, timestamp, and checks the box", async () => {
  const { dir, s } = await fixture();
  const v = await run(dir, "verify", "ig", "T1");
  assert.equal(v.rc, 0);
  assert.equal(readState(s).tasks.T1.verification.verified, true);

  const r = await run(dir, "task", "ig", "T1", "--status", "complete");
  assert.equal(r.rc, 0);
  const st = readState(s);
  assert.equal(st.tasks.T1.status, "complete");
  assert.match(st.tasks.T1.evidence, /^verified: `true` → exit 0/); // derived from the record
  assert.ok(st.tasks.T1.finishedAt);
  assert.equal(st.status, "executing"); // T2 still pending
  // dual-write: tasks.md checkbox + annotation reflect state
  const md = readFileSync(join(s, "tasks.md"), "utf8");
  assert.match(md, /- \[x\] T1 .*✓ complete · evidence: verified:/);
});

test("explicit --evidence overrides the derived verified summary", async () => {
  const { dir, s } = await fixture();
  await run(dir, "verify", "ig", "T1");
  const r = await run(dir, "task", "ig", "T1", "--status", "complete", "--evidence", "commit abc");
  assert.equal(r.rc, 0);
  assert.equal(readState(s).tasks.T1.evidence, "commit abc");
});

test("blocked: requires --reason and records a blocker", async () => {
  const { dir, s } = await fixture();
  const noReason = await run(dir, "task", "ig", "T1", "--status", "blocked");
  assert.equal(noReason.rc, 1);
  assert.match(noReason.err, /requires --reason/);

  const r = await run(dir, "task", "ig", "T1", "--status", "blocked", "--reason", "missing key");
  assert.equal(r.rc, 0);
  const st = readState(s);
  assert.equal(st.tasks.T1.status, "blocked");
  assert.equal(st.blockers.length, 1);
  assert.equal(st.blockers[0].reason, "missing key");
  assert.match(readFileSync(join(s, "tasks.md"), "utf8"), /⚠ blocked · reason: missing key/);
});

test("awaiting-approval gate refuses task flips unless --force", async () => {
  const { dir, s } = await fixture();
  await run(dir, "midreq", "ig", "scope change", "--impact", "critical"); // sets gate
  const blocked = await run(dir, "task", "ig", "T1", "--status", "complete", "--unverified", "--evidence", "x");
  assert.equal(blocked.rc, 1);
  assert.match(blocked.err, /gated \(awaiting-approval\)/);
  assert.equal(readState(s).tasks.T1.status, "pending");

  const forced = await run(dir, "task", "ig", "T1", "--status", "complete", "--unverified", "--evidence", "x", "--force");
  assert.equal(forced.rc, 0);
  assert.equal(readState(s).tasks.T1.status, "complete");
});

test("invalid --status is a usage error (exit 2)", async () => {
  const { dir } = await fixture();
  const r = await run(dir, "task", "ig", "T1", "--status", "done");
  assert.equal(r.rc, 2);
});

test("unknown task id is not-found (exit 3)", async () => {
  const { dir } = await fixture();
  const r = await run(dir, "task", "ig", "T99", "--status", "running");
  assert.equal(r.rc, 3);
});

test("corrupt state.json surfaces a located gate error, not a raw crash", async () => {
  const { dir, s } = await fixture();
  writeFileSync(join(s, "state.json"), "{ not json");
  const r = await run(dir, "task", "ig", "T1", "--status", "running");
  assert.equal(r.rc, 1);
  assert.match(r.err, /corrupt state\.json for spec 'ig'/);
});

test("re-completing an already-complete task stays idempotent (no checkbox/state drift)", async () => {
  const { dir, s } = await fixture();
  await run(dir, "task", "ig", "T1", "--status", "complete", "--unverified", "--evidence", "e1");
  const r = await run(dir, "task", "ig", "T1", "--status", "complete", "--unverified", "--evidence", "e2");
  assert.equal(r.rc, 0);
  assert.equal(readState(s).tasks.T1.evidence, "e2");
  // sync invariant: exactly one [x] for T1, state agrees it is complete
  const md = readFileSync(join(s, "tasks.md"), "utf8");
  assert.equal((md.match(/- \[x\] T1/g) ?? []).length, 1);
  assert.equal(readState(s).tasks.T1.status, "complete");
});
