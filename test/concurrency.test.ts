// Concurrency-hardening coverage (§5.1–5.3): the per-spec lock, optimistic-revision compare-and-swap,
// atomic ledger appends, and the runnable-frontier (`next --all`). These guard the parallel
// multi-agent workload the philosophy invites, which the serial correctness tests never exercise.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";
import { loadState, saveState } from "../src/core/state.js";
import { appendFile } from "../src/core/io.js";

const FRONTIER_TASKS = `# Tasks — F

## Wave 1
- [ ] T1 — first
  - why: a
  - role: builder
  - files: a.ts
  - contract: do
  - acceptance: done
  - verify: npm test
  - depends: —
  - requirements: 1
- [ ] T2 — second
  - why: b
  - role: builder
  - files: b.ts
  - contract: do
  - acceptance: done
  - verify: npm test
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T3 — third
  - why: c
  - role: builder
  - files: c.ts
  - contract: do
  - acceptance: done
  - verify: npm test
  - depends: T1, T2
  - requirements: 1
`;

const lockFile = (dir: string, slug: string) => join(dir, ".specd", "specs", slug, ".lock");

test("revision CAS: a stale in-memory state cannot clobber a newer on-disk write", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "x");

  // Two independent loads of the same revision — simulating two processes that read, then race to write.
  const a = loadState(dir, "x")!;
  const b = loadState(dir, "x")!;
  assert.equal(a.revision, b.revision);

  saveState(dir, "x", a); // first writer wins, bumps the on-disk revision
  assert.throws(() => saveState(dir, "x", b), /changed underfoot/); // second writer is caught, not silently lost
});

test("lock contention: a live lock makes a mutating command fail loudly (exit 1)", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "y");
  writeFileSync(join(dir, ".specd", "specs", "y", "tasks.md"), FRONTIER_TASKS);

  // Plant a fresh (non-stale) lock and shorten the acquire timeout so the test is fast.
  writeFileSync(lockFile(dir, "y"), `99999 ${Date.now()}\n`);
  const prevTimeout = process.env.SPECD_LOCK_TIMEOUT_MS;
  process.env.SPECD_LOCK_TIMEOUT_MS = "120";
  try {
    const r = await run(dir, "task", "y", "T1", "--status", "complete", "--evidence", "x");
    assert.equal(r.rc, 1);
    assert.match(r.err, /locked by another specd process/);
  } finally {
    if (prevTimeout === undefined) delete process.env.SPECD_LOCK_TIMEOUT_MS;
    else process.env.SPECD_LOCK_TIMEOUT_MS = prevTimeout;
  }
});

test("stale lock is reclaimed: an orphaned lock does not wedge the spec forever", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "z");
  writeFileSync(join(dir, ".specd", "specs", "z", "tasks.md"), FRONTIER_TASKS);

  // Plant a lock far older than the stale threshold (default 30s) — presumed orphaned by a dead process.
  writeFileSync(lockFile(dir, "z"), `12345 ${Date.now() - 9_999_999}\n`);
  const r = await run(dir, "task", "z", "T1", "--status", "complete", "--unverified", "--evidence", "npm test PASS");
  assert.equal(r.rc, 0);
  assert.equal(existsSync(lockFile(dir, "z")), false); // reclaimed and released
});

test("atomic ledger append: many appends preserve every entry, none truncated", () => {
  const dir = newTmp();
  const f = join(dir, "ledger.md");
  for (let i = 0; i < 50; i++) appendFile(f, `line ${i}\n`);
  const lines = readFileSync(f, "utf8").trim().split("\n");
  assert.equal(lines.length, 50);
  assert.equal(lines[0], "line 0");
  assert.equal(lines[49], "line 49");
});

test("midreq logs accumulate (append, never overwrite)", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "m");
  await run(dir, "midreq", "m", "first change", "--impact", "low");
  await run(dir, "midreq", "m", "second change", "--impact", "low");
  const log = readFileSync(join(dir, ".specd", "specs", "m", "mid-requirements.md"), "utf8");
  assert.match(log, /first change/);
  assert.match(log, /second change/);
  assert.match(log, /Turn 1/);
  assert.match(log, /Turn 2/);
});

test("next --all: returns the whole runnable frontier and advances as deps clear", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "f");
  writeFileSync(join(dir, ".specd", "specs", "f", "tasks.md"), FRONTIER_TASKS);

  // Frontier is both independent wave-1 tasks; the wave-2 task is gated by its deps.
  let j = JSON.parse((await run(dir, "next", "f", "--all", "--json")).out);
  assert.equal(j.kind, "frontier");
  assert.deepEqual(j.tasks.map((t: { id: string }) => t.id).sort(), ["T1", "T2"]);

  await run(dir, "task", "f", "T1", "--status", "complete", "--unverified", "--evidence", "ok");
  j = JSON.parse((await run(dir, "next", "f", "--all", "--json")).out);
  assert.deepEqual(j.tasks.map((t: { id: string }) => t.id), ["T2"]); // T1 done, T3 still gated

  await run(dir, "task", "f", "T2", "--status", "complete", "--unverified", "--evidence", "ok");
  j = JSON.parse((await run(dir, "next", "f", "--all", "--json")).out);
  assert.deepEqual(j.tasks.map((t: { id: string }) => t.id), ["T3"]); // deps cleared, T3 now runnable

  await run(dir, "task", "f", "T3", "--status", "complete", "--unverified", "--evidence", "ok");
  j = JSON.parse((await run(dir, "next", "f", "--all", "--json")).out);
  assert.equal(j.count, 0); // frontier empty — all complete
});

test("next --all: respects the awaiting-approval gate", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "g");
  writeFileSync(join(dir, ".specd", "specs", "g", "tasks.md"), FRONTIER_TASKS);
  await run(dir, "midreq", "g", "big", "--impact", "critical"); // raises the gate
  const r = await run(dir, "next", "g", "--all");
  assert.equal(r.rc, 1);
  assert.match(r.err, /awaiting-approval/);
});
