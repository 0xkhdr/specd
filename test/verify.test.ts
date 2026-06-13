// G1: `specd verify` runs a task's verify: command deterministically and records the OS exit code.
// `specd task --status complete` then requires a matching exit-0 record (or an explicit
// --unverified manual proof for read-only roles). specd makes no judgment — it records the result.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

/** One-builder + one-investigator DAG with a parameterized builder verify command. */
const tasks = (builderVerify: string) => `# Tasks — Verify

## Wave 1
- [ ] T1 — read
  - why: locate
  - role: investigator
  - files: a.ts
  - contract: read only
  - acceptance: found
  - verify: N/A
  - depends: —
  - requirements: 1
- [ ] T2 — build
  - why: implement
  - role: builder
  - files: a.ts
  - contract: add a
  - acceptance: a exists
  - verify: ${builderVerify}
  - depends: —
  - requirements: 1
`;

async function fixture(builderVerify: string): Promise<{ dir: string; s: string }> {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "vr");
  await run(dir, "init");
  await run(dir, "new", "vr");
  writeFileSync(join(s, "tasks.md"), tasks(builderVerify));
  return { dir, s };
}

const readState = (s: string) => JSON.parse(readFileSync(join(s, "state.json"), "utf8"));

test("verify-pass: exit-0 command writes a verified record and unlocks complete", async () => {
  const { dir, s } = await fixture("true");
  const v = await run(dir, "verify", "vr", "T2");
  assert.equal(v.rc, 0);
  const rec = readState(s).tasks.T2.verification;
  assert.equal(rec.verified, true);
  assert.equal(rec.exitCode, 0);
  assert.equal(rec.command, "true");

  const c = await run(dir, "task", "vr", "T2", "--status", "complete");
  assert.equal(c.rc, 0);
  assert.equal(readState(s).tasks.T2.status, "complete");
});

test("verify-fail: non-zero exit records verified:false and blocks complete", async () => {
  const { dir, s } = await fixture("false");
  const v = await run(dir, "verify", "vr", "T2");
  assert.equal(v.rc, 1);
  const rec = readState(s).tasks.T2.verification;
  assert.equal(rec.verified, false);
  assert.notEqual(rec.exitCode, 0);

  const c = await run(dir, "task", "vr", "T2", "--status", "complete");
  assert.equal(c.rc, 1);
  assert.match(c.err, /requires a passing `specd verify/);
  assert.equal(readState(s).tasks.T2.status, "pending");
});

test("stale record: editing the verify line after verifying rejects complete", async () => {
  const { dir, s } = await fixture("true");
  assert.equal((await run(dir, "verify", "vr", "T2")).rc, 0);
  // Author changes the verify command — the recorded proof no longer matches.
  writeFileSync(join(s, "tasks.md"), tasks("echo changed"));
  const c = await run(dir, "task", "vr", "T2", "--status", "complete");
  assert.equal(c.rc, 1);
  assert.match(c.err, /verification is stale/);
  assert.equal(readState(s).tasks.T2.status, "pending");
});

test("read-only role: verify refuses N/A, but --unverified --evidence completes", async () => {
  const { dir, s } = await fixture("true");
  const v = await run(dir, "verify", "vr", "T1");
  assert.equal(v.rc, 1);
  assert.match(v.err, /no runnable command/);

  const c = await run(dir, "task", "vr", "T1", "--status", "complete", "--unverified", "--evidence", "findings a.ts:1-9");
  assert.equal(c.rc, 0);
  assert.equal(readState(s).tasks.T1.status, "complete");
});

test("check gate 5: a builder completed without a verified record is a violation", async () => {
  const { dir, s } = await fixture("true");
  // Force-complete the builder via the manual escape hatch — allowed by `task`, flagged by `check`.
  await run(dir, "task", "vr", "T2", "--status", "complete", "--unverified", "--evidence", "manual");
  assert.equal(readState(s).tasks.T2.status, "complete");
  const c = await run(dir, "check", "vr");
  assert.equal(c.rc, 1);
  assert.match(c.err, /T2: complete without a verified record/);
});
