import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "../helpers.js";

const REQS = `# Requirements — E2E

## Introduction
End-to-end portability proof.

## Requirement 1 — gate
**User story:** As an agent, I want gated tasks, so that quality holds.

**Acceptance criteria:**
1. WHEN a task completes THE SYSTEM SHALL require evidence
`;

const DESIGN = `# Design — E2E

## Overview
o
## Architecture
a
## Components and interfaces
c
## Data models
d
## Error handling
e
## Verification strategy
v
## Risks and open questions
r
`;

const TASKS = `# Tasks — E2E

## Wave 1
- [ ] T1 — investigate
  - why: locate
  - role: investigator
  - files: a.ts
  - contract: read only
  - acceptance: found
  - verify: N/A
  - depends: —
  - requirements: 1

## Wave 2
- [ ] T2 — build
  - why: implement requirement 1
  - role: builder
  - files: a.ts
  - contract: add check
  - acceptance: works
  - verify: true
  - depends: T1
  - requirements: 1
`;

test("full spec lifecycle: init -> new -> gates -> execute -> report", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "e2e");

  // init + new
  assert.equal((await run(dir, "init")).rc, 0);
  assert.ok(existsSync(join(dir, "AGENTS.md")));
  assert.equal((await run(dir, "new", "e2e", "--title", "E2E")).rc, 0);

  // author artifacts, then gate
  writeFileSync(join(s, "requirements.md"), REQS);
  writeFileSync(join(s, "design.md"), DESIGN);
  writeFileSync(join(s, "tasks.md"), TASKS);
  assert.equal((await run(dir, "check", "e2e")).rc, 0);

  // next -> T1
  const next1 = await run(dir, "next", "e2e", "--json");
  assert.equal(JSON.parse(next1.out).id, "T1");

  // deliberate proof-less complete is rejected (no verified record, no --unverified)
  const bad = await run(dir, "task", "e2e", "T1", "--status", "complete");
  assert.equal(bad.rc, 1);
  assert.match(bad.err, /requires a passing `specd verify/);

  // complete T2 before T1 rejected (dep gate)
  const depFail = await run(dir, "task", "e2e", "T2", "--status", "complete", "--evidence", "x");
  assert.equal(depFail.rc, 1);

  // proper completion: T1 is read-only (verify N/A) → manual proof; T2 is a builder → must verify
  assert.equal((await run(dir, "task", "e2e", "T1", "--status", "complete", "--unverified", "--evidence", "findings a.ts:1-9")).rc, 0);
  const next2 = await run(dir, "next", "e2e", "--json");
  assert.equal(JSON.parse(next2.out).id, "T2");
  assert.equal((await run(dir, "verify", "e2e", "T2")).rc, 0);
  assert.equal((await run(dir, "task", "e2e", "T2", "--status", "complete")).rc, 0);

  // all tasks done → spec-level VERIFY gate (status verifying, phase verify), not complete yet
  const verifying = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(verifying.status, "verifying");
  assert.equal(verifying.phase, "verify");
  // human accepts verification → complete / reflect
  assert.equal((await run(dir, "approve", "e2e")).rc, 0);

  // post-completion check stays green (no drift) and status is complete
  assert.equal((await run(dir, "check", "e2e")).rc, 0);
  const st = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(st.status, "complete");
  assert.equal(st.phase, "reflect");
  assert.equal(st.tasks.T1.evidence.length > 0, true);
  assert.equal(st.tasks.T2.evidence.length > 0, true);

  // tasks.md checkboxes reflect completion
  const tasksMd = readFileSync(join(s, "tasks.md"), "utf8");
  assert.match(tasksMd, /- \[x\] T1/);
  assert.match(tasksMd, /- \[x\] T2/);

  // report renders both formats
  const md = await run(dir, "report", "e2e");
  assert.match(md.out, /Complete/);
  assert.equal((await run(dir, "report", "e2e", "--format", "html", "--out", "r.html")).rc, 0);
  assert.ok(existsSync(join(dir, "r.html")));
});

test("blocked task records blocker and stops the frontier", async () => {
  const dir = newTmp();
  const s = join(dir, ".specd", "specs", "b");
  await run(dir, "init");
  await run(dir, "new", "b");
  writeFileSync(join(s, "tasks.md"), TASKS.replace(/E2E/g, "B"));
  await run(dir, "task", "b", "T1", "--status", "blocked", "--reason", "missing API key");
  const st = JSON.parse(readFileSync(join(s, "state.json"), "utf8"));
  assert.equal(st.tasks.T1.status, "blocked");
  assert.equal(st.blockers.length, 1);
  const next = await run(dir, "next", "b", "--json");
  // T1 blocked, T2 waits on T1
  assert.notEqual(JSON.parse(next.out).kind, "task");
});
