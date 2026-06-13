// Coverage for this hardening pass: spec-level VERIFY gate (Fable 5 phase wiring),
// `specd context` (context-engineering briefing), and bidirectional traceability.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const ONE_TASK = `# Tasks — V

## Wave 1
- [ ] T1 — only
  - why: sole task
  - role: builder
  - files: a.ts
  - contract: do it
  - acceptance: done
  - verify: npm test
  - depends: —
  - requirements: 1
`;

test("phase wiring: new spec is ANALYZE; verify gate sits between done and complete", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "v");
  await run(dir, "init");
  await run(dir, "new", "v");

  // fresh spec → status requirements, phase analyze (perceive is pre-spec, never persisted)
  let st = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(st.phase, "analyze");
  assert.notEqual(st.phase, "perceive");

  writeFileSync(join(s, "tasks.md"), ONE_TASK);
  assert.equal((await run(dir, "task", "v", "T1", "--status", "complete", "--unverified", "--evidence", "npm test PASS")).rc, 0);

  // all tasks complete → verifying / verify, NOT complete
  st = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(st.status, "verifying");
  assert.equal(st.phase, "verify");

  // approve = accept verification → complete / reflect
  const ap = await run(dir, "approve", "v");
  assert.equal(ap.rc, 0);
  assert.match(ap.out, /verification accepted/);
  st = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(st.status, "complete");
  assert.equal(st.phase, "reflect");
});

test("context: emits phase-scoped minimal load list + next action", async () => {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "c");

  const r = await run(dir, "context", "c");
  assert.equal(r.rc, 0);
  assert.match(r.out, /PHASE ANALYZE/);
  assert.match(r.out, /LOAD NOW/);
  assert.match(r.out, /steering\/reasoning\.md/); // always-on base steering
  assert.match(r.out, /specs\/c\/requirements\.md/); // phase-scoped artifact
  assert.match(r.out, /specd check c/); // next action

  const j = JSON.parse((await run(dir, "context", "c", "--json")).out);
  assert.equal(j.status, "requirements");
  assert.equal(j.phaseLabel, "ANALYZE");
  assert.ok(j.load.includes(".specd/steering/reasoning.md"));
  assert.ok(j.load.includes(".specd/specs/c/requirements.md"));
});

test("context: surfaces awaiting-approval gate and freezes work", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "g");
  await run(dir, "init");
  await run(dir, "new", "g");
  writeFileSync(join(s, "tasks.md"), ONE_TASK);
  await run(dir, "midreq", "g", "big change", "--impact", "high");
  const r = await run(dir, "context", "g");
  assert.match(r.out, /GATE awaiting-approval/);
  assert.match(r.out, /specd approve g/);
});

test("memory promote: refused below threshold, allowed with --force or enough recurrence", async () => {
  const dir = newTmp();
  await run(dir, "init"); // default config promotionThreshold = 3
  const add = (spec: string) => run(dir, "memory", spec, "add",
    "--key", "retry-idempotency", "--pattern", "retries must be idempotent",
    "--body", "wrap in txn", "--source", "T1", "--criticality", "important");

  await run(dir, "new", "a");
  await add("a");

  // seen in 1 spec < threshold 3 → refused
  const refused = await run(dir, "memory", "a", "promote", "--key", "retry-idempotency");
  assert.equal(refused.rc, 1);
  assert.match(refused.err, /seen in 1 spec\(s\); promotion threshold is 3/);

  // --force overrides
  const forced = await run(dir, "memory", "a", "promote", "--key", "retry-idempotency", "--force");
  assert.equal(forced.rc, 0);

  // recurrence across ≥3 specs clears the gate without --force
  await run(dir, "new", "b"); await add("b");
  await run(dir, "new", "c"); await add("c");
  const ok = await run(dir, "memory", "a", "promote", "--key", "retry-idempotency");
  assert.equal(ok.rc, 0);
  assert.match(ok.out, /seen in 3 spec\(s\), threshold 3/);
});

test("traceability: a task referencing an undefined requirement fails check", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "tr");
  await run(dir, "init");
  await run(dir, "new", "tr");
  writeFileSync(join(s, "requirements.md"), `# Requirements — TR

## Requirement 1 — only
**User story:** As a user, I want X, so that Y.

**Acceptance criteria:**
1. THE SYSTEM SHALL do X
`);
  writeFileSync(join(s, "tasks.md"), ONE_TASK.replace("requirements: 1", "requirements: 5"));
  const r = await run(dir, "check", "tr");
  assert.equal(r.rc, 1);
  assert.match(r.err, /references requirement 5 which is not defined/);
});
