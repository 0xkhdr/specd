// G5: VERIFY gets per-criterion proof + traceability is a configurable gate.
// `specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence "..."` records a
// spec-level acceptance ledger; with `gates.acceptance: required`, `approve` of verifying →
// complete refuses while any requirement lacks a passing criterion. `gates.traceability: error`
// promotes the forward-traceability warning to a hard `check` violation.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const oneBuilder = `# Tasks — G5

## Wave 1
- [ ] T1 — build
  - why: implement
  - role: builder
  - files: a.ts
  - contract: add a
  - acceptance: a exists
  - verify: true
  - depends: —
  - requirements: 1
`;

const readState = (s: string) => JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
const writeConfig = (dir: string, gates: object) =>
  writeFileSync(join(dir, ".specd", "config.json"), JSON.stringify({ version: 1, gates }, null, 2));

/** Init a spec, write config + tasks, drive the single builder to `verifying`. */
async function atVerify(gates: object): Promise<{ dir: string; s: string }> {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "g5");
  await run(dir, "init");
  await run(dir, "new", "g5");
  writeConfig(dir, gates);
  writeFileSync(join(s, "tasks.md"), oneBuilder);
  assert.equal((await run(dir, "verify", "g5", "T1")).rc, 0);
  assert.equal((await run(dir, "task", "g5", "T1", "--status", "complete")).rc, 0);
  assert.equal(readState(s).status, "verifying");
  return { dir, s };
}

test("acceptance gate: approve refuses verifying→complete until every requirement has a pass", async () => {
  const { dir, s } = await atVerify({ acceptance: "required" });

  const blocked = await run(dir, "approve", "g5");
  assert.equal(blocked.rc, 1);
  assert.match(blocked.err, /requirement 1: no passing acceptance criterion/);
  assert.equal(readState(s).status, "verifying");

  // Record the passing criterion, then approve clears.
  const rec = await run(dir, "verify", "g5", "--criterion", "1.1", "--status", "pass", "--evidence", "manual UAT ok");
  assert.equal(rec.rc, 0);
  assert.equal(readState(s).acceptance["1.1"].status, "pass");

  const ok = await run(dir, "approve", "g5");
  assert.equal(ok.rc, 0);
  assert.equal(readState(s).status, "complete");
});

test("acceptance gate: a recorded fail blocks approve and exits 1", async () => {
  const { dir, s } = await atVerify({ acceptance: "required" });
  const rec = await run(dir, "verify", "g5", "--criterion", "1.1", "--status", "fail", "--evidence", "regression in edge case");
  assert.equal(rec.rc, 1);

  const blocked = await run(dir, "approve", "g5");
  assert.equal(blocked.rc, 1);
  assert.match(blocked.err, /criterion 1\.1: recorded as fail/);
  assert.equal(readState(s).status, "verifying");
});

test("acceptance off (default): approve advances verifying→complete with no criterion map", async () => {
  const { dir, s } = await atVerify({ acceptance: "off" });
  assert.equal((await run(dir, "approve", "g5")).rc, 0);
  assert.equal(readState(s).status, "complete");
});

test("criterion validation: bad format, unknown requirement, missing evidence", async () => {
  const { dir } = await atVerify({ acceptance: "required" });

  const badFmt = await run(dir, "verify", "g5", "--criterion", "1", "--status", "pass", "--evidence", "x");
  assert.equal(badFmt.rc, 2);
  assert.match(badFmt.err, /--criterion must be <requirement>\.<n>/);

  const unknown = await run(dir, "verify", "g5", "--criterion", "9.1", "--status", "pass", "--evidence", "x");
  assert.equal(unknown.rc, 1);
  assert.match(unknown.err, /requirement 9 is not defined/);

  const noEv = await run(dir, "verify", "g5", "--criterion", "1.1", "--status", "pass");
  assert.equal(noEv.rc, 2);
  assert.match(noEv.err, /--evidence "<proof>" is required/);
});

test("traceability gate: uncovered requirement warns by default, errors when configured", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "g5");
  await run(dir, "init");
  await run(dir, "new", "g5");
  writeFileSync(join(s, "tasks.md"), oneBuilder);
  // Add a second requirement that no task references.
  const req = readFileSync(join(s, "requirements.md"), "utf8");
  writeFileSync(join(s, "requirements.md"), req + "\n## Requirement 2 — uncovered\n\nWHEN x the system SHALL y.\n");

  writeConfig(dir, { traceability: "warn" });
  const warned = await run(dir, "check", "g5");
  assert.match(warned.out + warned.err, /requirement 2 not referenced by any task/);

  writeConfig(dir, { traceability: "error" });
  const errored = await run(dir, "check", "g5");
  assert.equal(errored.rc, 1);
  assert.match(errored.err, /requirement 2 not referenced by any task .*traceability/);
});
