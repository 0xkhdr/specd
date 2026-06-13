// G2 — `specd dispatch`: ready-to-run packets for the runnable frontier. Each packet must carry the
// resolved role-prompt body + contract/files/acceptance/verify + the exact completion command, so an
// orchestrator can fan the frontier out to parallel subagents with zero hand-assembly.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const FRONTIER_TASKS = `# Tasks — F

## Wave 1
- [ ] T1 — first
  - why: a
  - role: builder
  - files: a.ts
  - contract: do a
  - acceptance: a done
  - verify: npm test
  - depends: —
  - requirements: 1
- [ ] T2 — second
  - why: b
  - role: reviewer
  - files: b.ts
  - contract: do b
  - acceptance: b done
  - verify: N/A
  - depends: —
  - requirements: 1, 2

## Wave 2
- [ ] T3 — third
  - why: c
  - role: builder
  - files: c.ts
  - contract: do c
  - acceptance: c done
  - verify: npm test
  - depends: T1, T2
  - requirements: 3
`;

async function setup(): Promise<string> {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "f");
  writeFileSync(join(dir, ".specd", "specs", "f", "tasks.md"), FRONTIER_TASKS);
  return dir;
}

test("dispatch --json: full packets for the runnable frontier with resolved role bodies", async () => {
  const dir = await setup();
  const j = JSON.parse((await run(dir, "dispatch", "f", "--json")).out);

  assert.equal(j.kind, "frontier");
  assert.equal(j.count, 2);
  assert.deepEqual(j.packets.map((p: { id: string }) => p.id), ["T1", "T2"]); // wave-2 T3 still gated

  const t1 = j.packets[0];
  assert.equal(t1.role, "builder");
  assert.equal(t1.contract, "do a");
  assert.equal(t1.files, "a.ts");
  assert.equal(t1.acceptance, "a done");
  assert.equal(t1.verify, "npm test");
  assert.deepEqual(t1.requirements, [1]);
  assert.deepEqual(t1.depends, []);
  assert.equal(t1.completion, 'specd task f T1 --status complete --evidence "<proof>"');

  // Role prompt is the actual body of .specd/roles/builder.md, not a stub.
  const builderMd = readFileSync(join(dir, ".specd", "roles", "builder.md"), "utf8");
  assert.equal(t1.rolePrompt, builderMd);
  assert.match(t1.rolePrompt, /Role: Builder/);

  // Distinct role resolves to its own body.
  const t2 = j.packets[1];
  assert.equal(t2.role, "reviewer");
  const reviewerMd = readFileSync(join(dir, ".specd", "roles", "reviewer.md"), "utf8");
  assert.equal(t2.rolePrompt, reviewerMd);
  assert.deepEqual(t2.requirements, [1, 2]);
});

test("dispatch: frontier advances as deps clear, then classifies empty as all-complete", async () => {
  const dir = await setup();

  await run(dir, "task", "f", "T1", "--status", "complete", "--unverified", "--evidence", "ok");
  await run(dir, "task", "f", "T2", "--status", "complete", "--unverified", "--evidence", "ok");
  let j = JSON.parse((await run(dir, "dispatch", "f", "--json")).out);
  assert.deepEqual(j.packets.map((p: { id: string }) => p.id), ["T3"]); // deps cleared

  await run(dir, "task", "f", "T3", "--status", "complete", "--unverified", "--evidence", "ok");
  j = JSON.parse((await run(dir, "dispatch", "f", "--json")).out);
  assert.equal(j.count, 0);
  assert.equal(j.reason, "all-complete");
  assert.deepEqual(j.packets, []);
});

test("dispatch text mode: compact summary lists each frontier task + completion command", async () => {
  const dir = await setup();
  const r = await run(dir, "dispatch", "f");
  assert.equal(r.rc, 0);
  assert.match(r.out, /DISPATCH FRONTIER \(2\)/);
  assert.match(r.out, /T1 .* first .*\(builder\)/);
  assert.match(r.out, /specd task f T1 --status complete/);
  assert.match(r.out, /specd dispatch f --json/);
});

test("dispatch: awaiting-approval gate halts dispatch until approve", async () => {
  const dir = await setup();
  await run(dir, "midreq", "f", "scope change", "--impact", "high");

  const j = JSON.parse((await run(dir, "dispatch", "f", "--json")).out);
  assert.equal(j.kind, "gated");
  assert.equal(j.gate, "awaiting-approval");

  const rc = (await run(dir, "dispatch", "f", "--json")).rc;
  assert.equal(rc, 1);

  // --force overrides the gate and emits packets.
  const forced = JSON.parse((await run(dir, "dispatch", "f", "--json", "--force")).out);
  assert.equal(forced.kind, "frontier");
  assert.equal(forced.count, 2);
});
