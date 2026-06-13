import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

// Requirements.md with TWO requirements; tasks below reference only requirement 1, so requirement 2
// is uncovered — exercised by the VERIFY-coverage signal.
const REQS = `# Requirements — Ctx

## Introduction
Context signal coverage.

## Requirement 1 — gate
**User story:** As an agent, I want gated tasks, so that quality holds.

**Acceptance criteria:**
1. WHEN a task completes THE SYSTEM SHALL require evidence

## Requirement 2 — orphan
**User story:** As an agent, I want full traceability, so that nothing is missed.

**Acceptance criteria:**
1. THE SYSTEM SHALL flag uncovered requirements
`;

const DESIGN = `# Design — Ctx

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

const TASKS = `# Tasks — Ctx

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
  - verify: npm test
  - depends: T1
  - requirements: 1
`;

/** Scaffold a spec with all three artifacts authored. */
async function scaffold(dir: string, slug: string): Promise<string> {
  const s = join(dir, ".specd", "specs", slug);
  await run(dir, "init");
  await run(dir, "new", slug, "--title", "Ctx");
  writeFileSync(join(s, "requirements.md"), REQS);
  writeFileSync(join(s, "design.md"), DESIGN);
  writeFileSync(join(s, "tasks.md"), TASKS);
  return s;
}

test("context: blocker surfaces as a SIGNAL during EXECUTE", async () => {
  const dir = newTmp();
  await scaffold(dir, "blk");
  // Block T1 (T2 depends on it) → status executing, one recorded blocker.
  await run(dir, "task", "blk", "T1", "--status", "blocked", "--reason", "missing API key");

  const txt = await run(dir, "context", "blk");
  assert.match(txt.out, /SIGNALS:/);
  assert.match(txt.out, /! blocker T1: missing API key/);

  const j = JSON.parse((await run(dir, "context", "blk", "--json")).out);
  assert.equal(j.status, "executing");
  assert.deepEqual(j.signals.blockers, ["T1: missing API key"]);
});

test("context: gated midreq shows impact + verbatim input", async () => {
  const dir = newTmp();
  await scaffold(dir, "mr");
  await run(dir, "midreq", "mr", "drop the export feature", "--impact", "high",
    "--interpretation", "remove export", "--changes", "delete T2");

  const txt = await run(dir, "context", "mr");
  assert.match(txt.out, /GATE awaiting-approval/);
  assert.match(txt.out, /midreq Turn 1 \(high\): "drop the export feature"/);

  const j = JSON.parse((await run(dir, "context", "mr", "--json")).out);
  assert.equal(j.signals.latestMidreq.impact, "high");
  assert.equal(j.signals.latestMidreq.input, "drop the export feature");
});

test("context: VERIFY lists uncovered requirements", async () => {
  const dir = newTmp();
  await scaffold(dir, "vf");
  // Complete every task → spec enters `verifying`. Requirement 2 is referenced by no task.
  await run(dir, "task", "vf", "T1", "--status", "complete", "--unverified", "--evidence", "findings a.ts:1-9");
  await run(dir, "task", "vf", "T2", "--status", "complete", "--unverified", "--evidence", "commit abc; npm test PASS");

  const txt = await run(dir, "context", "vf");
  assert.match(txt.out, /! uncovered requirements \(no covering task\): 2/);

  const j = JSON.parse((await run(dir, "context", "vf", "--json")).out);
  assert.equal(j.status, "verifying");
  assert.deepEqual(j.signals.uncoveredRequirements, [2]);
});

test("context: no SIGNALS section when nothing is live", async () => {
  const dir = newTmp();
  await scaffold(dir, "clean");
  const txt = await run(dir, "context", "clean");
  assert.equal(txt.rc, 0);
  assert.doesNotMatch(txt.out, /SIGNALS:/);

  const j = JSON.parse((await run(dir, "context", "clean", "--json")).out);
  assert.deepEqual(j.signals.blockers, []);
  assert.equal(j.signals.latestMidreq, null);
  assert.deepEqual(j.signals.uncoveredRequirements, []);
});
